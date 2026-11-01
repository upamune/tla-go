package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Java TLC 1.7.4, upstream commit 5a47802. TLC_JAR enables the live oracle.
func TestJavaTLCEqualityOracle(t *testing.T) {
	cases := []struct{ expr, message string }{
		{`1 = <<>>`, "Attempted to check equality of integer 1 with non-integer:\n<<>>"},
		{`1 # <<>>`, "Attempted to check equality of integer 1 with non-integer:\n<<>>"},
		{`<<>> = 1`, "Attempted to check equality of the function << >> with the value:\n1"},
		{`<<>> # 1`, "Attempted to check equality of the function << >> with the value:\n1"},
		{`TRUE = 1`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`1 = TRUE`, "Attempted to check equality of integer 1 with non-integer:\nTRUE"},
		{`"a" = 1`, "Attempted to check equality of string \"a\" with non-string:\n1"},
		{`1 = "a"`, "Attempted to check equality of integer 1 with non-integer:\n\"a\""},
		{`[a |-> 1] = 1`, "Attempted to check equality of record:\n[a |-> 1]\nwith non-record\n1"},
		{`1 = [a |-> 1]`, "Attempted to check equality of integer 1 with non-integer:\n[a |-> 1]"},
		{`[i \in {1} |-> 1] = 1`, "Attempted to check equality of the function <<1>> with the value:\n1"},
		{`1 = [i \in {1} |-> 1]`, "Attempted to check equality of integer 1 with non-integer:\n<<1>>"},
		{`{1} = 1`, "Attempted to check equality of the set {1} with the value:\n1"},
		{`1 = {1}`, "Attempted to check equality of integer 1 with non-integer:\n{1}"},
		{`<<1>> = <<<<>>>>`, "Attempted to check equality of integer 1 with non-integer:\n<<>>"},
		{`<<TRUE>> = <<1>>`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`<<"a">> = <<1>>`, "Attempted to check equality of string \"a\" with non-string:\n1"},
		{`{1} = {<<>>}`, "Attempted to check equality of integer 1 with non-integer:\n<<>>"},
		{`[a |-> TRUE, b |-> 1] = [a |-> 1, c |-> 1]`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`[i \in {1,2} |-> IF i = 1 THEN TRUE ELSE 1] = [i \in {1,3} |-> 1]`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`{<<TRUE, 1>>} = ({1} \X {1})`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`{TRUE} = 1..1`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`1..1 = {TRUE}`, "Attempted to check equality of integer 1 with non-integer:\nTRUE"},
		{`{{TRUE}} = {1..1}`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`({1} \X {1}) = {<<TRUE, 1>>}`, "Attempted to check equality of integer 1 with non-integer:\nTRUE"},
		{`[{} -> {TRUE}] = [{} -> {1}]`, "Attempted to compare equality of boolean TRUE with non-boolean:\n1"},
		{`1 < <<>>`, "The second argument of < should be an integer, but instead it is:\n<<>>"},
		{`<<>> < 1`, "The first argument of < should be an integer, but instead it is:\n<<>>"},
		{`1 <= <<>>`, "The second argument of <= should be an integer, but instead it is:\n<<>>"},
		{`<<>> <= 1`, "The first argument of <= should be an integer, but instead it is:\n<<>>"},
		{`1 > <<>>`, "The second argument of > should be an integer, but instead it is:\n<<>>"},
		{`<<>> > 1`, "The first argument of > should be an integer, but instead it is:\n<<>>"},
		{`1 >= <<>>`, "The second argument of >= should be an integer, but instead it is:\n<<>>"},
		{`<<>> >= 1`, "The first argument of >= should be an integer, but instead it is:\n<<>>"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Integers\nASSUME " + tc.expr + "\n====\n"
			writeEqualityModel(t, dir, source, "")
			for _, tool := range []bool{false, true} {
				args := []string{"-workers", "1", "-fp", "0", "-checkpoint", "0"}
				if tool {
					args = append(args, "-tool")
				}
				args = append(args, filepath.Join(dir, "M.tla"))
				var out, stderr bytes.Buffer
				code := run(context.Background(), args, &out, &stderr)
				location := fmt.Sprintf("line 3, col 8 to line 3, col %d of module M", 7+len(tc.expr))
				if code != 10 || stderr.Len() != 0 || !strings.Contains(out.String(), tc.message) || !strings.Contains(out.String(), location) {
					t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					java := equalityJava(t, jar, dir, tool, 10)
					if !strings.Contains(java, tc.message) || !strings.Contains(java, location) {
						t.Fatalf("Java: %s", java)
					}
				}
			}
		})
	}
}

