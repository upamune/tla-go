package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJavaTLCSimulationLiveness(t *testing.T) {
	cases := []struct {
		name, body, cfg string
		code            int
	}{
		{"violated at initial", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"satisfied at initial", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"never satisfied", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=2)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"constrained trace", "Init == x=0\nNext == x'=1\nBound == x=0\nP == <>(x=1)", "INIT Init\nNEXT Next\nCONSTRAINT Bound\nPROPERTY P\n", 13},
		{"deadlock without checking", "Init == x=0\nNext == /\\ x=0 /\\ x'=1\nP == <>(x=2)", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\nPROPERTY P\n", 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
			code := run(context.Background(), args, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				got := simulationMessages(readToolMessages(out.String()))
				want := simulationMessages(javaSimulationTool(t, jar, dir, code))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}
