package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Empty.tla and Empty.cfg are copied verbatim from tlaplus/tlaplus commit
// 5a47802b5c391f59ecdd44117981f4ff8c0656ba, test-model/Empty.*.
// EmptyTest#testSpec checks FINISHED, absence of GENERAL and stats 0/0/0.
func TestJavaTLCCoverageEmpty(t *testing.T) {
	fixture := filepath.Join("testdata", "java-tlc", "empty-upstream")
	tla, err := os.ReadFile(filepath.Join(fixture, "Empty.tla"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(fixture, "Empty.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, source, config string }{
		{"upstream", string(tla), string(cfg)},
		{"definitions", "---- MODULE Empty ----\nUnused == 1\nASSUME TRUE\n====\n", ""},
		{"false assumption", "---- MODULE Empty ----\nASSUME FALSE\n====\n", ""},
		{"evaluation error", "---- MODULE Empty ----\nEXTENDS Naturals\nASSUME 1 \\div 0 = 0\n====\n", ""},
	}
	jar := os.Getenv("TLC_JAR")
	if jar != "" {
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		data, e := os.ReadFile(jar)
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatal("TLC_JAR is not pinned TLC 1.7.4")
		}
	}
	for _, tc := range cases {
		for _, interval := range []string{"0", "1"} {
			for _, tool := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/tool=%t", tc.name, interval, tool), func(t *testing.T) {
					dir := t.TempDir()
					if e := os.WriteFile(filepath.Join(dir, "Empty.tla"), []byte(tc.source), 0o600); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(filepath.Join(dir, "Empty.cfg"), []byte(tc.config), 0o600); e != nil {
						t.Fatal(e)
					}
					args := []string{"-workers", "1", "-coverage", interval, "Empty.tla"}
					if tool {
						args = append([]string{"-tool"}, args...)
					}
					out, code, stderr := runGoInDir(t, dir, args)
					if stderr != "" {
						t.Fatalf("stderr: %s", stderr)
					}
					if strings.Contains(out, "coverage statistics") {
						t.Fatalf("unexpected coverage: %s", out)
					}
					wantCode := 0
					if strings.Contains(tc.name, "assumption") || tc.name == "evaluation error" {
						wantCode = 10
					}
					if code != wantCode {
						t.Fatalf("exit=%d output=%s", code, out)
					}
					if wantCode == 0 && !strings.Contains(out, "0 states generated, 0 distinct states found, 0 states left on queue.") {
						t.Fatalf("missing empty-model statistics: %s", out)
					}
					if jar != "" {
						java, jcode := runJavaTLCArgs(t, jar, dir, args)
						if code != jcode {
							t.Fatalf("exit Go=%d Java=%d\nGo:\n%s\nJava:\n%s", code, jcode, out, java)
						}
						got, want := emptyComparable(out, tool), emptyComparable(java, tool)
						if got != want {
							t.Errorf("output differs\nGo:\n%s\nJava:\n%s\nraw Go:\n%s\nraw Java:\n%s", got, want, out, java)
						}
					}
				})
			}
		}
	}
}

func TestCoverageEmptyDoesNotReplayAssumptions(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS TLC\nASSUME PrintT(\"once\")\n====\n",
		"M.cfg": "",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, interval := range []string{"0", "1"} {
		out, code, stderr := runGoInDir(t, dir, []string{"-coverage", interval, "M.tla"})
		if code != 0 || stderr != "" || strings.Count(out, `"once"`) != 1 {
			t.Fatalf("interval=%s exit=%d stderr=%s output=%s", interval, code, stderr, out)
		}
	}
}

var emptyTime = regexp.MustCompile(`\d{4}-\d\d-\d\d \d\d:\d\d:\d\d`)

// Runtime banners, SANY diagnostics, timings and progress rates are known
// implementation differences. Compare the complete result stream after them.
func emptyComparable(s string, tool bool) string {
	if tool {
		msgs := readToolMessages(s)
		var b strings.Builder
		for _, m := range msgs {
			if m.code == 2262 || m.code == 2187 || m.code == 2220 || m.code == 2219 {
				continue
			}
			body := emptyTime.ReplaceAllString(m.body, "TIME")
			if m.code == 2186 {
				body = "Finished"
			}
			if m.code == 2200 {
				body = regexp.MustCompile(` \([^)]*s/min\)`).ReplaceAllString(body, "")
			}
			fmt.Fprintf(&b, "%d:%s\n", m.code, body)
		}
		return b.String()
	}
	start := strings.Index(s, "Model checking completed.")
	if e := strings.Index(s, "Error:"); e >= 0 && (start < 0 || e < start) {
		start = e
	}
	if start < 0 {
		return s
	}
	s = s[start:]
	if i := strings.Index(s, "Finished in "); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(emptyTime.ReplaceAllString(s, "TIME"))
}

func TestCoverageEmptyRejectsUnmodeledActions(t *testing.T) {
	for _, tc := range []struct{ source, config string }{
		{"Spec == [][TRUE]_<<>>", "SPECIFICATION Spec\n"},
		{"Inv == TRUE", "INVARIANT Inv\n"},
		{"C == TRUE", "CONSTRAINT C\n"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte("---- MODULE M ----\n"+tc.source+"\n====\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
		})
	}
}
