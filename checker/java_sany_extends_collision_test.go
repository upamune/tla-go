package checker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SANY 1.7.4 retains each extended module's internal operator bindings.
func TestJavaTLCExtendedOperatorCollisions(t *testing.T) {
	jar := instanceCollisionOracle(t)
	const a = "---- MODULE A ----\nF(x) == 1\n====\n"
	const b = "---- MODULE B ----\nEXTENDS Naturals\nF(x) == 2\nG(x) == F(x) + 10\nParam(z) == z\nUse(Op(_)) == Op(1)\nQ == Use(F)\nASSUME F(1) = 2\n====\n"
	for _, tc := range []struct {
		name, root, c, want string
		invalid             bool
	}{
		{"instance then original", "EXTENDS C, A, Naturals\nINSTANCE B\nY == <<F(1), G(1)>>", "INSTANCE A", "<<1, 12>>", false},
		{"original then instance", "EXTENDS A, C, Naturals\nINSTANCE B\nY == <<F(1), G(1)>>", "INSTANCE A", "<<1, 12>>", false},
		{"independent modules", "EXTENDS A, B, Naturals\nY == <<F(1), G(1), Param(7), Q>>", "", "<<1, 12, 7, 2>>", false},
		{"reverse independent modules", "EXTENDS B, A, Naturals\nY == <<F(1), G(1)>>", "", "<<2, 12>>", false},
		{"transitive binding", "EXTENDS A, C, Naturals\nY == <<F(1), G(1), H>>", "EXTENDS B, Naturals\nH == F(1)", "<<1, 12, 2>>", false},
		{"instantiate merged module", "EXTENDS Naturals\nI == INSTANCE C\nY == <<I!F(1), I!G(1)>>", "EXTENDS A, B, Naturals", "<<1, 12>>", false},
		{"local duplicate", "EXTENDS A, B, Naturals\nF(x) == 3\nY == 1", "", "1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := "---- MODULE C ----\n" + tc.c + "\n====\n"
			source := "---- MODULE M ----\n" + tc.root + "\nVARIABLE x\nInit == x = Y\nNext == UNCHANGED x\nInv == x = " + tc.want + "\n====\n"
			cfg := "INIT Init\nNEXT Next\nINVARIANT Inv\n"
			files := map[string]string{"M.tla": source, "M.cfg": cfg, "A.tla": a, "B.tla": b, "C.tla": c}
			loader := func(_ context.Context, name string) (string, error) {
				s, ok := files[name+".tla"]
				if !ok {
					return "", os.ErrNotExist
				}
				return s, nil
			}
			if jar != "" {
				dir := t.TempDir()
				for name, data := range files {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				checkCollisionSANY(t, dir, jar, tc.invalid)
				out, code := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				want := 0
				if tc.invalid {
					want = 150
				}
				if code != want || strings.Contains(out, "Invariant Inv is violated") {
					t.Fatalf("Java exit=%d want=%d output=%s", code, want, out)
				}
			}
			result, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if tc.invalid {
				if err == nil {
					t.Fatalf("accepted invalid model: %+v", result)
				}
				return
			}
			if err != nil || result.Status != Success {
				t.Fatalf("Go result=%+v error=%v", result, err)
			}
		})
	}
}

func TestExtendedCollisionDeclinesScopedRenaming(t *testing.T) {
	a := "---- MODULE A ----\nF(x) == 1\n====\n"
	root := "---- MODULE M ----\nEXTENDS A, B\nVARIABLE x\nInit == x = G\nNext == UNCHANGED x\n====\n"
	cfg := "INIT Init\nNEXT Next\n"
	for _, body := range []string{
		"G == LET P == F(1) IN P",
		"G == CHOOSE n \\in {1, 2}: n = F(1)",
	} {
		b := "---- MODULE B ----\nF(x) == 2\n" + body + "\n====\n"
		loader := func(_ context.Context, name string) (string, error) {
			switch name {
			case "A":
				return a, nil
			case "B":
				return b, nil
			default:
				return "", os.ErrNotExist
			}
		}
		if _, err := CheckWithLoader(context.Background(), root, cfg, loader); err == nil {
			t.Fatalf("expected scoped renaming to be rejected for %q", body)
		}
	}
}

func TestJavaSANYExtendedCollisionForwardReferences(t *testing.T) {
	jar := instanceCollisionOracle(t)
	const a = "---- MODULE A ----\nF(x) == 1\n====\n"
	for _, tc := range []struct {
		name, body string
		invalid    bool
	}{
		{"earlier definition", "H == 1\nG == H", false},
		{"later definition", "G == H\nH == 1", true},
		{"later LET definition", "G == LET K == J J == 1 IN K", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := "---- MODULE B ----\nF(x) == 2\n" + tc.body + "\n====\n"
			source := "---- MODULE M ----\nEXTENDS A, B\nVARIABLE x\nInit == x = G\nNext == UNCHANGED x\n====\n"
			cfg := "INIT Init\nNEXT Next\n"
			loader := func(_ context.Context, name string) (string, error) {
				switch name {
				case "A":
					return a, nil
				case "B":
					return b, nil
				default:
					return "", os.ErrNotExist
				}
			}
			if jar != "" {
				dir := t.TempDir()
				for name, data := range map[string]string{"A.tla": a, "B.tla": b, "M.tla": source} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				checkCollisionSANY(t, dir, jar, tc.invalid)
			}
			result, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if tc.invalid && err == nil || !tc.invalid && (err != nil || result.Status != Success) {
				t.Fatalf("invalid=%t result=%+v error=%v", tc.invalid, result, err)
			}
		})
	}
}
