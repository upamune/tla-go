package checker

import "testing"

// Statuses and state counts come from the pinned tla-ts implementation.
func TestSpecificationRecurrenceAndQuantifiedAliases(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSome(i) == []<>(x = i)\nSpec == Init /\\ [][Next]_x /\\ []<>(x = 1) /\\ []<><<Next>>_x\nQuantified == Init /\\ [][Next]_x /\\ \\A i \\in {0,1,2}: Some(i)\nReach == <>(x = 2)\nAbsent == <>(x = 9)\nOften == []<>(x = 1)\n===="
	for _, spec := range []string{"Spec", "Quantified"} {
		for _, property := range []string{"Reach", "Absent", "Often"} {
			t.Run(spec+"/"+property, func(t *testing.T) {
				configuration := "SPECIFICATION " + spec + " PROPERTY " + property
				result, err := Check(source, configuration)
				if err != nil {
					t.Fatal(err)
				}
				want := Success
				if property == "Absent" {
					want = LivenessViolation
				}
				if result.Status != want || result.GeneratedStates != 4 || result.DistinctStates != 3 {
					t.Fatalf("got %+v", result)
				}
				if result.Status == LivenessViolation {
					checkLasso(t, source, configuration, result.Violation)
				}
			})
		}
	}
}

func TestSpecificationRecurrenceRejectsUnsupportedLevels(t *testing.T) {
	for _, assumption := range []string{
		"[]<>(x' = 1)",
		"[]<>(<>(x = 1))",
		"\\A i \\in {x}: []<>(x = i)",
	} {
		t.Run(assumption, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x /\\ " + assumption + "\nP == <>(x = 0)\n===="
			if result, err := Check(source, "SPECIFICATION Spec PROPERTY P"); err == nil {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
