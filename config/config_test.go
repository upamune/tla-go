package config

import (
	"reflect"
	"strings"
	"testing"
)

// These cases port the parseConfig tests in tla-ts tests/unit/check.test.ts.
func TestUpstreamConfigCases(t *testing.T) {
	c, err := Parse("CONSTANT ++ <- PlusPlus\n  Plus <- +\n  \\o <- +\nINVARIANT 1Inv")
	if err != nil {
		t.Fatal(err)
	}
	want := []Substitution{{Name: "++", Target: "PlusPlus"}, {Name: "Plus", Target: "+"}, {Name: "\\o", Target: "+"}}
	if !reflect.DeepEqual(c.Substitutions, want) || !reflect.DeepEqual(c.Invariants, []string{"1Inv"}) {
		t.Fatalf("got %#v", c)
	}
	c, err = Parse("SPECIFICATION Spec\n_POSSIBLE Done Started\nPOSTCONDITION Post")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Possible, []string{"Done", "Started"}) || !reflect.DeepEqual(c.Postconditions, []string{"Post"}) {
		t.Fatalf("got %#v", c)
	}
	c, err = Parse("INIT Init\n_PERIODIC Periodic\n_RL_REWARD Reward")
	if err != nil || c.Periodic != "Periodic" || c.RLReward != "Reward" {
		t.Fatalf("got %#v, %v", c, err)
	}
	for _, k := range []string{"CHECK_DEADLOCK", "_PERIODIC", "_RL_REWARD"} {
		_, err = Parse(k + " FALSE\n" + k + " TRUE")
		if err == nil || !strings.Contains(err.Error(), "The keyword "+k+" appears twice") {
			t.Errorf("%s: %v", k, err)
		}
	}
	c, err = Parse(`CONSTANT S = "a\"b\\n"`)
	if err != nil || c.Constants[0].Value.Text != `a\"b\\n` {
		t.Fatalf("got %#v, %v", c, err)
	}
	for _, s := range []string{"CONSTANT N = -1", "CONSTANT S = {1,}"} {
		if _, err = Parse(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}

func TestAssignmentsAndErrors(t *testing.T) {
	c, err := Parse("\\* comment\nCONSTANTS F(1, {a, TRUE}) = [M] 2\nG <- [M] H\nCHECK_DEADLOCK FALSE\nINVARIANTS A B")
	if err != nil {
		t.Fatal(err)
	}
	if c.CheckDeadlock || len(c.Constants) != 1 || c.Constants[0].Module != "M" || len(c.Constants[0].Args) != 2 || len(c.Substitutions) != 1 || c.Substitutions[0].Module != "M" {
		t.Fatalf("got %#v", c)
	}
	for _, s := range []string{"INIT A INIT B", "CHECK_DEADLOCK X", "CONSTANT X = {1,"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if _, err := Parse("(* bad"); err != nil {
		t.Fatalf("TLC treats an unfinished comment as end of input: %v", err)
	}
	if mixed, err := Parse("SPECIFICATION Spec INIT Init"); err != nil || mixed.Specification != "Spec" || mixed.Init != "Init" {
		t.Fatalf("mixed config: %#v, %v", mixed, err)
	}
}

func BenchmarkParse(b *testing.B) {
	const source = "INIT Init NEXT Next INVARIANTS Safety TypeOK CONSTANTS N = 100 S = {a,b,c} CHECK_DEADLOCK FALSE"
	for i := 0; i < b.N; i++ {
		if _, err := Parse(source); err != nil {
			b.Fatal(err)
		}
	}
}

func TestConfigOperatorAndNumberLexing(t *testing.T) {
	c, err := Parse("CONSTANT ∈ <- Plus\n Huge = 123456789012345678901234567890\nINVARIANT 1Inv")
	if err != nil {
		t.Fatal(err)
	}
	if c.Substitutions[0].Name != "\\in" || c.Constants[0].Value.Kind != "Number" || c.Invariants[0] != "1Inv" {
		t.Fatalf("got %#v", c)
	}
}

func TestDigitPrefixedIdentifiers(t *testing.T) {
	c, err := Parse("INVARIANTS 1Inv 12Inv 123Inv")
	if err != nil || !reflect.DeepEqual(c.Invariants, []string{"1Inv", "12Inv", "123Inv"}) {
		t.Fatalf("got %#v, %v", c, err)
	}
}
