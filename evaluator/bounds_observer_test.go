package evaluator

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestBoundVisitObserver(t *testing.T) {
	for _, tc := range []struct {
		name, expr string
		want       []bool
	}{
		{"empty", `\A x \in 1..0 : TRUE`, []bool{true}},
		{"all", `\A x \in 1..3 : TRUE`, []bool{true, false, false, false}},
		{"short circuit", `\E x \in 1..3 : TRUE`, []bool{true, false}},
		{"error", `\A x \in 1..3 : 1 \div 0 = x`, []bool{true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nA == " + tc.expr + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			domain := m.Definitions[0].Body.Bounds[0].Set
			var got []bool
			_, err = Evaluate(m, m.Definitions[0].Body, Context{OnBoundVisit: func(expr *parser.Expression, initial bool) {
				if expr != domain {
					t.Errorf("unexpected domain %v", expr)
				}
				got = append(got, initial)
			}})
			if (err != nil) != (tc.name == "error") {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("visits %v, want %v", got, tc.want)
			}
		})
	}
}
