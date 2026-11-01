package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJavaTLCQuantifiedSafetyProperties(t *testing.T) {
	cases := []struct {
		name, property string
		code, message  int
	}{
		{"quantified state", `\A v \in {2} : [](x # v)`, 13, 2116},
		{"multiple values", `\A v \in {1, 2} : [](x # v)`, 13, 2116},
		{"nested quantifiers", `\A v \in {1, 2} : \A w \in {v} : [](x # w)`, 13, 2116},
		{"parameterized state", `Q(2)`, 13, 2116},
		{"quantified action", `\A v \in {2} : [][x' # v]_x`, 255, 2214},
		{"parameterized action", `R(2)`, 255, 2214},
		{"tla-ts quantified conjunction", `\A i \in {2, 3} : [](x < i) /\ [][x' < i]_x`, 255, 2214},
		{"tla-ts quantified action", `\A n \in {0, 1} : [][x' # n]_x`, 255, 2214},
		{"empty quantified state", `\A v \in {} : [](x # v)`, 77, 2253},
		{"empty quantified action", `\A v \in {} : [][x' # v]_x`, 77, 2253},
		{"empty inner quantifier", `\A v \in {1, 2} : \A w \in {} : [](x # w)`, 77, 2253},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nQ(v) == [](x # v)\nR(v) == [][x' # v]_x\nP == " + tc.property
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if code != tc.code || stderr != "" || !hasToolMessage(messages, tc.message) {
				t.Fatalf("exit %d, messages %+v, stderr %s; want exit %d and message %d", code, messages, stderr, tc.code, tc.message)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"})
				if javaCode != tc.code || !hasToolMessage(readToolMessages(java), tc.message) {
					t.Fatalf("Java exit %d, output %s", javaCode, java)
				}
				if got, want := toolConsumerMessages(messages), toolConsumerMessages(readToolMessages(java)); !reflect.DeepEqual(got, want) {
					t.Fatalf("Java messages %+v, Go messages %+v", want, got)
				}
			}
		})
	}
}

func TestJavaTLCEmptyQuantifiedSafetyWithOtherProperty(t *testing.T) {
	for _, tc := range []struct {
		name, property string
		code, message  int
	}{
		{"temporal", "<>(x = 1)", 13, 2116},
		{"safety", "[](x < 5)", 77, 2253},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == \\A v \\in {} : [](x # v)\nQ == " + tc.property
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P Q\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if code != tc.code || stderr != "" || !hasToolMessage(messages, tc.message) {
				t.Fatalf("exit %d, messages %+v, stderr %s", code, messages, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

// Java 1.8.0 splits quantified safety properties differently from 1.7.4.
func TestJavaTLCModernQuantifiedSafetyVersionDifference(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("TLC_MODERN_JAR is not set")
	}
	for _, tc := range []struct {
		name, property         string
		goCode, javaCode       int
		goMessage, javaMessage int
	}{
		{"state", `\A v \in {2} : [](x # v)`, 13, 12, 2116, 2110},
		{"action", `\A v \in {2} : [][x' # v]_x`, 255, 13, 2214, 2112},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == " + tc.property
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
			got, code, stderr := runGoToolInDir(t, dir, args)
			if code != tc.goCode || stderr != "" || !hasToolMessage(got, tc.goMessage) {
				t.Fatalf("Go exit %d, messages %+v, stderr %s", code, got, stderr)
			}
			java, javaCode := runJavaTLCArgs(t, jar, dir, args)
			if javaCode != tc.javaCode || !hasToolMessage(readToolMessages(java), tc.javaMessage) {
				t.Fatalf("Java exit %d, output %s", javaCode, java)
			}
		})
	}
}
