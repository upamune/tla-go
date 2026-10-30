package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCSpecialProofSteps(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		code        int
	}{
		{"pick", "<1>1. PICK x: x=x\n<1>2. x=x BY <1>1\n<1> QED", 0},
		{"pick child scope", "<1>1. PICK x: x=x\n <2>1. x=x\n <2> QED\n<1> QED", 150},
		{"pick proof scope", "<1>1. PICK x: x=x BY x=x\n<1> QED", 150},
		{"pick bounded", "<1>1. PICK x \\in {1,2}: x=x\n<1>2. x=x\n<1> QED", 0},
		{"pick tuple", "<1>1. PICK <<x,y>> \\in {<<1,2>>}: x=y\n<1>2. x=x /\\ y=y\n<1> QED", 0},
		{"pick grouped bounds", "<1>1. PICK x, y \\in {1}: x=y\n<1>2. x=x\n<1> QED", 0},
		{"pick dependent bounds", "<1>1. PICK x \\in {1}, y \\in {x}: x=y\n<1> QED", 150},
		{"pick forward bound", "<1>1. PICK x \\in {y}, y \\in {1}: x=y\n<1> QED", 150},
		{"pick parent proof scope", "<1>1. TAKE y\n<1>2. PICK x: x=y BY y=y\n<1> QED", 0},
		{"pick parent bound scope", "<1>1. TAKE y\n<1>2. PICK x \\in {y}: TRUE\n<1> QED", 0},
		{"pick self bound", "<1>1. PICK x \\in {x}: TRUE\n<1> QED", 150},
		{"pick proof refers to step", "<1>1. PICK x: TRUE BY <1>1\n<1> QED", 0},
		{"pick unbounded tuple", "<1>1. PICK <<x,y>>: TRUE\n<1> QED", 150},
		{"pick duplicate", "<1>1. PICK x, x: TRUE\n<1> QED", 150},
		{"pick duplicate later", "<1>1. PICK x: TRUE\n<1>2. PICK x: TRUE\n<1> QED", 150},
		{"pick scope does not escape", "<1>1. TRUE\n <2>1. PICK x: TRUE\n <2> QED\n<1>2. x=x\n<1> QED", 150},
		{"take", "<1>1. TAKE x\n<1>2. x=x\n<1> QED", 0},
		{"take bounded", "<1>1. TAKE x \\in {1}\n<1>2. x=x\n<1> QED", 0},
		{"take tuple", "<1>1. TAKE <<x,y>> \\in {<<1,2>>}\n<1>2. x=x /\\ y=y\n<1> QED", 0},
		{"take grouped bounds", "<1>1. TAKE x, y \\in {1}\n<1>2. x=y\n<1> QED", 0},
		{"take dependent bounds", "<1>1. TAKE x \\in {1}, y \\in {x}\n<1> QED", 150},
		{"take proof", "<1>1. TAKE x BY TRUE\n<1> QED", 150},
		{"take fact", "<1>1. TAKE x\n<1>2. TRUE BY <1>1\n<1> QED", 0},
		{"take duplicate", "<1>1. TAKE x, x\n<1> QED", 150},
		{"take scope does not escape", "<1>1. TRUE\n <2>1. TAKE x\n <2> QED\n<1>2. x=x\n<1> QED", 150},
		{"have", "<1>1. HAVE TRUE\n<1>2. TRUE BY <1>1\n<1> QED", 0},
		{"have proof", "<1>1. HAVE TRUE BY TRUE\n<1> QED", 150},
		{"have undefined", "<1>1. HAVE Missing\n<1> QED", 150},
		{"witness", "<1>1. WITNESS 1, TRUE\n<1> QED", 0},
		{"witness fact", "<1>1. WITNESS 1\n<1>2. TRUE BY <1>1\n<1> QED", 0},
		{"witness proof", "<1>1. WITNESS 1 BY TRUE\n<1> QED", 150},
		{"witness undefined", "<1>1. WITNESS Missing\n<1> QED", 150},
		{"witness membership", "<1>1. WITNESS 1 \\in {1}\n<1> QED", 0},
		{"case", "<1>1. CASE TRUE\n<1>2. TRUE BY <1>1\n<1> QED", 0},
		{"case proof", "<1>1. CASE TRUE BY TRUE\n<1> QED", 0},
		{"case nested proof", "<1>1. CASE TRUE\n <2>1. TRUE BY <1>1\n <2> QED\n<1> QED", 0},
		{"case undefined", "<1>1. CASE Missing\n<1> QED", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nTHEOREM T == TRUE\n" + tc.proof + "\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

func TestJavaTLCSpecialProofStepsModel(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE v
Init == v = 0
Next == v' = 1 - v
THEOREM T == TRUE
<1>1. TAKE y \in {0, 1}
<1>2. PICK x \in {y}: x = y BY y = y
<1>3. HAVE x = y
<1>4. WITNESS x, y
<1>5. CASE x = y BY <1>3
<1> QED
====
`
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	stats := "3 states generated, 2 distinct states found, 0 states left on queue."
	if code != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	var metadata, metaErr bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", filepath.Join(dir, "M.tla")}, &metadata, &metaErr); code != 0 {
		t.Fatalf("metadata exit %d: %s", code, metaErr.String())
	}
	if level, ok := summarizeSANY(t, metadata.Bytes()).definitions["T"]; !ok || level != 0 {
		t.Fatalf("theorem level %d (present %t), want 0", level, ok)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java output: %s", java)
		}
	}
}
