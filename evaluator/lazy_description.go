package evaluator

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func lazyDescription(domain value.Value, bounds []parser.Bound, body *parser.Expression) string {
	if domain.IsEnumerable() || len(bounds) == 0 || body == nil {
		return ""
	}
	factors := []value.Value{domain}
	if len(bounds) > 1 {
		var err error
		factors, err = domain.ProductFactors()
		if err != nil || len(factors) != len(bounds) {
			return ""
		}
	}
	parts, ok := lazyBoundDescriptions(bounds, factors)
	if !ok {
		return ""
	}
	r := body.Location.Range
	return fmt.Sprintf("[%s |-> <expression line %d, col %d to line %d, col %d of module %s>]", strings.Join(parts, ", "), r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
}

func lazyBoundDescriptions(bounds []parser.Bound, factors []value.Value) ([]string, bool) {
	parts := make([]string, 0, len(bounds))
	for i, bound := range bounds {
		if i > 0 && bound.Set == bounds[i-1].Set {
			continue
		}
		set, err := factors[i].SymbolicString()
		if err != nil {
			return nil, false
		}
		name := strings.Join(bound.Names, ", ")
		if bound.Tuple {
			name = "<<" + name + ">>"
		}
		parts = append(parts, name+" \\in "+set)
	}
	return parts, true
}
