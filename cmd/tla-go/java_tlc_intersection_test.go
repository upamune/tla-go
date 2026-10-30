package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCReducibleIntersection(t *testing.T) {
	for _, tc := range []struct {
		name, expression, expected string
		code                       int
	}{
		{"interval on right", `(SUBSET (1..3)) \cap (1..4)`, "", 75},
		{"interval on left", `(1..4) \cap (SUBSET (1..3))`, "", 75},
		{"enumerated on right", `(SUBSET (1..3)) \cap {1, 4}`, "", 75},
		{"enumerated on left", `{1, 4} \cap (SUBSET (1..3))`, "", 75},
		{"boolean member", `{TRUE} \cap (SUBSET (1..3))`, "", 75},
		{"string member", `{"a"} \cap (SUBSET (1..3))`, "", 75},
		{"tuple member", `{<<1>>} \cap (SUBSET (1..3))`, "", 75},
		{"reduced intersection as operand", `((1..4) \cap (1..3)) \cap (SUBSET (1..3))`, "", 75},
		{"valid enumerated intersection", `(SUBSET (1..3)) \cap {{1}, {2}}`, `{{1}, {2}}`, 0},
		{"valid intervals", `(1..4) \cap (3..6)`, `{3, 4}`, 0},
		{"disjoint intervals", `(1..3) \cap (4..6)`, `{}`, 0},
		{"empty interval", `(3..1) \cap (1..3)`, `{}`, 0},
		{"large interval endpoints", `(2147483644..2147483646) \cap (2147483645..2147483646)`, `{2147483645, 2147483646}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = " + tc.expression + "\nNext == UNCHANGED x"
			cfg := "INIT Init\nNEXT Next\n"
			if tc.expected != "" {
				body += "\nInv == x = " + tc.expected
				cfg += "INVARIANT Inv\n"
			}
			writeDOTFixture(t, dir, body, cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				messages := readToolMessages(out.String())
				if tc.code == 75 {
					compareJavaFingerprintToolMessages(t, jar, dir, "M", fingerprintToolMessages(messages))
				} else {
					compareJavaToolMessages(t, jar, dir, code, messages)
				}
			}
		})
	}
}
