package main

import "testing"

func TestJavaTLCSymmetryTableauRegeneration(t *testing.T) {
	for _, name := range []string{"ChooseTableauSymmetryMC", "ChooseTableauSymmetryMCa"} {
		t.Run(name, func(t *testing.T) {
			dir := copySymmetryRecurrenceFixtures(t)
			checkSymmetryRecurrence(t, dir, name, 13, "13 states generated, 6 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryLeadsToFairness(t *testing.T) {
	for _, tc := range []struct {
		name, variables, fairness, property string
		constraints                         bool
		code                                int
	}{
		{"no fairness", "x,bit", "", "(x=a) ~> (bit=1)", false, 13},
		{"weak fairness", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"strong fairness", "x,bit", " /\\ SF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"other variable order", "bit,x", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"state and action constraints", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", true, 0},
		{"other trigger without fairness", "x,bit", "", "(bit=1) ~> (x=a)", false, 13},
		{"other trigger with fairness", "x,bit", " /\\ WF_vars(Next)", "(bit=1) ~> (x=a)", false, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES " + tc.variables + "\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ bit' \\in {0,1}\n" +
				"Spec == Init /\\ [][Next]_vars" + tc.fairness + "\nSym == Permutations(P)\nProp == " + tc.property
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			if tc.constraints {
				body += "\nCon == bit=0 \\/ x=a\nAct == bit'=bit \\/ x'=x"
				cfg += " CONSTRAINT Con ACTION_CONSTRAINT Act"
			}
			writeDOTFixture(t, dir, body, cfg)
			checkSymmetryRecurrence(t, dir, "M", tc.code, "10 states generated, 2 distinct states found, 0 states left on queue")
		})
	}
}
