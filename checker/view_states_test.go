package checker

import (
	"context"
	"reflect"
	"testing"
)

func TestViewStatesFormatTrace(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x' = x + 1 /\\ y' = y\nInv == x < 1\nV == <<x, y>>\n===="
	for _, tc := range []struct {
		viewStates bool
		want       []string
	}{
		{false, []string{"", ""}},
		{true, []string{"/\\ x = 0\n/\\ y = 0\n", "/\\ x = 1\n/\\ y = 0\n"}},
	} {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv VIEW V", Options{ViewStates: tc.viewStates})
		if err != nil || result.Violation == nil {
			t.Fatalf("result %+v, error %v", result, err)
		}
		var views []string
		for _, step := range result.Violation.Trace {
			views = append(views, step.View)
		}
		if !reflect.DeepEqual(views, tc.want) {
			t.Fatalf("ViewStates %v: views %q, want %q", tc.viewStates, views, tc.want)
		}
	}
}