func TestJavaTLCFunctionSetEquality(t *testing.T) {
	for _, expr := range []string{
		`~([{} -> Nat] = [{} -> Int])`,
		`~([Nat -> {}] = [Int -> {}])`,
		`~([{} -> {1}] = [{} -> {2}])`,
		`~([{1} -> {}] = [{2} -> {}])`,
		`[{} -> Nat] = [{} -> Nat]`,
		`[1..2 -> {1}] = [{1, 2} -> 1..1]`,
		`[{} -> {1}] = {<<>>}`,
		`~([{1} -> {TRUE}] = [{2} -> {1}])`,
		`~({TRUE} = 1..2)`,
		`{1, 2} = 1..2`,
		`1..2 = {2, 1}`,
	} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != 0 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				equalityJava(t, jar, dir, false, 0)
			}
		})
	}
}

// Records the remaining intern-order mismatch without changing string ordering.
func TestJavaTLCStringInternEqualityGap(t *testing.T) {
	dir := t.TempDir()
	expr := `{<<"z", TRUE>>, <<"a", 1>>} = {<<"z", 1>>, <<"a", TRUE>>}`
	writeEqualityModel(t, dir, "---- MODULE M ----\nASSUME "+expr+"\n====\n", "")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
	if code != 10 || !strings.Contains(out.String(), "Attempted to check equality of integer 1 with non-integer:\nTRUE") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := equalityJava(t, jar, dir, false, 10)
		if !strings.Contains(java, "Attempted to compare equality of boolean TRUE with non-boolean:\n1") {
			t.Fatalf("Java: %s", java)
		}
	}
}

