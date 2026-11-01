package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Java TLC 1.7.4 checks state and action constraints before computing VIEW.
// Excluded candidates still participate in invariant and action checks.
func TestJavaTLCViewConstraintOrder(t *testing.T) {
	jar := maxSetOracleJar(t)
	const next = "IF x = 0 THEN x' \\in 0..2 ELSE UNCHANGED x"
	for _, tc := range []struct {
		name, init, next, cfg string
		exit                  int
		prints                []string
		stats                 string
		prefixOnly            bool
	}{
		{
			"initial", "x \\in 0..2", "UNCHANGED x", "CONSTRAINT C\n", 0,
			[]string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 1>>`, `<<"constraint", 2>>`, `<<"view", 2>>`, `<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 2>>`, `<<"view", 2>>`},
			"5 states generated, 2 distinct states found, 0 states left on queue.", false,
		},
		{
			"state constraint", "x = 0", next, "CONSTRAINT C\n", 0,
			[]string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 1>>`, `<<"constraint", 2>>`, `<<"view", 2>>`, `<<"constraint", 2>>`, `<<"view", 2>>`},
			"5 states generated, 2 distinct states found, 0 states left on queue.", false,
		},
		{
			"action constraint", "x = 0", next, "ACTION_CONSTRAINT A\n", 0,
			[]string{`<<"view", 0>>`, `<<"action", 0, 0>>`, `<<"view", 0>>`, `<<"action", 0, 1>>`, `<<"action", 0, 2>>`, `<<"view", 2>>`, `<<"action", 2, 2>>`, `<<"view", 2>>`},
			"5 states generated, 2 distinct states found, 0 states left on queue.", false,
		},
		{
			"both constraints", "x = 0", next, "CONSTRAINT C\nACTION_CONSTRAINT A\n", 0,
			[]string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 0>>`, `<<"action", 0, 0>>`, `<<"view", 0>>`, `<<"constraint", 1>>`, `<<"constraint", 2>>`, `<<"action", 0, 2>>`, `<<"view", 2>>`, `<<"constraint", 2>>`, `<<"action", 2, 2>>`, `<<"view", 2>>`},
			"5 states generated, 2 distinct states found, 0 states left on queue.", false,
		},
		{
			"excluded initial invariant", "x \\in 0..2", "UNCHANGED x", "CONSTRAINT C\nINVARIANT I\n", 12,
			[]string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"invariant", 0>>`, `<<"constraint", 1>>`, `<<"invariant", 1>>`},
			"", false,
		},
		{
			"excluded successor invariant", "x = 0", next, "ACTION_CONSTRAINT A\nINVARIANT I\n", 12,
			[]string{`<<"view", 0>>`, `<<"invariant", 0>>`, `<<"action", 0, 0>>`, `<<"view", 0>>`, `<<"action", 0, 1>>`, `<<"invariant", 1>>`},
			"3 states generated, 1 distinct states found, 0 states left on queue.", true,
		},
		{
			"excluded action property", "x = 0", next, "ACTION_CONSTRAINT A\nPROPERTY ActionP\n", 13,
			[]string{`<<"view", 0>>`, `<<"action", 0, 0>>`, `<<"view", 0>>`, `<<"property", 0, 0>>`, `<<"action", 0, 1>>`, `<<"property", 0, 1>>`},
			"3 states generated, 1 distinct states found, 0 states left on queue.", true,
		},
		{
			"liveness", "x = 0", next, "CONSTRAINT C\nPROPERTY P\n", 0,
			[]string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 1>>`},
			"5 states generated, 2 distinct states found, 0 states left on queue.", true,
		},
		{
			"included failure", "x = 0", "x' = 1", "CONSTRAINT Included\n", 75,
			[]string{`<<"view", 0>>`, `<<"view", 1>>`, `<<"view", 1>>`},
			"2 states generated, 1 distinct states found, 0 states left on queue.", false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := viewConstraintFixture(tc.init, tc.next)
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\nCHECK_DEADLOCK FALSE\n"+tc.cfg)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			checkViewConstraintOutput(t, "Go", out, code, stderr, tc.exit, tc.prints, tc.stats, tc.prefixOnly)
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				checkViewConstraintOutput(t, "Java", java, javaCode, "", tc.exit, tc.prints, tc.stats, tc.prefixOnly)
				if (tc.exit == 12 || tc.exit == 13) && !reflect.DeepEqual(viewConstraintViolationMessages(out), viewConstraintViolationMessages(java)) {
					t.Fatalf("Go violations %v; Java violations %v", viewConstraintViolationMessages(out), viewConstraintViolationMessages(java))
				}
				if tc.name == "included failure" && !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go failure %q; Java failure %q", unionStateErrorMessages(out), unionStateErrorMessages(java))
				}
			}
		})
	}
}

func viewConstraintViolationMessages(output string) []toolMessage {
	var result []toolMessage
	for _, message := range readToolMessages(output) {
		if slices.Contains([]int{2107, 2110, 2112, 2116}, message.code) {
			result = append(result, message)
		}
	}
	return result
}

func viewConstraintFixture(init, next string) string {
	return "VARIABLE x\nInit == " + init + "\nNext == " + next + `
C == IF PrintT(<<"constraint", x>>) THEN x # 1 ELSE FALSE
A == IF PrintT(<<"action", x, x'>>) THEN x' # 1 ELSE FALSE
I == IF PrintT(<<"invariant", x>>) THEN x # 1 ELSE FALSE
Act == IF PrintT(<<"property", x, x'>>) THEN x' # 1 ELSE FALSE
ActionP == [][Act]_x
V == IF PrintT(<<"view", x>>) THEN IF x = 1 THEN UNION {1} ELSE x ELSE x
Included == TRUE
P == []<>(x \in {0, 2})`
}

func checkViewConstraintOutput(t *testing.T, implementation, output string, code int, stderr string, wantCode int, wantPrints []string, stats string, prefix bool) {
	t.Helper()
	if code != wantCode || stderr != "" || stats != "" && !strings.Contains(output, stats) {
		t.Fatalf("%s exit %d stderr %q output:\n%s", implementation, code, stderr, output)
	}
	prints := viewConstraintPrints(output)
	if wantCode == 0 && slices.Contains(prints, `<<"view", 1>>`) {
		t.Fatalf("%s evaluated VIEW for the excluded state: %q", implementation, prints)
	}
	if prefix && len(prints) >= len(wantPrints) {
		prints = prints[:len(wantPrints)]
	}
	if !slices.Equal(prints, wantPrints) {
		t.Fatalf("%s prints %q, want %q", implementation, prints, wantPrints)
	}
}

func viewConstraintPrints(output string) []string {
	var prints []string
	for line := range strings.SplitSeq(output, "\n") {
		for _, prefix := range []string{`<<"constraint",`, `<<"action",`, `<<"view",`, `<<"invariant",`, `<<"property",`} {
			if strings.HasPrefix(line, prefix) {
				prints = append(prints, line)
				break
			}
		}
	}
	return prints
}

func TestJavaTLCViewConstraintsMultipleWorkers(t *testing.T) {
	jar := maxSetOracleJar(t)
	dir := t.TempDir()
	body := viewConstraintFixture("x = 0", "IF x = 0 THEN x' \\in 0..2 ELSE UNCHANGED x")
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\nCHECK_DEADLOCK FALSE\nCONSTRAINT C\n")
	args := []string{"-tool", "-fp", "0", "-workers", "4", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	stats := "5 states generated, 2 distinct states found, 0 states left on queue."
	if code != 0 || stderr != "" || !strings.Contains(out, stats) || slices.Contains(viewConstraintPrints(out), `<<"view", 1>>`) {
		t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
	}
	if jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != 0 || !strings.Contains(java, stats) || slices.Contains(viewConstraintPrints(java), `<<"view", 1>>`) {
			t.Fatalf("Java exit %d output:\n%s", javaCode, java)
		}
	}
}

// Symmetry also fingerprints states, so excluded candidates must not be
// reduced even when their stored values cannot be fingerprinted.
func TestJavaTLCSymmetryConstraintBeforeFingerprint(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, view string }{
		{"symmetry", "(x = 0 /\\ y = {}) \\/ (x = 1 /\\ y = UNION {1})", ""},
		{"symmetry with view", "x \\in {0,1} /\\ y = A", "VIEW V\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS A, B\nVARIABLES x, y\nInit == " + tc.init + `
Next == UNCHANGED <<x, y>>
C == x = 0
V == IF x = 1 THEN UNION {1} ELSE x
Sym == Permutations({A, B})`
			cfg := "CONSTANTS A=A B=B\nINIT Init\nNEXT Next\nCONSTRAINT C\nSYMMETRY Sym\nCHECK_DEADLOCK FALSE\n" + tc.view
			writeDOTFixture(t, dir, body, cfg)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			stats := "3 states generated, 1 distinct states found, 0 states left on queue."
			if code != 0 || stderr != "" || !strings.Contains(out, stats) {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 || !strings.Contains(java, stats) {
					t.Fatalf("Java exit %d output:\n%s", javaCode, java)
				}
			}
		})
	}
}

// DFID and simulation use separate exploration paths. The breadth-first fix
// must leave their existing constraint behavior intact.
func TestJavaTLCViewConstraintsOtherModes(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, constraint := range []string{"CONSTRAINT C", "ACTION_CONSTRAINT A"} {
		for _, mode := range [][]string{{"-dfid", "3"}, {"-simulate", "num=1", "-depth", "3", "-seed", "1"}} {
			t.Run(constraint+"/"+mode[0], func(t *testing.T) {
				dir := t.TempDir()
				body := `VARIABLE x
Init == x = 0
Next == x' = 1
C == x # 1
A == x' # 1
V == IF x = 1 THEN UNION {1} ELSE x`
				writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\nCHECK_DEADLOCK FALSE\n"+constraint+"\n")
				args := append([]string{"-tool", "-workers", "1"}, mode...)
				args = append(args, "M.tla")
				out, code, stderr := runGoInDir(t, dir, args)
				if code != 0 || stderr != "" || strings.Contains(out, "Attempted to enumerate UNION") {
					t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
				}
				if jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, args)
					if javaCode != 0 || strings.Contains(java, "Attempted to enumerate UNION") {
						t.Fatalf("Java exit %d output:\n%s", javaCode, java)
					}
				}
			})
		}
	}
}

func TestJavaTLCViewConstraintEvaluationError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, condition, cfg string
		exit                 int
		prints               []string
	}{
		{"initial", "TRUE", "CONSTRAINT C\n", 75, []string{`<<"constraint", 0>>`}},
		{"successor", "x = 1", "CONSTRAINT C\n", 255, []string{`<<"constraint", 0>>`, `<<"view", 0>>`, `<<"constraint", 1>>`}},
		{"action", "FALSE", "ACTION_CONSTRAINT A\n", 255, []string{`<<"view", 0>>`, `<<"action", 0, 1>>`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := `VARIABLE x
Init == x = 0
Next == x' = 1
C == IF PrintT(<<"constraint", x>>) THEN IF ` + tc.condition + ` THEN 1 \div 0 = 0 ELSE TRUE ELSE FALSE
A == IF PrintT(<<"action", x, x'>>) THEN 1 \div 0 = 0 ELSE FALSE
V == IF PrintT(<<"view", x>>) THEN x ELSE x`
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\nCHECK_DEADLOCK FALSE\n"+tc.cfg)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			checkViewConstraintOutput(t, "Go", out, code, stderr, tc.exit, tc.prints, "", true)
			if slices.Contains(viewConstraintPrints(out), `<<"view", 1>>`) {
				t.Fatalf("Go evaluated VIEW after a constraint error:\n%s", out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				checkViewConstraintOutput(t, "Java", java, javaCode, "", tc.exit, tc.prints, "", true)
				if slices.Contains(viewConstraintPrints(java), `<<"view", 1>>`) {
					t.Fatalf("Java evaluated VIEW after a constraint error:\n%s", java)
				}
			}
		})
	}
}
