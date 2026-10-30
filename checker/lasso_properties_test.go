package checker

import (
	"reflect"
	"testing"
)

// Expected property lists were produced by the pinned tla-ts checkSequentially.
func TestLassoReportsOnlyPropertiesViolatedOnItsCycle(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 3
Spec == Init /\ [][Next]_x /\ WF_x(Next)
Reach == <>(x = 5)
Visit == []<>(x = 2)
Settle == <>[](x = 0)
Response == (x = 1) ~> (x = 9)
Progress == []<><<x' = x + 1>>_x
Missing == []<><<x' = 9>>_x
Every == \A i \in {0,1} : <>[](x = i)
====`
	for _, tc := range []struct {
		properties string
		want       []string
	}{
		{"Reach Visit Settle Response Progress Missing Every", []string{"Reach", "Settle", "Response", "Missing", "Every"}},
		{"Settle Reach Visit", []string{"Settle", "Reach"}},
		{"Visit Missing", []string{"Missing"}},
		{"Visit Every", []string{"Every"}},
	} {
		t.Run(tc.properties, func(t *testing.T) {
			result, err := Check(source, "SPECIFICATION Spec PROPERTIES "+tc.properties)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != LivenessViolation || result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, tc.want) || result.Violation.Name != tc.want[0] {
				t.Fatalf("got %+v violation=%+v", result, result.Violation)
			}
		})
	}
}

func TestLassoPropertiesAccountForPrefix(t *testing.T) {
	source := `---- MODULE M ----
VARIABLE x
Init == x = 0
Next == x' = IF x = 0 THEN 5 ELSE 1
Spec == Init /\ [][Next]_x /\ WF_x(Next)
Missing == <>(x = 9)
Once == <>(x = 5)
Often == []<>(x = 5)
Response == (x = 5) ~> (x = 1)
Unmet == (x = 5) ~> (x = 9)
====`
	result, err := Check(source, "SPECIFICATION Spec PROPERTIES Missing Once Often Response Unmet")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Missing", "Often", "Unmet"}
	if result.Status != LivenessViolation || result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, want) {
		t.Fatalf("got %+v violation=%+v", result, result.Violation)
	}
}
