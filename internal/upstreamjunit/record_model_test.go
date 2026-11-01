package upstreamjunit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This model compares concrete observations, not whole-run or JUnit parity.
func TestRecordOrderModel(t *testing.T) {
	source, err := os.ReadFile("testdata/record_order/Probe.tla")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	jar := os.Getenv("TLA_GO_JUNIT_JAR")
	root := os.Getenv("TLA_GO_JUNIT_ROOT")
	if jar != "" && root != "" {
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		root, err = filepath.Abs(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyInputs(root, jar); err != nil {
			t.Fatal(err)
		}
	} else {
		jar = ""
	}
	for _, tc := range []struct {
		name, cfg, stats string
		reverse          bool
	}{
		{"plain", "", "8 states generated, 4 distinct states found", false},
		{"reversed", "", "8 states generated, 4 distinct states found", true},
		{"symmetry", "SYMMETRY Sym\n", "6 states generated, 2 distinct states found", false},
		{"view", "VIEW View\n", "8 states generated, 4 distinct states found", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := string(source)
			if tc.reverse {
				model = strings.Replace(model, "S == [op : {A, B}, adr : {A, B}]", "S == [adr : {A, B}, op : {A, B}]", 1)
			}
			for _, java := range []bool{false, true} {
				if java && jar == "" {
					continue
				}
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "Probe.tla"), []byte(model), 0o600); err != nil {
					t.Fatal(err)
				}
				cfg := "CONSTANTS A = A B = B\nINIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n" + tc.cfg
				if err := os.WriteFile(filepath.Join(dir, "Probe.cfg"), []byte(cfg), 0o600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				args := []string{"-workers", "1", "-fp", "0", "-metadir", "metadata", "-dump", "dot", "graph.dot", "Probe.tla"}
				var cmd *exec.Cmd
				if java {
					cmd = exec.CommandContext(ctx, "java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-XX:+UseParallelGC", "-jar", jar}, args...)...)
				} else {
					cmd = exec.CommandContext(ctx, binary, args...)
				}
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("java=%v: %v\n%s", java, err, out)
				}
				record := "[adr |-> A, op |-> A]"
				if java && !tc.reverse {
					record = "[op |-> A, adr |-> A]"
				}
				for _, want := range []string{tc.stats, `"direct"  ` + record, `"string"  "` + record + `"`, `"nested"  <<` + record} {
					if !strings.Contains(string(out), want) {
						t.Errorf("java=%v missing %q\n%s", java, want, out)
					}
				}
				dot, err := os.ReadFile(filepath.Join(dir, "graph.dot"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(dot), record) {
					t.Errorf("java=%v DOT missing %q:\n%s", java, record, dot)
				}
			}
		})
	}
}
