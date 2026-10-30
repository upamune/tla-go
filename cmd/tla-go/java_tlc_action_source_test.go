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

func TestJavaTLCBranchActionSources(t *testing.T) {
	cases := []struct {
		name, source, cfg string
		exit              int
		markers           []string
	}{
		{
			"Branches", `---- MODULE Branches ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == \/ /\ x = 0 /\ x' = 1
        \/ /\ x = 1 /\ x' = 2
Inv == x < 2
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <Next line 5, col 12 to line 5, col 29 of module Branches>", "State 3: <Next line 6, col 12 to line 6, col 29 of module Branches>"},
		},
		{
			"Quantified", `---- MODULE Quantified ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == \E n \in {1,2} : /\ x = 0 /\ x' = n
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <Next line 5, col 26 to line 5, col 43 of module Quantified>"},
		},
		{
			"IfNext", `---- MODULE IfNext ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == x' = 1
B == x' = 2
Next == IF x = 0 THEN A ELSE B
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <Next line 7, col 9 to line 7, col 30 of module IfNext>"},
		},
		{
			"DynamicQuantified", `---- MODULE DynamicQuantified ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A(n) == x' = n
Next == \E n \in {x+1,x+2} : A(n)
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <Next line 6, col 9 to line 6, col 33 of module DynamicQuantified>"},
		},
		{
			"SpecBranch", `---- MODULE SpecBranch ----
EXTENDS Naturals
VARIABLE x
Spec == /\ x = 0 /\ [][x' = 1 \/ x' = 2]_x
Inv == x < 1
====`, "SPECIFICATION Spec\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <Action line 4, col 24 to line 4, col 29 of module SpecBranch>"},
		},
		{
			"Parameterized", `---- MODULE Parameterized ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A(n) == x' = n
Next == A(x+1)
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <A line 6, col 9 to line 6, col 14 of module Parameterized>"},
		},
		{
			"ConstantAction", `---- MODULE ConstantAction ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A(n) == x' = n
Next == A(1)
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <A line 5, col 9 to line 5, col 14 of module ConstantAction>"},
		},
		{
			"DynamicSplit", `---- MODULE DynamicSplit ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A(n) == \/ x' = n
        \/ x' = n + 1
Next == A(x+1)
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <A line 7, col 9 to line 7, col 14 of module DynamicSplit>"},
		},
		{
			"ConstantSplit", `---- MODULE ConstantSplit ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A(n) == \/ x' = n
        \/ x' = n + 1
Next == A(1)
Inv == x < 1
====`, "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12,
			[]string{"State 1: <Initial predicate>", "State 2: <A line 5, col 12 to line 5, col 17 of module ConstantSplit>"},
		},
		{
			"M", `---- MODULE M ----
EXTENDS TLC
VARIABLE x
Init == x = "a"
Next == \/ /\ x = "a" /\ x' \in {"b", "c"}
        \/ /\ x = "b" /\ x' = "a"
        \/ /\ x = "c" /\ x' = "b"
P == <>[](x = "a")
====`, "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n", 13,
			[]string{"State 1: <Initial predicate>", "State 2: <Next line 5, col 12 to line 5, col 42 of module M>", "State 3: <Next line 7, col 12 to line 7, col 33 of module M>", "Back to state 1: <Next line 6, col 12 to line 6, col 33 of module M>"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{tc.name + ".tla": tc.source, tc.name + ".cfg": tc.cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != tc.exit || errout.Len() != 0 || !reflect.DeepEqual(actionMarkers(out.String()), tc.markers) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, tc.name, tc.exit)
				if got := actionMarkers(java); !reflect.DeepEqual(got, tc.markers) {
					t.Fatalf("Java action markers %q; want %q", got, tc.markers)
				}
			}
		})
	}
}

func actionMarkers(output string) []string {
	var markers []string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "State ") || strings.HasPrefix(line, "Back to state ") {
			markers = append(markers, line)
		}
	}
	return markers
}
