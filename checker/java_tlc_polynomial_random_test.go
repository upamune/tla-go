package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCRandomWorkerPolynomials(t *testing.T) {
	const source = `---- MODULE Worker ----
EXTENDS Naturals, TLC, Randomization
VARIABLES x, step
Init == /\ x = {} /\ step = 0
Next == /\ step < 2 /\ x' = RandomSubset(3, 1..20) /\ step' = step + 1
Inv == step < 2
====
`
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, index := range []int{0, 1, 63, 130} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			result, err := CheckWithOptions(context.Background(), source, cfg, Options{Seed: 1, Fingerprint: index})
			if err != nil || result.Status != InvariantViolation || result.Violation == nil {
				t.Fatalf("tla-go: %+v, %v", result, err)
			}
			var got []string
			for _, step := range result.Violation.Trace {
				got = append(got, step.State["x"].String())
			}
			dir := t.TempDir()
			for name, content := range map[string]string{"Worker.tla": source, "Worker.cfg": cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", "1", "-fp", fmt.Sprint(index), "-deadlock", "Worker.tla")
			cmd.Dir = dir
			javatmp.Apply(cmd)
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 12 {
				t.Fatalf("Java TLC: %v\n%s", err, output)
			}
			matches := regexp.MustCompile(`(?m)^/\\ x = (.*)$`).FindAllStringSubmatch(string(output), -1)
			var want []string
			for _, match := range matches {
				want = append(want, match[1])
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Go trace %q; Java trace %q", got, want)
			}
		})
	}
}
