package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// These small deterministic models were written for tla-go. Java TLC 1.7.4
// (upstream 5a47802b5c391f59ecdd44117981f4ff8c0656ba) rejects the ALIAS
// keyword; Java TLC 1.8.0 (090e85c31a6d7fb3d34a33a9f1a46310cdc2aa7f)
// accepts it but leaves these DFID traces raw.
func TestJavaTLCDepthFirstAliasOracle(t *testing.T) {
	cases := []struct {
		name, next, extra string
		code, states      int
	}{
		{"invariant", "x < 3 /\\ x' = x + 1", "INVARIANT Inv\n", 12, 3},
		{"deadlock", "x < 2 /\\ x' = x + 1", "", 11, 3},
		{"next error", "x' = IF x = 2 THEN 1 \\div 0 ELSE x + 1", "", 255, 3},
	}
	for _, tc := range cases {
		for _, tool := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/normal", true: "/tool"}[tool], func(t *testing.T) {
				dir := t.TempDir()
				source := "---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\nInv == x < 2\nA == [z |-> x + 10, a |-> x]\n====\n"
				files := map[string]string{"M.tla": source, "plain.cfg": "INIT Init\nNEXT Next\n" + tc.extra, "alias.cfg": "INIT Init\nNEXT Next\nALIAS A\n" + tc.extra}
				for name, content := range files {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"-workers", "1", "-fp", "0", "-dfid", "4"}
				if tool {
					args = append([]string{"-tool"}, args...)
				}
				var control []string
				for _, alias := range []bool{false, true} {
					cfg := "plain.cfg"
					if alias {
						cfg = "alias.cfg"
					}
					javaArgs := append(append([]string{}, args...), "-config", cfg, "M.tla")
					goArgs := append(append([]string{}, args...), "-config", filepath.Join(dir, cfg), filepath.Join(dir, "M.tla"))
					var out, errout bytes.Buffer
					code := run(context.Background(), goArgs, &out, &errout)
					got := dfidAliasTrace(out.String(), tool)
					if code != tc.code || errout.Len() != 0 || len(got) != tc.states || !strings.Contains(strings.Join(got, "\n"), "x = 2") {
						t.Fatalf("Go exit %d, stderr %s, trace %q\n%s", code, errout.String(), got, out.String())
					}
					if !alias {
						control = got
					} else if !reflect.DeepEqual(got, control) {
						t.Fatalf("ALIAS changed DFID trace: %q vs %q", got, control)
					}
					if jar := os.Getenv("TLC_JAR"); jar != "" {
						output, javaCode := runJavaTLCArgs(t, jar, dir, javaArgs)
						if alias {
							if javaCode != 255 || !strings.Contains(output, "configuration file at line 3") || !strings.Contains(output, "expecting a keyword") {
								t.Fatalf("Java 1.7.4 exit %d: %s", javaCode, output)
							}
						} else if trace := dfidAliasTrace(output, tool); javaCode != code || !reflect.DeepEqual(trace, got) {
							t.Fatalf("Java 1.7.4 exit %d, trace %q; Go exit %d, trace %q", javaCode, trace, code, got)
						}
					}
					if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
						output, javaCode := runJavaTLCArgs(t, jar, dir, javaArgs)
						if trace := dfidAliasTrace(output, tool); javaCode != code || !reflect.DeepEqual(trace, got) {
							t.Fatalf("Java modern exit %d, trace %q; Go exit %d, trace %q\n%s", javaCode, trace, code, got, output)
						}
					}
				}
			})
		}
	}
}

var dfidNormalState = regexp.MustCompile(`(?ms)^State [0-9]+: *\n.*?\n\n`)

func dfidAliasTrace(output string, tool bool) []string {
	if tool {
		var states []string
		for _, message := range readToolMessages(output) {
			if message.code == 2216 || message.code == 2217 {
				states = append(states, strings.ReplaceAll(strings.TrimSpace(message.body), ": \n", ":\n"))
			}
		}
		return states
	}
	states := dfidNormalState.FindAllString(output, -1)
	for i, state := range states {
		states[i] = strings.ReplaceAll(strings.TrimSpace(state), ": \n", ":\n")
	}
	return states
}
