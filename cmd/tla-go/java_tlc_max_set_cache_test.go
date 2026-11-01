package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJavaTLCMaxSetSizeCachedUnionRegression(t *testing.T) {
	jar := maxSetOracleJar(t)
	dir := t.TempDir()
	body := "VARIABLE x\nF(S) == IF S = {1,2,3,4} THEN Cardinality(UNION {S, 2..7}) ELSE 0\nInit == x = F(UNION {1..3, 2..4})\nNext == UNCHANGED x\nCorrect == x = 7"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Correct\n")
	args := maxSetOracleArgs()
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" || !strings.Contains(out, "2 states generated, 1 distinct states found") {
		t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
	}
	if jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != 0 || !strings.Contains(java, "2 states generated, 1 distinct states found") {
			t.Fatalf("Java exit %d output:\n%s", javaCode, java)
		}
	}
}

// SetEnumValue.equals calls toSetEnum on the right operand without caching it;
// UnionValue.equals caches its own result. This remains a Go compatibility gap.
func TestJavaTLCMaxSetSizeReversedComparisonOracle(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR for the Java-only reference")
	}
	dir := t.TempDir()
	body := "VARIABLE x\nF(S) == IF {1,2,3,4} = S THEN Cardinality(UNION {S, 2..7}) ELSE 0\nInit == x = F(UNION {1..3, 2..4})\nNext == UNCHANGED x"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	out, code := runJavaTLCArgs(t, jar, dir, maxSetOracleArgs())
	if code != 75 || !strings.Contains(out, "@!@!@STARTMSG 2154:0") || !strings.Contains(out, "too many elements (>10)") {
		t.Fatalf("Java exit %d output:\n%s", code, out)
	}
}

func TestJavaTLCMaxSetSizeCupAssertionOrderRegression(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, set := range []string{
		"(UNION {1..6, 2..7}) \\cup {100}",
		"{100} \\cup (UNION {1..6, 2..7})",
	} {
		t.Run(set, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = Cardinality({i \\in " + set + " : IF i = 100 THEN Assert(FALSE, \"marker\") ELSE TRUE})\nNext == UNCHANGED x"
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := maxSetOracleArgs()
			out, code, stderr := runGoInDir(t, dir, args)
			checkMaxSetAssertion(t, "Go", out, code, stderr)
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				checkMaxSetAssertion(t, "Java", java, javaCode, "")
				if got, want := errorBehaviorMessages(readToolMessages(out)), errorBehaviorMessages(readToolMessages(java)); !reflect.DeepEqual(got, want) {
					t.Fatalf("Go errors %q; Java errors %q", got, want)
				}
			}
		})
	}
}

func checkMaxSetAssertion(t *testing.T, engine, out string, code int, stderr string) {
	t.Helper()
	if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2132:0") || !strings.Contains(out, `"marker"`) || strings.Contains(out, "too many elements") {
		t.Fatalf("%s exit %d stderr %q output:\n%s", engine, code, stderr, out)
	}
}

// Go's current error is a known compatibility gap; this asserts it alongside
// the absence of output so a different early failure cannot masquerade as it.
func TestJavaTLCMaxSetSizeCupIteratorSetupRegression(t *testing.T) {
	jar := maxSetOracleJar(t)
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = Cardinality({i \\in (UNION {1..6,2..7}) \\cup (SUBSET Nat) : PrintT(i)})\nNext == UNCHANGED x"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := maxSetOracleArgs()
	out, code, stderr := runBoundedMaxSetCLI(t, dir)
	if code != 255 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 1000:1 @!@!@\nexpected an enumerable set\n@!@!@ENDMSG 1000 @!@!@") || maxSetInitialPrints(t, out, "@!@!@STARTMSG 1000:") != "" {
		t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
	}
	if jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != 75 || maxSetInitialPrints(t, java, "@!@!@STARTMSG 1000:") != "" || !strings.Contains(java, "Attempted to compute the number of elements in the overridden value Nat") {
			t.Fatalf("Java exit %d output:\n%s", javaCode, java)
		}
	}
}

// These are Java-only observations. Go currently normalizes some of these
// enumerators earlier and does not yet preserve their complete PrintT order.
func TestJavaTLCMaxSetSizeSetOperationOrderOracle(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR for the Java-only reference")
	}
	for _, tc := range []struct{ name, set, prints string }{
		{"reducible right", "(UNION {1..4,2..5}) \\cup {100}", "100\n1\n2\n3\n4\n2\n3\n4\n5\n"},
		{"reducible left", "{100} \\cup (UNION {1..4,2..5})", "100\n1\n2\n3\n4\n2\n3\n4\n5\n"},
		{"both deferred", "(UNION {1..3,2..4}) \\cup (UNION {100..101,101..102})", "1\n2\n3\n2\n3\n4\n100\n101\n101\n102\n"},
		{"both deferred reversed", "(UNION {100..101,101..102}) \\cup (UNION {1..3,2..4})", "100\n101\n101\n102\n1\n2\n3\n2\n3\n4\n"},
		{"both eager", "{1,2} \\cup {2,3}", "1\n2\n3\n"},
		{"intersection right", "(UNION {1..4,2..5}) \\cap {2,3,100}", "2\n3\n"},
		{"intersection left", "{2,3,100} \\cap (UNION {1..4,2..5})", "2\n3\n"},
		{"difference deferred", "(UNION {1..4,2..5}) \\ {1,4}", "2\n3\n2\n3\n5\n"},
		{"difference reducible", "{100} \\ (UNION {1..4,2..5})", "100\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = Cardinality({i \\in " + tc.set + " : PrintT(i)})\nNext == UNCHANGED x"
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			out, code := runJavaTLCArgs(t, jar, dir, maxSetOracleArgs())
			if code != 0 || maxSetInitialPrints(t, out, "@!@!@STARTMSG 2190:") != tc.prints {
				t.Fatalf("Java exit %d output:\n%s", code, out)
			}
		})
	}
}

func maxSetOracleJar(t *testing.T) string {
	t.Helper()
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		return ""
	}
	absolute, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func maxSetOracleArgs() []string {
	return []string{"-tool", "-deadlock", "-workers", "1", "-checkpoint", "0", "-maxSetSize", "10", "M.tla"}
}

func maxSetInitialPrints(t *testing.T, output, marker string) string {
	t.Helper()
	_, after, found := strings.Cut(output, "@!@!@ENDMSG 2189 @!@!@\n")
	if !found {
		t.Fatalf("initial state message missing: %s", output)
	}
	prints, _, found := strings.Cut(after, marker)
	if !found {
		t.Fatalf("message %s missing: %s", marker, output)
	}
	return prints
}
