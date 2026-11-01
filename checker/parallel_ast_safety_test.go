package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestParallelExpressionShapes(t *testing.T) {
	m := &model{variables: map[string]bool{"x": true}}
	number := &parser.Expression{Kind: "Number", Text: "1"}
	for _, tc := range []struct {
		name string
		expr *parser.Expression
		want bool
	}{
		{"number", number, true},
		{"number with argument", &parser.Expression{Kind: "Number", Text: "1", Args: []*parser.Expression{number}}, false},
		{"identifier with argument", &parser.Expression{Kind: "Identifier", Text: "x", Args: []*parser.Expression{number}}, false},
		{"empty conjunction", &parser.Expression{Kind: "And"}, false},
		{"empty disjunction", &parser.Expression{Kind: "Or"}, false},
		{"prime", &parser.Expression{Kind: "Prime", Args: []*parser.Expression{{Kind: "Identifier", Text: "x"}}}, true},
		{"missing prime operand", &parser.Expression{Kind: "Prime"}, false},
		{"missing unchanged operand", &parser.Expression{Kind: "Unchanged"}, false},
		{"extra unchanged operand", &parser.Expression{Kind: "Unchanged", Args: []*parser.Expression{number, number}}, false},
		{"unary negation", &parser.Expression{Kind: "Apply", Text: "~", Args: []*parser.Expression{{Kind: "Identifier", Text: "TRUE"}}}, true},
		{"binary addition", &parser.Expression{Kind: "Apply", Text: "+", Args: []*parser.Expression{number, number}}, true},
		{"addition missing operand", &parser.Expression{Kind: "Apply", Text: "+", Args: []*parser.Expression{number}}, false},
		{"implication missing operand", &parser.Expression{Kind: "Apply", Text: "=>", Args: []*parser.Expression{number}}, false},
		{"marked standard operator", &parser.Expression{Kind: "Apply", Text: "+", BuiltinStandard: "Randomization", Args: []*parser.Expression{number, number}}, false},
		{"marked natural operator", &parser.Expression{Kind: "Apply", Text: "+", BuiltinNatural: true, Args: []*parser.Expression{number, number}}, false},
		{"valid marked natural operator", &parser.Expression{Kind: "Apply", Text: "!naturals!+", BuiltinNatural: true, Args: []*parser.Expression{number, number}}, false},
		{"marked tlc operator", &parser.Expression{Kind: "Apply", Text: "+", BuiltinTLC: true, Args: []*parser.Expression{number, number}}, false},
		{"nil operand", &parser.Expression{Kind: "Apply", Text: "+", Args: []*parser.Expression{number, nil}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.parallelExpression(tc.expr); got != tc.want {
				t.Fatalf("eligible %t, want %t", got, tc.want)
			}
		})
	}
}

// Invalid direct-API ASTs must not turn a caller-recoverable panic into an
// unrecoverable worker-goroutine panic. Each check runs in a child process.
func TestParallelMalformedASTSubprocess(t *testing.T) {
	if variant := os.Getenv("TLA_GO_PARALLEL_AST_VARIANT"); variant != "" {
		parallelMalformedASTChild(t, variant)
		return
	}
	for _, variant := range []string{"marked", "implication", "prime", "unchanged", "extra-addition"} {
		t.Run(variant, func(t *testing.T) {
			var output [2]string
			for i, workers := range []string{"1", "4"} {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestParallelMalformedASTSubprocess$")
				cmd.Env = append(os.Environ(), "TLA_GO_PARALLEL_AST_VARIANT="+variant, "TLA_GO_PARALLEL_AST_WORKERS="+workers)
				data, err := cmd.CombinedOutput()
				cancel()
				if err != nil {
					t.Fatalf("workers=%s child failed: %v\n%s", workers, err, data)
				}
				output[i] = string(data)
			}
			if output[0] != output[1] {
				t.Fatalf("serial output %q, parallel output %q", output[0], output[1])
			}
			if variant == "marked" && !strings.Contains(output[0], "caller recovered: runtime error: index out of range") {
				t.Fatalf("did not exercise the caller-recoverable panic: %q", output[0])
			}
		})
	}
}

func parallelMalformedASTChild(t *testing.T, variant string) {
	t.Helper()
	runtime.GOMAXPROCS(4)
	workers, err := strconv.Atoi(os.Getenv("TLA_GO_PARALLEL_AST_WORKERS"))
	if err != nil {
		t.Fatal(err)
	}
	module, err := parser.ParseModule(parallelConstantModel())
	if err != nil {
		t.Fatal(err)
	}
	replaced := false
	for _, definition := range module.Definitions {
		if definition.Name == "Next" && replaceParallelNumber(definition.Body, variant) {
			replaced = true
			break
		}
	}
	if !replaced {
		t.Fatal("did not replace the intended expression")
	}
	cfg, err := config.Parse(parallelConstantConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if p := recover(); p != nil {
			fmt.Printf("caller recovered: %v\n", p)
		}
	}()
	result, err := CheckModuleWithOptions(context.Background(), module, cfg, Options{Workers: workers})
	fmt.Printf("status=%s error=%v\n", result.Status, err)
}

func replaceParallelNumber(expr *parser.Expression, variant string) bool {
	if expr.Kind == "Number" && expr.Text == "13" {
		switch variant {
		case "marked":
			*expr = parser.Expression{Kind: "Apply", Text: "+", BuiltinStandard: "Randomization"}
		case "implication":
			*expr = parser.Expression{Kind: "Apply", Text: "=>"}
		case "prime":
			*expr = parser.Expression{Kind: "Prime"}
		case "unchanged":
			*expr = parser.Expression{Kind: "Unchanged"}
		case "extra-addition":
			number := &parser.Expression{Kind: "Number", Text: "1"}
			*expr = parser.Expression{Kind: "Apply", Text: "+", Args: []*parser.Expression{number, number, number}}
		}
		return true
	}
	for _, child := range expr.Args {
		if replaceParallelNumber(child, variant) {
			return true
		}
	}
	return false
}
