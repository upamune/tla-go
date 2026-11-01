package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRecursiveLetProjection(t *testing.T) {
	m, err := ParseModule("---- MODULE M ----\nX == LET RECURSIVE F(_) F(x) == x IN F(1)\n====")
	if err != nil {
		t.Fatal(err)
	}
	let := m.Definitions[0].Body
	if len(let.RecursiveDeclarations) != 1 || let.RecursiveDeclarations[0].Name != "F" || let.RecursiveDeclarations[0].Arity != 1 {
		t.Fatalf("LET declarations: %+v", let.RecursiveDeclarations)
	}
	data, err := json.Marshal(let)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "RecursiveDeclarations") {
		t.Fatal("LET projection changed")
	}
	var projected Expression
	if err := json.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if len(projected.RecursiveDeclarations) != 0 || len(projected.Definitions) != 1 || !projected.Definitions[0].Recursive {
		t.Fatalf("unexpected projection: %+v", projected)
	}
}
