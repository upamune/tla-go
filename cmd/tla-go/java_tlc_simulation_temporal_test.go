package main

import (
	"fmt"
	"os"
	"testing"
)

func TestJavaTLCSimulationFairness(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, fairness := range []string{"WF_x(Next)", "SF_x(Next)", "WF_x(Up)", "SF_x(Up) /\\ WF_x(Down)", "[]<><<Up>>_x", "\\A d \\in {1, 2} : WF_x(x' = d)"} {
		for _, property := range []string{"<>(x=2)", "[]<>(x=0)", "<>[](x=2)", "(x=1) ~> (x=0)", "[](x=1 => <>(x=2))", "<>(x=1) /\\ []<>(x=2)"} {
			for _, seed := range []string{"1", "2"} {
				t.Run(fmt.Sprintf("%s/%s/%s", fairness, property, seed), func(t *testing.T) {
					body := "VARIABLE x\nUp == x < 2 /\\ x' = x + 1\nDown == x > 0 /\\ x' = x - 1\nInit == x = 0\nNext == Up \\/ Down\nSpec == Init /\\ [][Next]_x /\\ " + fairness + "\nP == " + property
					compareSimulationGraphParameters(t, jar, body, "SPECIFICATION Spec PROPERTY P", seed, "8", "3")
				})
			}
		}
	}
}

// These forms cover the property classes of model checking, which simulation
// converts to TLC's normal form, and negations above temporal operators.
func TestJavaTLCSimulationTemporalForms(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, property := range []string{
		"[](x = 1 => [](x = 1))", "<>(x = 1 /\\ <>(x = 2))", "[]<><<x' = 0>>_x", "<>[][x' = x]_x",
		"[]<>(x = 1) => []<>(x = 2)", "(x = 1) ~> [](x = 2)", "<>[](x = 1) \\/ []<>(x = 2)", "\\A v \\in {1, 2} : <>(x = v)",
		"IF x = 0 THEN <>(x = 2) ELSE [](x = 1)", "~[](x = 0)", "<>~[](x = 1)", "[](<>(x = 1) \\/ <>(x = 2))",
		// TLC does not split a property below a quantifier, so this is not an invariant.
		"\\A v \\in {2} : [](x # v)",
	} {
		for _, next := range []string{"x' \\in {0, 1, 2}", "IF x = 2 THEN x' = 2 ELSE x' = x + 1"} {
			t.Run(property+"/"+next, func(t *testing.T) {
				body := "VARIABLE x\nInit == x = 0\nNext == " + next + "\nP == " + property
				compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next PROPERTY P", "1", "6", "3")
			})
		}
	}
}

// With VIEW, TLC fingerprints merge the behavior graph's states and select
// the recovered states of the counterexample.
func TestJavaTLCSimulationLivenessView(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, property := range []string{"<>(x = 3)", "[]<>(y = 0)", "(x = 1) ~> (y = 2)"} {
		for _, seed := range []string{"1", "2"} {
			t.Run(property+"/"+seed, func(t *testing.T) {
				body := "VARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x' = (x + 1) % 3 /\\ y' \\in {y, (y + 1) % 4}\nV == x\nP == " + property
				compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next VIEW V PROPERTY P", seed, "8", "2")
			})
		}
	}
}

// TLC recovers each printed step from the first action that generates it, so
// the label can differ from the action that the simulation sampled.
func TestJavaTLCSimulationLivenessActionRecovery(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, seed := range []string{"1", "2", "3", "4"} {
		t.Run(seed, func(t *testing.T) {
			body := "VARIABLE x\nA == x' = (x + 1) % 3\nB == x' \\in {0, 1, 2}\nInit == x = 0\nNext == B \\/ A\nP == <>[](x = 2) \\/ [](x = 0)"
			compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next PROPERTY P", seed, "6", "2")
		})
	}
}

// TLC recovers each counterexample state with its predecessor as the current
// state, so TLCGet("level") in Next is the predecessor's position in the
// counterexample. A step of the trace can then be impossible to recover.
func TestJavaTLCSimulationLivenessRecoveryLevel(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for i, next := range []string{
		"x' = IF TLCGet(\"level\") < 4 THEN (x + 1) % 3 ELSE x",
		"\\/ TLCGet(\"level\") < 3 /\\ x' = (x + 1) % 3\n        \\/ TLCGet(\"level\") >= 3 /\\ x' = (x + 2) % 3",
	} {
		for _, property := range []string{"[]<>(x = 0)", "<>[](x = 0)"} {
			for _, seed := range []string{"1", "2", "3"} {
				t.Run(fmt.Sprintf("Next%d/%s/%s", i+1, property, seed), func(t *testing.T) {
					body := "VARIABLE x\nInit == x = 0\nNext == " + next + "\nSpec == Init /\\ [][Next]_x\nP == " + property
					compareSimulationGraphParameters(t, jar, body, "SPECIFICATION Spec PROPERTY P", seed, "8", "3")
				})
			}
		}
	}
	// TLC generates the initial states again with the last sampled state as
	// the current state: the successor beyond a full trace, or the state that
	// violates the constraint.
	for _, level := range []string{"3", "8", "9"} {
		t.Run("Init/"+level, func(t *testing.T) {
			body := "VARIABLE x\nInit == x = 0 /\\ TLCGet(\"level\") # " + level + "\nNext == x' = (x + 1) % 3\nC == x < 2\nP == []<>(x = 5)"
			compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next PROPERTY P", "1", "8", "1")
			compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next CONSTRAINT C PROPERTY P", "1", "8", "1")
		})
	}
}
