package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These are observations of the pinned Java oracle, not JUnit executions or
// assertions that the entire graph has a stable order across runs.
func TestPinnedLivenessDumpOracle(t *testing.T) {
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Skip("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInputs(root, jar); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, tc := range []struct {
		name, path, model, initial, witness string
		exit                                int
	}{
		{"NQ", "symmetry/NQ", "MC", "5659690209985959133 [label=", "-5139828646329981039 [label=", 13},
		{"OneBitMutex", "symmetry/OneBitMutex", "OneBitMutexMC", "-8407932991663587895 [label=", `-3946579319044950398 [label="/\\ unchecked = (A :> {} @@ B :> {})\n/\\ other = (A :> A @@ B :> B)\n/\\ x = (A :> FALSE @@ B :> FALSE)\n/\\ pc = (A :> \"e1\" @@ B :> \"ncs\")"`, 13},
		{"SymmetryTableau", "symmetry", "SymmetryLivenessTableauMC", "-2383001886493292559 [label=", `579606404721062638 [label="/\\ mem =`, 13},
		{"TwoPhaseCommit", "symmetry/TwoPhaseCommit", "MC", "-8348026569971206050 [label=", "-7140997411712456272 [label=", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := copyModels(root, dir); err != nil {
				t.Fatal(err)
			}
			c := Case{Model: tc.model, Path: tc.path, Coverage: true, Dump: true, Workers: "1", Args: []string{"-metadir", "metadata"}}
			j, g, diffs, err := runModelCase(dir, jar, binary, c, 60*time.Second)
			if err != nil || j.ExitCode != tc.exit || g.ExitCode != tc.exit {
				t.Fatalf("Java exit=%d Go exit=%d error=%v", j.ExitCode, g.ExitCode, err)
			}
			java, err := os.ReadFile(filepath.Join(dir, "java.dot"))
			if err != nil {
				t.Fatal(err)
			}
			goDump, err := os.ReadFile(filepath.Join(dir, "go.dot"))
			if err != nil {
				t.Fatal(err)
			}
			for _, needle := range []string{tc.initial, tc.witness} {
				if !strings.Contains(string(java), needle) {
					t.Errorf("Java graph missing oracle witness %q", needle)
				}
			}
			if !strings.Contains(string(goDump), tc.witness) {
				t.Errorf("Go graph missing witness %q", tc.witness)
			}
			if !strings.Contains(string(goDump), tc.initial) {
				t.Errorf("Go graph missing common initial node %q", tc.initial)
			}
			t.Logf("raw partial comparison: %v; whole-graph order and JUnit lifecycle are not asserted", diffs)
		})
	}
}

func TestCompareDumpDetectsByteDifferences(t *testing.T) {
	dir := t.TempDir()
	java := filepath.Join(dir, "java.dot")
	goFile := filepath.Join(dir, "go.dot")
	if err := os.WriteFile(java, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goFile, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if diffs, err := compareDump(dir); err != nil || len(diffs) != 0 {
		t.Fatalf("equal: %v, %v", diffs, err)
	}
	if err := os.WriteFile(goFile, []byte("b\na\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if diffs, err := compareDump(dir); err != nil || len(diffs) != 1 || diffs[0] != "dump:content" {
		t.Fatalf("different order: %v, %v", diffs, err)
	}
}
