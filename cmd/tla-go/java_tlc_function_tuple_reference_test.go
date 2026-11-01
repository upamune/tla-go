package main

import (
	"os"
	"strings"
	"testing"
)

// Java TLC 1.7.4's FcnParams.Enumerator primes a tuple-bound iterator using
// the source-group index, rather than the flattened argument index. These are
// reference observations, not Go compatibility tests: the general iterator
// and its resulting failure timing are not implemented yet.
func TestJavaTLCFunctionTupleBoundReference(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	tests := []struct {
		name, bounds, exception, detail, statistics, prints string
		code                                                int
	}{
		{"integer iterator", `a, b \in 1..2, <<i,j>> \in {<<3,4>>,<<5,6>>}`, "java.lang.ClassCastException", "tlc2.value.impl.IntValue cannot be cast to class tlc2.value.impl.TupleValue", "1 states generated, 1 distinct states found", "", 75},
		{"singleton iterator", `a, b \in 1..1, <<i,j>> \in {<<3,4>>,<<5,6>>}`, "java.lang.RuntimeException", "java.lang.NullPointerException", "2 states generated, 1 distinct states found", "", 75},
		{"empty preceding bound", `a, b \in 1..0, <<i,j>> \in {<<3,4>>,<<5,6>>}`, "", "", "3 states generated, 2 distinct states found", "", 0},
		{"tuple iterator", `a, b \in {<<7,8>>,<<9,10>>}, <<i,j>> \in {<<3,4>>,<<5,6>>}`, "java.lang.ArrayIndexOutOfBoundsException", "Index 8 out of bounds for length 8", "1 states generated, 1 distinct states found", "<<9, 10>>\n<<9, 10>>\n<<3, 4>>\n<<3, 4>>\n<<3, 4>>\n<<3, 4>>\n<<5, 6>>\n<<5, 6>>\n", 75},
		{"separate bounds", `a \in 1..2, b \in 1..2, <<i,j>> \in {<<3,4>>,<<5,6>>}`, "", "", "3 states generated, 2 distinct states found", "", 0},
		{"empty tuple bound", `a, b \in 1..2, <<i,j>> \in {}`, "java.lang.ArrayIndexOutOfBoundsException", "Index 0 out of bounds for length 0", "1 states generated, 1 distinct states found", "", 75},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = [a \\in {1} |-> 0]\nNext == x' = [" + tc.bounds + " |-> IF PrintT(<<i,j>>) THEN 0 ELSE 1]"
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
			output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "M.tla"})
			if code != tc.code || !strings.Contains(rawToolMessage(output, 2199), tc.statistics) {
				t.Fatalf("exit %d, want %d; output:\n%s", code, tc.code, output)
			}
			message := rawToolMessage(output, 1000)
			if tc.exception != "" && (!strings.Contains(message, tc.exception) || !strings.Contains(message, tc.detail)) {
				t.Fatalf("wrong exception:\n%s", output)
			}
			if tc.prints != "" && strings.Count(output, tc.prints) != 2 {
				t.Fatalf("wrong evaluation or replay order:\n%s", output)
			}
		})
	}
}

func TestJavaTLCFunctionTupleBoundDirectApplicationReference(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	body := `VARIABLE x
Init == x = [a,b \in 1..2, <<i,j>> \in {<<3,4>>,<<5,6>>} |-> 0][1,1,<<3,4>>]
Next == UNCHANGED x`
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "M.tla"})
	if code != 0 || !strings.Contains(rawToolMessage(output, 2199), "2 states generated, 1 distinct states found") {
		t.Fatalf("direct application: exit %d\n%s", code, output)
	}
}

// SetPredValue.size caches the enumerated set before the buggy function
// enumerator runs. Failed function rendering falls back to the source form;
// neither step repeats the predicate's observable effects.
func TestJavaTLCFunctionTupleBoundDeferredEffectsReference(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	body := `VARIABLE x
D == {z \in SUBSET {1,2} : PrintT(z)}
F == [a,b \in D, <<i,j>> \in {<<3,4>>,<<5,6>>} |-> 0]
Init == x = 0 /\ PrintT(F)
Next == UNCHANGED x`
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "M.tla"})
	prints := "{}\n{1}\n{2}\n{1, 2}\n[a \\in {{}, {1}, {2}, {1, 2}}, <<i, j>> \\in {<<3, 4>>, <<5, 6>>} |-> <expression 0>]"
	if code != 0 || strings.Count(output, prints) != 1 || strings.Count(output, "\n{1, 2}\n") != 1 {
		t.Fatalf("deferred function effects: exit %d\n%s", code, output)
	}
}