func TestJavaTLCIntervalEqualitySizes(t *testing.T) {
	for _, tc := range []struct{ expr, overflow string }{
		{`(0..2147483647) = (0..2147483647)`, "0..2147483647"},
		{`(0..2147483647) = (1..0)`, "0..2147483647"},
		{`(1..0) = (0..2147483647)`, "0..2147483647"},
		{`~((1..1) = (0..2147483647))`, ""},
		{`(0..2147483647) = (1..1)`, "0..2147483647"},
		{`(0..2147483647) = {1}`, "0..2147483647"},
		{`{1} = (0..2147483647)`, "0..2147483647"},
		{`(1..0) = {}`, ""},
		{`{} = (1..0)`, ""},
		{`~((1..0) = {1})`, ""},
		{`~({1} = (1..0))`, ""},
		{`(1..0) = (3..2)`, ""},
		{`~((1..0) = (1..1))`, ""},
		{`~((1..1) = (1..0))`, ""},
		{`(0..2147483646) = (0..2147483646)`, ""},
		{`~((0..2147483646) = (1..2147483647))`, ""},
		{`~((1..2147483647) = (0..2147483646))`, ""},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+tc.expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			want := 0
			message := ""
			if tc.overflow != "" {
				want = 10
				message = "Size of interval value exceeds the maximum representable size (32bits): " + tc.overflow + "."
			}
			if code != want || !strings.Contains(out.String(), message) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := equalityJava(t, jar, dir, false, want)
				if !strings.Contains(java, message) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCIntervalLazySetEqualityOrder(t *testing.T) {
	for _, tc := range []struct{ expr, message string }{
		{`LET f[a \in [k : {1}], x \in 1..50000, y \in 1..50000] == Print("body-marker", 1) IN f = f`, "Overflow when computing the number of elements"},
		{`LET f[x \in [1..31 -> {1,2}]] == Print("body-marker", 1) IN f = f`, "Overflow when computing the number of elements"},
		{`LET f[a \in [k : {1}, l : {2}], x \in 1..50000, y \in 1..50000] == Print("body-marker", 1) IN f = f`, "Overflow when computing the number of elements"},
		{`(0..2147483647) = (SUBSET (1..31))`, "Size of interval value exceeds the maximum representable size (32bits): 0..2147483647."},
		{`(SUBSET (1..31)) = (0..2147483647)`, "Overflow when computing the number of elements in:\nSUBSET 1..31"},
		{`(1..0) = (SUBSET (1..31))`, "Overflow when computing the number of elements in:\nSUBSET 1..31"},
		{`(SUBSET (1..31)) = (1..0)`, "Overflow when computing the number of elements in:\nSUBSET 1..31"},
		{`(-2147483647..1) = (SUBSET (1..31))`, "Size of interval value exceeds the maximum representable size (32bits): -2147483647..1."},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+tc.expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != 10 || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := equalityJava(t, jar, dir, false, 10)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCNestedEqualitySuccessorAndInvariant(t *testing.T) {
	for _, tc := range []struct {
		name, source, cfg, message string
		code                       int
	}{
		{"successor", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == /\\ <<1>> = <<<<>>>> /\\ x' = x\n====\n", "INIT Init\nNEXT Next\n", "Attempted to check equality of integer 1 with non-integer:\n<<>>", 75},
		{"invariant", "---- MODULE M ----\nVARIABLE x\nInit == x = <<>>\nNext == (x = <<>> /\\ x' = <<1, TRUE, \"a\">>) \\/ (x = <<1, TRUE, \"a\">> /\\ x' = <<<<1>>, FALSE, \"\">>) \\/ (x = <<<<1>>, FALSE, \"\">> /\\ UNCHANGED x)\nInv == x # <<<<1>>, FALSE, \"\">>\n====\n", "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n", "Attempted to check equality of integer 1 with non-integer:\n<<1>>", 76},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, tc.source, tc.cfg)
			for _, tool := range []bool{false, true} {
				args := []string{"-workers", "1", "-fp", "0", "-checkpoint", "0"}
				if tool {
					args = append(args, "-tool")
				}
				args = append(args, filepath.Join(dir, "M.tla"))
				var out, stderr bytes.Buffer
				code := run(context.Background(), args, &out, &stderr)
				if code != tc.code || !strings.Contains(out.String(), tc.message) || !strings.Contains(out.String(), "line 4, col") {
					t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
				}
				if tool && tc.name == "successor" && !strings.Contains(out.String(), "@!@!@STARTMSG 2103:1") {
					t.Fatalf("Go evaluation stack: %s", out.String())
				}
				if tool && tc.name == "invariant" && !strings.Contains(out.String(), "@!@!@STARTMSG 2111:1") {
					t.Fatalf("Go invariant code: %s", out.String())
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					java := equalityJava(t, jar, dir, tool, tc.code)
					if !strings.Contains(java, tc.message) || !strings.Contains(java, "line 4, col") {
						t.Fatalf("Java: %s", java)
					}
					// This records an outstanding source-provenance mismatch.
					if tool && tc.name == "successor" && (!strings.Contains(java, "@!@!@STARTMSG 2147:1") || !strings.Contains(java, "line 4, col 12 to line 4, col 16 of module M")) {
						t.Fatalf("Java fingerprint stack: %s", java)
					}
					if tool && tc.name == "invariant" && !strings.Contains(java, "@!@!@STARTMSG 2111:1") {
						t.Fatalf("Java invariant code: %s", java)
					}
				}
			}
		})
	}
}

func writeEqualityModel(t *testing.T, dir, source, cfg string) {
	t.Helper()
	for name, data := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func equalityJava(t *testing.T, jar, dir string, tool bool, want int) string {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-XX:+UseParallelGC", "-Xmx256m", "-jar", jar, "-workers", "1", "-fp", "0", "-checkpoint", "0"}
	if tool {
		args = append(args, "-tool")
	}
	args = append(args, "M.tla")
	javaDir := t.TempDir()
	for _, name := range []string{"M.tla", "M.cfg"} {
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(javaDir, name), data, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	cmd := exec.Command("java", args...)
	cmd.Dir = javaDir
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); (want == 0 && err != nil) || (want != 0 && (!ok || exit.ExitCode() != want)) {
		t.Fatalf("Java: %v\n%s", err, out)
	}
	return string(out)
}

func TestJavaTLCEqualityControls(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Integers\nCONSTANT M\nVARIABLE x\nASSUME /\\ M # 1 /\\ 1 # M /\\ <<1, TRUE>> = <<1, TRUE>> /\\ {1, 2} = {2, 1}\nInit == x = 0\nNext == x' = x\nInv == /\\ 1 < 2 /\\ <<1, 1>> # <<2, <<>>>> /\\ {1} # {1, 2}\n====\n"
	writeEqualityModel(t, dir, source, "CONSTANT M = M\nINIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{"-workers", "1", "-fp", "0", "-checkpoint", "0", filepath.Join(dir, "M.tla")}, &out, &stderr)
	if code != 0 || !strings.Contains(out.String(), "No error has been found") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := equalityJava(t, jar, dir, false, 0)
		if !strings.Contains(java, "No error has been found") {
			t.Fatalf("Java: %s", java)
		}
	}
}

// Typed model comparison is still unsupported; preserve its status and record
// Java's directional diagnostics until the whole model-value path is handled.
func TestJavaTLCTypedModelEqualityGap(t *testing.T) {
	for _, tc := range []struct{ expr, message string }{
		{`M_1 = 1`, "Attempted to check equality of typed model value M_1 and non-model value\n1"},
		{`1 = M_1`, "Attempted to check equality of the typed model value M_1 and the non-model value\n1"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nCONSTANT M_1\nASSUME "+tc.expr+"\n====\n", "CONSTANT M_1 = M_1\n")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != 255 || !strings.Contains(out.String()+stderr.String(), "cannot compare incompatible typed model values") {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := equalityJava(t, jar, dir, false, 10)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCEqualityNormalizationAndLazyFunctions(t *testing.T) {
	cases := []struct {
		name, body, message string
		code                int
	}{
		{"normalize", `ASSUME Cardinality({[{} -> {1}], [{} -> {2}]}) = 1`, "", 0},
		{"nontransitive", `ASSUME /\ ~([{} -> {1}] = [{} -> {2}]) /\ {[{} -> {1}]} = {[{} -> {1}], [{} -> {2}]}`, "", 0},
		{"lazy different domains", "F[i \\in {1, 2}] == IF i = 1 THEN TRUE ELSE 1\nG[i \\in {1, 3}] == 1\nASSUME F = G", "Attempted to compare equality of boolean TRUE with non-boolean:\n1", 10},
		{"lazy equal domains", "F[i \\in {1}] == TRUE\nG[i \\in {1}] == 1\nASSUME F = G", "Attempted to compare equality of boolean TRUE with non-boolean:\n1", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, FiniteSets\n"+tc.body+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := equalityJava(t, jar, dir, false, tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCLazyEqualityEvaluationOrder(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Integers, TLC
F[i \in {1, 2}] == Print(<<"left", i>>, i)
G[i \in {1, 2}] == Print(<<"right", i>>, i)
ASSUME F = G
====
`
	writeEqualityModel(t, dir, source, "")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
	check := func(name, output string) {
		t.Helper()
		last := -1
		for _, marker := range []string{`<<"left", 1>>`, `<<"left", 2>>`, `<<"right", 1>>`, `<<"right", 2>>`} {
			pos := strings.Index(output, marker)
			if pos <= last || strings.Count(output, marker) != 1 {
				t.Fatalf("%s marker %s: %s", name, marker, output)
			}
			last = pos
		}
	}
	if code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	check("Go", out.String())
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		check("Java", equalityJava(t, jar, dir, false, 0))
	}
}

func TestJavaTLCInfiniteFunctionEquality(t *testing.T) {
	for _, expr := range []string{
		`LET f[n \in Nat] == n IN f = f`,
		`LET f[n \in Nat] == n IN <<f>> = <<f>>`,
		`\A i \in 1..3 : LET f[n \in Nat] == n + i IN /\ f = f /\ f[0] = i`,
		`LET f[n \in Nat] == n IN LET g[n \in Nat] == n IN f = g`,
	} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			want := "Attempted to compute the number of elements in the overridden value Nat."
			if code != 10 || !strings.Contains(out.String(), want) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if java := equalityJava(t, jar, dir, false, 10); !strings.Contains(java, want) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCAnyFunctionMaterialization(t *testing.T) {
	// Java also wraps the @@ failure in an overridden-operator diagnostic.
	// Go currently reports the underlying error without that context.
	for _, expr := range []string{
		`LET f[x \in Any] == x IN f = f`,
		`LET f[x \in Any] == x IN (f @@ (1 :> 1)) = f`,
	} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS TLC\nASSUME "+expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			want := "Attempted to compute the number of elements in the overridden value ANY."
			if code != 10 || !strings.Contains(out.String(), want) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if java := equalityJava(t, jar, dir, false, 10); !strings.Contains(java, want) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCFiniteFunctionCombine(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
ASSUME LET f[x \in 1..2] == x IN (f @@ (2 :> 9) @@ (3 :> 3)) = <<1, 2, 3>>
====
`
	writeEqualityModel(t, dir, source, "")
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		equalityJava(t, jar, dir, false, 0)
	}
}

func TestJavaTLCFiniteFunctionSelfEqualityEffects(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Integers, TLC\nF[i \\in 1..2] == Print(i, i)\nASSUME F = F\n====\n"
	writeEqualityModel(t, dir, source, "")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
	check := func(name, output string) {
		t.Helper()
		var effects []string
		for line := range strings.SplitSeq(output, "\n") {
			if line == "1  1" || line == "2  2" {
				effects = append(effects, line[:1])
			}
		}
		if strings.Join(effects, ",") != "1,2" {
			t.Fatalf("%s effects %v; output %s", name, effects, output)
		}
	}
	if code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	check("Go", out.String())
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		check("Java", equalityJava(t, jar, dir, false, 0))
	}
}

// This characterizes an existing operator-definition caching difference.
func TestJavaTLCLazyLetSideEffectsGap(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Integers, TLC
ASSUME LET F == [i \in 1..2 |-> Print(i, i)] IN /\ F[1] = 1 /\ F = F
====
`
	writeEqualityModel(t, dir, source, "")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
	check := func(name, output, want string) {
		t.Helper()
		// Print's output is on a line by itself, unlike TLC's progress messages.
		lines := strings.Split(output, "\n")
		var effects []string
		for _, line := range lines {
			if line == "1  1" || line == "2  2" {
				effects = append(effects, line[:1])
			}
		}
		if strings.Join(effects, ",") != want {
			t.Fatalf("%s effects %v; output %s", name, effects, output)
		}
	}
	if code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	check("Go", out.String(), "1,1,2,1,2")
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		check("Java", equalityJava(t, jar, dir, false, 0), "1,1,2")
	}
}

func TestJavaTLCProductFunctionInfiniteDomains(t *testing.T) {
	cases := []struct{ expr, infinite string }{
		{`LET f[x \in Nat, y \in Nat] == x IN f = f`, "Nat"},
		{`LET f[x \in Int, y \in Nat] == x IN f = f`, "Int"},
		{`LET f[x \in Nat, y \in Any] == x IN <<f>> = <<f>>`, "Nat"},
		{`LET f[x \in {1}, y \in Any] == x IN f = f`, "ANY"},
		{`LET f[x \in {}, y \in Nat] == x IN f = f`, "Nat"},
		{`LET f[x \in Nat, y \in {}] == x IN f = f`, "Nat"},
		{`LET f[x \in ({} \X (Int \X Nat))] == x IN f = f`, "Int"},
		{`LET f[<<x,y>> \in ({} \X Nat)] == x IN f = f`, "Nat"},
		{`LET f[x \in Print("bound-one", {}), y \in Print("bound-two", Nat)] == Print("body-marker", x) IN f = f`, "Nat"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME "+tc.expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			want := "Attempted to compute the number of elements in the overridden value " + tc.infinite + "."
			check := func(name, output string) {
				t.Helper()
				if !strings.Contains(output, want) {
					t.Fatalf("%s missing %q: %s", name, want, output)
				}
				if strings.Contains(tc.expr, "bound-one") {
					a, b := strings.Index(output, `"bound-one"`), strings.Index(output, `"bound-two"`)
					if a < 0 || b <= a || strings.Contains(output, `"body-marker"`) {
						t.Fatalf("%s effects: %s", name, output)
					}
				}
			}
			if code != 10 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			check("Go", out.String())
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				check("Java", equalityJava(t, jar, dir, false, 10))
			}
		})
	}
}

func TestJavaTLCProductFunctionFiniteControls(t *testing.T) {
	for _, expr := range []string{
		`LET f[x \in {}, y \in 1..2000000000] == 1 \div 0 IN f = f`,
		`LET f[x \in 1..2, y \in 1..2] == x + y IN f[2, 1] = 3 /\ f = f`,
		`LET f[x \in [{} -> {1,2}]] == 1 IN f = f`,
		`LET f[x \in [1..2 -> {}]] == 1 \div 0 IN f = f`,
		`LET f[x \in [a : {1}, b : {2}], y \in {3}] == x.a + y IN f = f`,
	} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 0 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				equalityJava(t, jar, dir, false, 0)
			}
		})
	}
}

func TestJavaTLCProductFunctionSizeOrder(t *testing.T) {
	for _, tc := range []struct{ expr, message string }{
		{`LET f[x \in 1..50000, y \in 1..50000, z \in {}] == Print("body-marker", 1) IN f = f`, "Overflow when computing the number of elements"},
		{`LET f[x \in (1..50000) \X (1..50000), y \in Nat] == Print("body-marker", 1) IN f = f`, "Overflow when computing the number of elements"},
		{`LET f[x \in 0..2147483647, y \in Nat] == 1 IN f = f`, "Size of interval value exceeds the maximum representable size"},
		{`LET f[x \in SUBSET (1..31), y \in Nat] == 1 IN f = f`, "Overflow when computing the number of elements in:\nSUBSET 1..31"},
		{`LET f[x \in SUBSET {1,2}, y \in {}, z \in Nat] == 1 IN f = f`, "Attempted to compute the number of elements in the overridden value Nat."},
		{`LET f[x \in [1..2 -> {1,2}], y \in {}, z \in Nat] == 1 IN f = f`, "Attempted to compute the number of elements in the overridden value Nat."},
		{`LET f[x \in [{} -> Nat], y \in Int] == 1 IN f = f`, "Attempted to compute the number of elements in the overridden value Nat."},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, FiniteSets, TLC\nASSUME "+tc.expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != 10 || !strings.Contains(out.String(), tc.message) || strings.Contains(out.String(), `"body-marker"`) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := equalityJava(t, jar, dir, false, 10)
				if !strings.Contains(java, tc.message) || strings.Contains(java, `"body-marker"`) {
					t.Fatalf("Java: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCRecordSetSizeOrder(t *testing.T) {
	for _, tc := range []struct{ domain, message string }{
		{`[a : {}, b : Nat]`, "overridden value Nat"},
		{`[b : Nat, a : {}]`, "overridden value Nat"},
		{`[a : 1..50000, b : 1..50000]`, "Overflow when computing the number of elements"},
		{`[b : 1..50000, a : 1..50000]`, "Overflow when computing the number of elements"},
		{`[a : [p : {}, q : Nat], b : {1}]`, "overridden value Nat"},
		{`[b : {1}, a : [q : 1..50000, p : 1..50000]]`, "Overflow when computing the number of elements"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			dir := t.TempDir()
			expr := `LET f[x \in ` + tc.domain + `, y \in {1}] == Print("body-marker", 1 \div 0) IN f = f`
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME "+expr+"\n====\n", "")
			check := func(name, output string) {
				t.Helper()
				if !strings.Contains(output, tc.message) || strings.Contains(output, `"body-marker"`) || strings.Contains(output, "argument of \\div is 0") {
					t.Fatalf("%s: %s", name, output)
				}
			}
			var out, stderr bytes.Buffer
			if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 10 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			check("Go", out.String())
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				check("Java", equalityJava(t, jar, dir, false, 10))
			}
		})
	}
}

func TestJavaTLCSingleRecordFunctionDomain(t *testing.T) {
	for _, tc := range []struct{ domain, message string }{
		{`[a : {}, b : Nat]`, "overridden value Nat"},
		{`[b : Nat, a : {}]`, "overridden value Nat"},
		{`[a : 1..50000, b : 1..50000]`, "Overflow when computing the number of elements"},
		{`[a : [p : {}, q : Nat], b : {1}]`, "overridden value Nat"},
		{`[a : [p : 1..50000, q : 1..50000], b : {1}]`, "Overflow when computing the number of elements"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			dir := t.TempDir()
			expr := `LET f[x \in ` + tc.domain + `] == Print("body-marker", 1 \div 0) IN f = f`
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME "+expr+"\n====\n", "")
			check := func(name, output string) {
				t.Helper()
				if !strings.Contains(output, tc.message) || strings.Contains(output, `"body-marker"`) || strings.Contains(output, "argument of \\div is 0") {
					t.Fatalf("%s: %s", name, output)
				}
			}
			var out, stderr bytes.Buffer
			if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 10 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			check("Go", out.String())
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				check("Java", equalityJava(t, jar, dir, false, 10))
			}
		})
	}
}

func TestJavaTLCSingleRecordFunctionFinite(t *testing.T) {
	for _, expr := range []string{
		`LET f[x \in [a : {}, b : {1}]] == 1 \div 0 IN f = f`,
		`LET f[x \in [a : {1,2}, b : {3}]] == x.a IN f = f /\ f[[a |-> 2, b |-> 3]] = 2`,
	} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers\nASSUME "+expr+"\n====\n", "")
			var out, stderr bytes.Buffer
			if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 0 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				equalityJava(t, jar, dir, false, 0)
			}
		})
	}
}

func TestJavaTLCRecordSetMultipleErrorsInternOrder(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR not set")
	}
	for _, tc := range []struct{ expr, first string }{
		{`LET f[x \in [a : Nat, b : Int], y \in {1}] == Print("body-marker", 1) IN f = f`, "Nat"},
		{`LET f[x \in [b : Int, a : Nat], y \in {1}] == Print("body-marker", 1) IN f = f`, "Int"},
		{`LET z == [b |-> 1, a |-> 2] IN LET f[x \in [a : Nat, b : Int], y \in {1}] == Print("body-marker", 1) IN f = f`, "Int"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			dir := t.TempDir()
			writeEqualityModel(t, dir, "---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME "+tc.expr+"\n====\n", "")
			output := equalityJava(t, jar, dir, false, 10)
			if !strings.Contains(output, "overridden value "+tc.first+".") || strings.Contains(output, `"body-marker"`) {
				t.Fatalf("Java: %s", output)
			}
		})
	}
}

func TestJavaTLCFunctionSetDomainFiniteEffects(t *testing.T) {
	dir := t.TempDir()
	writeEqualityModel(t, dir, `---- MODULE M ----
EXTENDS Integers, TLC
ASSUME LET f[x \in Print("bound", [1..1 -> {1}])] == Print("body", 1) IN f = f
====
`, "")
	check := func(name, output string) {
		t.Helper()
		bound, body := strings.Index(output, `"bound"`), strings.Index(output, `"body"`)
		if bound < 0 || body <= bound || strings.Count(output, `"body"`) != 1 {
			t.Fatalf("%s effects: %s", name, output)
		}
	}
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	check("Go", out.String())
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		check("Java", equalityJava(t, jar, dir, false, 0))
	}
}

func TestJavaTLCProductFunctionBodyOrderGap(t *testing.T) {
	dir := t.TempDir()
	writeEqualityModel(t, dir, `---- MODULE M ----
EXTENDS Integers, TLC
ASSUME LET f[x \in {2,1}, y \in {4,3}] == Print("body", <<x,y>>) IN f = f
====
`, "")
	check := func(name, output string, want []string) {
		t.Helper()
		last := -1
		for _, tuple := range want {
			marker := `"body"  ` + tuple
			pos := strings.Index(output, marker)
			if pos <= last || strings.Count(output, marker) != 1 {
				t.Fatalf("%s body order: %s", name, output)
			}
			last = pos
		}
	}
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &stderr); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), stderr.String())
	}
	check("Go", out.String(), []string{"<<1, 3>>", "<<1, 4>>", "<<2, 3>>", "<<2, 4>>"})
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		check("Java", equalityJava(t, jar, dir, false, 0), []string{"<<1, 3>>", "<<2, 3>>", "<<1, 4>>", "<<2, 4>>"})
	}
}
