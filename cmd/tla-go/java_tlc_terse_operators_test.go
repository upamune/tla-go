package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Small independent observations against TLC 1.7.4, built from TLA+ Tools
// commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba (see docs/development.md).
func TestJavaTLCTerseOperators(t *testing.T) {
	cases := []struct {
		name, expr, plain, terse string
		codePlain, codeTerse     int
	}{
		{"ToString", `PrintT(ToString(SUBSET {1, 2}))`, `"{{}, {1}, {2}, {1, 2}}"`, `"SUBSET {1, 2}"`, 0, 0},
		{"nested function", `PrintT(ToString([i \in 1..2 |-> SUBSET {i}]))`, `"<<{{}, {1}}, {{}, {2}}>>"`, `"<<SUBSET {1}, SUBSET {2}>>"`, 0, 0},
		{"Assert", `Assert(FALSE, SUBSET {1, 2})`, `{{}, {1}, {2}, {1, 2}}`, `SUBSET {1, 2}`, 10, 10},
		{"equality error", `SUBSET {1,2} = 1`, `Attempted to check equality of the set {{}, {1}, {2}, {1, 2}}`, `Attempted to check equality of the set {{}, {1}, {2}, {1, 2}}`, 10, 10},
		{"infinite product", `PrintT(Nat \X {1})`, `Attempted to compute the number of elements`, `(Nat \X {1})`, 10, 0},
		{"infinite subset", `PrintT(ToString(SUBSET Nat))`, `Attempted to compute the number of elements`, `"SUBSET Nat"`, 10, 0},
		{"lazy Assert", `Assert(FALSE, [i \in Nat |-> 1 \div 0])`, `<expression line 3, col 37 to line 3, col 44 of module M>`, `<expression line 3, col 37 to line 3, col 44 of module M>`, 10, 10},
		{"lazy function", `PrintT(ToString([i \in Nat |-> 1 \div 0]))`, `<expression line 3, col 39 to line 3, col 46 of module M>`, `<expression line 3, col 39 to line 3, col 46 of module M>`, 0, 0},
		{"function evaluation error", `PrintT(ToString([i \in 1..2 |-> 1 \div (2-i)]))`, `The second argument of \div is 0`, `The second argument of \div is 0`, 10, 10},
	}
	for _, tc := range cases {
		for _, terse := range []bool{false, true} {
			for _, tool := range []bool{false, true} {
				t.Run(tc.name+"/terse="+boolName(terse)+"/tool="+boolName(tool), func(t *testing.T) {
					dir := t.TempDir()
					source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME " + tc.expr + "\n====\n"
					if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
						t.Fatal(err)
					}
					args := []string{}
					if tool {
						args = append(args, "-tool")
					}
					if terse {
						args = append(args, "-terse")
					}
					want, code := tc.plain, tc.codePlain
					if terse {
						want, code = tc.terse, tc.codeTerse
					}
					var out, errout bytes.Buffer
					goWant, goCode := want, code
					if (tc.name == "infinite subset" || tc.name == "infinite product") && !terse {
						goWant = "cardinality requires a finite set"
					}
					if tc.name == "equality error" {
						goWant, goCode = "cannot compare different TLA+ value kinds", 255
					}
					gotCode := run(context.Background(), append(append([]string{}, args...), filepath.Join(dir, "M.tla")), &out, &errout)
					if gotCode != goCode || !strings.Contains(out.String()+errout.String(), goWant) {
						t.Fatalf("Go exit %d want %d, missing %q; stdout %s; stderr %s", gotCode, goCode, goWant, out.String(), errout.String())
					}
					if jar := os.Getenv("TLC_JAR"); jar != "" {
						cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar, "-workers", "1"}, append(args, "M.tla")...)...)
						cmd.Dir = dir
						output, err := cmd.CombinedOutput()
						actual := 0
						if err != nil {
							if exit, ok := err.(*exec.ExitError); ok {
								actual = exit.ExitCode()
							} else {
								t.Fatal(err)
							}
						}
						if actual != code || !strings.Contains(string(output), want) {
							t.Fatalf("Java exit %d want %d, missing %q; output %s", actual, code, want, output)
						}
					}
				})
			}
		}
	}
}

func boolName(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
