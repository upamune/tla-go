package checker

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TLC 2.19 (tla2tools 1.7.4) explores both models. The pinned tla-ts
// returns an unassigned-variable error while evaluating the function action.
func TestParameterizedInstanceFunctionActions(t *testing.T) {
	dir := filepath.Join("testdata", "tlc-parameterized-function-action")
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	source, module := read("M.tla"), read("A.tla")
	loader := func(context.Context, string) (string, error) { return module, nil }
	for _, tt := range []struct {
		name, config string
		want         expectedCheck
	}{
		{"success", read("Success.cfg"), expectedCheck{Status: "success", Generated: 7, Distinct: 5, Depth: 3}},
		{"violation", read("Violation.cfg"), expectedCheck{Status: "invariant-violation", Name: "Inv", Generated: 5, Distinct: 4, Depth: 3, Trace: []expectedTrace{{"", map[string]string{"x": "0"}}, {"Next", map[string]string{"x": "1"}}, {"Next", map[string]string{"x": "3"}}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckWithLoader(context.Background(), source, tt.config, loader)
			if err != nil {
				t.Fatal(err)
			}
			if got := project(result); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
