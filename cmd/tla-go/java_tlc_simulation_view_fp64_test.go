package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const scalarViewCollisionModel = `---- MODULE M ----
EXTENDS Integers, TLC
CONSTANTS a,b
VARIABLES x,y,z
Init == x=0 /\ y=0 /\ z=0
Next == \/ x=0 /\ x'=-1130370423 /\ y'=1673453457 /\ z'=0
        \/ x#0 /\ x'=0 /\ y'=0 /\ z'=0
P == []<>(x=0)
V == <<x,y,z>>
Sym == Permutations({a,b})
====`

func TestJavaTLCSimulationScalarViewFingerprintCollision(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	for _, tc := range []struct {
		name, fp, reduction string
		tool                bool
		want                int
	}{
		{"fp0-tool", "0", "", true, 0},
		{"fp0-normal", "0", "", false, 0},
		{"fp1-tool", "1", "", true, 13},
		{"fp1-normal", "1", "", false, 13},
		{"symmetry-tool", "0", "SYMMETRY Sym", true, 0},
		{"symmetry-normal", "0", "SYMMETRY Sym", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := "CONSTANTS a=a b=b INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE " + tc.reduction
			for name, contents := range map[string]string{"M.tla": scalarViewCollisionModel, "M.cfg": cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workers", "1", "-fp", tc.fp, "-simulate", "num=1", "-seed", "1", "-depth", "3", "M.tla"}
			if tc.tool {
				args = append([]string{"-tool"}, args...)
			}
			goArgs := append([]string(nil), args...)
			goArgs[len(goArgs)-1] = filepath.Join(dir, "M.tla")
			var out, errout bytes.Buffer
			got := run(context.Background(), goArgs, &out, &errout)
			java, want := runJavaTLCArgs(t, jar, dir, args)
			if got != tc.want || want != tc.want || errout.Len() != 0 {
				t.Fatalf("Go exit=%d stderr=%s output=%s\nJava exit=%d output=%s", got, errout.String(), out.String(), want, java)
			}
		})
	}
}

// During recovery Java fingerprints the first candidate before reaching the
// scalar match. Sampling itself only visits scalar states, so this exercises the
// general-fingerprint fallback after the scalar graph path was selected.
func TestJavaTLCSimulationScalarViewRecoveryCandidates(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	for _, tc := range []struct {
		name, candidate string
		symmetry        bool
		want            int
	}{
		{"model", "a", false, 13},
		{"model-symmetry", "a", true, 13},
		{"tuple", "<<a,b>>", false, 13},
		{"tuple-symmetry", "<<a,b>>", true, 13},
		{"recovery-error", "(1 \\div 0)", true, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS a,b\nVARIABLE x\n" +
				"Init == IF TLCGet(\"level\") >= 3 THEN \\/ /\\ x = " + tc.candidate + " /\\ PrintT(\"model-candidate\")\n" +
				"                                     \\/ /\\ x=0 /\\ PrintT(\"scalar-candidate\")\n" +
				"        ELSE x=0\nNext == x' = 1-x\nP == <>(x=5)\nV == x\nSym == Permutations({a,b})"
			cfg := "CONSTANTS a=a b=b INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE"
			if tc.symmetry {
				cfg += " SYMMETRY Sym"
			}
			writeDOTFixture(t, dir, body, cfg)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", "M.tla"}
			goArgs := append([]string{}, args...)
			goArgs[len(goArgs)-1] = filepath.Join(dir, "M.tla")
			var out, errout bytes.Buffer
			got := run(context.Background(), goArgs, &out, &errout)
			java, want := runJavaTLCArgs(t, jar, dir, args)
			if got != tc.want || want != tc.want || errout.Len() != 0 {
				t.Fatalf("Go exit=%d stderr=%s output=%s\nJava exit=%d output=%s", got, errout.String(), out.String(), want, java)
			}
			if tc.want == 13 {
				for _, output := range []string{java, out.String()} {
					model, scalar := strings.Index(output, `"model-candidate"`), strings.Index(output, `"scalar-candidate"`)
					recovery, printed := strings.Index(output, "@!@!@STARTMSG 2264"), strings.Index(output, "@!@!@STARTMSG 2217")
					if !strings.Contains(output, "Computed 1 initial states") || recovery < 0 || model <= recovery || scalar <= model || printed <= scalar {
						t.Fatalf("expected one sampled initial state and nonscalar candidate first during recovery: %s", output)
					}
				}
			}
			if gotMessages, wantMessages := recoveryCandidateMessages(out.String()), recoveryCandidateMessages(java); !reflect.DeepEqual(gotMessages, wantMessages) {
				t.Fatalf("Go messages %+v\nJava messages %+v", gotMessages, wantMessages)
			}
		})
	}
}

func recoveryCandidateMessages(output string) []toolMessage {
	var selected []toolMessage
	for _, message := range readToolMessages(output) {
		switch message.code {
		case 1000, 2116, 2121, 2179, 2217, 2218, 2264:
			selected = append(selected, message)
		}
	}
	return selected
}
