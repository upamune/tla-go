package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
)

// The model files and state counts come from Java TLC v1.7.4's JUnit tests.
// Coverage assertions are not checked here; experimental coverage supports only
// a restricted subset of models.
func TestJavaTLCOriginalModels(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		generated, distinct, depth int
	}{
		{"AssignmentInit", 6, 5, 1},
		{"MinimumDiameter", 2, 1, 1},
		{"SubsetEq", 2, 1, 1},
		{"MinimalSetOfNextStates", 57, 7, 2},
		{"AssignmentInitNeg", 2, 1, 1},
		{"AssignmentInitExpensive", 10002, 1, 1},
		{"AssignmentNext", 3, 2, 2},
		{"AssignmentNext2", 3, 2, 2},
		{"AssignmentNext3", 26, 5, 2},
		{"MinimalSetOfInitStates", 14, 6, 1},
		{"DieHardTLA", 97, 16, 8},
		{"CodePlexBug21", 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", "java-tlc", tc.name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			cfgText, err := os.ReadFile(filepath.Join("testdata", "java-tlc", tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(string(cfgText))
			if err != nil {
				t.Fatal(err)
			}
			// ModelCheckerTestCase disables deadlock checking by default.
			cfg.CheckDeadlock = false
			got, err := CheckConfig(context.Background(), string(source), cfg)
			if err != nil || got.Status != Success || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v; want success, %d generated, %d distinct, depth %d", got, err, tc.generated, tc.distinct, tc.depth)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaTLCModel(t, jar, tc.name, source, cfgText, tc.generated, tc.distinct, tc.depth)
			}
		})
	}
}

func compareJavaTLCModel(t *testing.T, jar, name string, source, config []byte, generated, distinct, depth int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for filename, contents := range map[string][]byte{name + ".tla": source, name + ".cfg": config} {
		if err := os.WriteFile(filepath.Join(dir, filename), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-deadlock", name+".tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue", generated, distinct)
	depthLine := fmt.Sprintf("The depth of the complete state graph search is %d.", depth)
	if ctx.Err() != nil || err != nil || !strings.Contains(string(output), stats) || !strings.Contains(string(output), depthLine) {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), output)
	}
}

func TestSpecificationRejectsTemporalInitialDisjunction(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nNext == UNCHANGED x\nSpec == (x = 0 \\/ <> (x = 0)) /\\ [][Next]_x\n===="
	if _, err := Check(source, "SPECIFICATION Spec"); err == nil {
		t.Fatal("accepted temporal disjunction as an initial predicate")
	}
}

func TestSpecificationInitialDisjunctionWithTLCDefer(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLCExt\nVARIABLE x\nNext == UNCHANGED x\nSpec == /\\ x = 0\n /\\ (TLCDefer(x' = 1) \\/ x = 1)\n /\\ [][Next]_x\n===="
	got, err := Check(source, "SPECIFICATION Spec CHECK_DEADLOCK FALSE")
	if err != nil || got.Status != Success || got.DistinctStates != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestJavaTLCUniversalAction(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nWitness(i) == \\E j \\in 1..3 : j >= i\nInit == x = 0\nNext == /\\ x < 2 /\\ x' = x + 1 /\\ \\A i \\in {1,2} : Witness(i)\n====\n"
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n"
	got, err := Check(source, cfg)
	if err != nil || got.Status != Success || got.GeneratedStates != 3 || got.DistinctStates != 3 || got.Depth != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), 3, 3, 3)
	}
}

func TestJavaTLCEmptyInitialLiveness(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in {}\nNext == x' = x\nP == <>(x = 2)\n====\n"
	cfg := "INIT Init NEXT Next PROPERTY P\n"
	got, err := Check(source, cfg)
	if err != nil || got.Status != Success || got.GeneratedStates != 0 || got.DistinctStates != 0 || got.Depth != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), 0, 0, 1)
	}
}

func TestJavaTLCStateDependentChoice(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nChoice == CHOOSE n \\in 1..3 : n = x + 1\nInit == x = 0\nNext == x' = IF x < 2 THEN Choice ELSE x\n====\n"
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n"
	got, err := Check(source, cfg)
	if err != nil || got.Status != Success || got.GeneratedStates != 4 || got.DistinctStates != 3 || got.Depth != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), 4, 3, 3)
	}
}
