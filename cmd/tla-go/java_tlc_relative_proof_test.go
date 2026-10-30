package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJavaTLCRelativeProofSteps(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		code        int
	}{
		{"root plus", "<+>* TRUE\n<*>- TRUE\n<*> QED", 0},
		{"root star", "<*>* TRUE\n<*> QED", 0},
		{"implicit reference", "<1>1 TRUE\n<1>2 TRUE BY <*>1\n<1> QED", 0},
		{"nested plus", "<1>1 TRUE\n <+>* TRUE\n <*> QED\n<1> QED", 0},
		{"nested explicit star", "<1>1 TRUE\n PROOF <*>* TRUE\n <*> QED\n<1> QED", 0},
		{"implicit siblings", "<1>1 TRUE\n <*>* TRUE\n <*> QED", 0},
		{"zero level", "<0>1 TRUE\n<0> QED", 0},
		{"unnumbered star", "<1>* TRUE\n<1> QED", 0},
		{"unnumbered dash", "<1>- TRUE\n<1> QED", 0},
		{"implicit definition", "<1> P == TRUE\n<1>1 P BY DEF P\n<1> QED BY <1>1", 0},
		{"implicit function definition", "<1> F[x \\in {0}] == x\n<1>1 F[0] = 0 BY DEF F\n<1> QED BY <1>1", 0},
		{"implicit function delimiter string", `<1> F[x \in {"]"}] == x
<1>1 F["]"] = "]" BY DEF F
<1> QED BY <1>1`, 0},
		{"recursive local function", "<1> DEFINE F[n \\in {0}] == IF n = 0 THEN 0 ELSE F[n]\n<1>1 F[0] = 0 BY DEF F\n<1> QED BY <1>1", 0},
		{"step used as expression", "<1>1 TRUE\n<1>2 <1>1 = TRUE\n<1> QED BY <1>2", 0},
		{"step selector", "<1>1 \\E n \\in {1} : n = 1\n<1>2 <1>1!(1)\n<1> QED BY <1>2", 0},
		{"suffices step selector", "<1>1 SUFFICES \\E n \\in {1} : n = 1\n <2>1 ASSUME NEW n \\in {1}, <1>1!1!(n) PROVE TRUE\n <2> QED BY <2>1\n<1> QED BY <1>1", 0},
		{"invalid step selector", "<1>1 \\E n \\in {1} : n = 1\n<1>2 <1>1!1!(1)\n<1> QED BY <1>2", 150},
		{"unknown step selector", "<1>1 <1>9!1 = TRUE\n<1> QED", 150},
		{"unknown step in expression", "<1>1 <1>9 = TRUE\n<1> QED", 150},
		{"definition step is not an expression", "<1>1 DEFINE F == TRUE\n<1>2 <1>1 = TRUE\n<1> QED", 150},
		{"leading zero definition", "<01>1 TRUE\n<1>2 TRUE BY <1>1\n<1> QED", 0},
		{"leading zero reference", "<01>1 TRUE\n<1>2 TRUE BY <01>1\n<1> QED", 150},
		{"named relative", "<1>1 TRUE\n<*>2 TRUE\n<1> QED", 150},
		{"automatic step reference", "<1>* TRUE\n<1>2 TRUE BY <1>*\n<1> QED", 150},
		{"plus step reference", "<1>1 TRUE\n<1>2 TRUE BY <+>1\n<1> QED", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\nTHEOREM T == TRUE\n" + tc.proof + "\n====\n", "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}
