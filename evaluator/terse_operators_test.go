package evaluator

import (
	"strings"
	"sync"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestTerseOperatorsIndependentPrograms(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals, TLC\nResult == ToString(SUBSET {1, 2})\nFailure == Assert(FALSE, SUBSET {1, 2})\n====")
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for _, tc := range []struct {
		terse bool
		want  string
	}{{false, "{{}, {1}, {2}, {1, 2}}"}, {true, "SUBSET {1, 2}"}} {
		group.Go(func() {
			program := New(module)
			program.SetPrintStyle(PrintStyle{Terse: tc.terse})
			for range 20 {
				result, err := program.Evaluate(module.Definitions[0].Body, Context{})
				if err != nil || !strings.Contains(result.String(), tc.want) {
					t.Errorf("ToString: %v, %v; want %q", result, err, tc.want)
				}
				_, err = program.Evaluate(module.Definitions[1].Body, Context{})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("Assert: %v; want %q", err, tc.want)
				}
			}
		})
	}
	group.Wait()
}
