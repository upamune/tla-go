package checker

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// evaluationWarningHandler forwards warnings raised while evaluating
// expressions, such as warning 2144 for an EXCEPT outside a function's domain.
func (m *model) evaluationWarningHandler() func(int, string) {
	if m.warning == nil {
		return nil
	}
	return func(code int, message string) { m.warning(Warning{Code: code, Message: message}) }
}

// warnSmallSymmetrySets reports Java TLC's warning 2300 for symmetry sets
// with fewer than two elements. As in Tool.getSymmetryPerms, each set is
// named by the source text of its argument, without a Permutations call,
// or by the constant a configuration override replaces with it.
func (m *model) warnSmallSymmetrySets(sets []*parser.Expression) {
	if m.warning == nil || len(sets) == 0 {
		return
	}
	names := make([]string, len(sets))
	for i, set := range sets {
		names[i] = m.symmetrySetName(set)
	}
	plural, verb, suffix := "", "has", "s"
	if len(sets) > 1 {
		plural, verb, suffix = "s", "have", ""
	}
	message := fmt.Sprintf("The set%s %s %s been defined to be a symmetry set but contain%s less than two elements.", plural, strings.Join(names, ", and "), verb, suffix)
	m.warning(Warning{Code: 2300, Message: message})
}

// unpermutedSymmetrySets returns the Permutations arguments of the symmetry
// definition that name an enumerated set none of whose model values is moved
// by the symmetry group.
func (m *model) unpermutedSymmetrySets(body *parser.Expression) []*parser.Expression {
	if body.Kind != "Apply" {
		return nil
	}
	moved := map[string]bool{}
	for _, p := range m.symmetry {
		for name := range p.images {
			moved[name] = true
		}
	}
	var unpermuted []*parser.Expression
	for _, arg := range body.Args {
		if elements, ok := m.permutedSetElements(arg); ok && !movesAny(elements, moved) {
			unpermuted = append(unpermuted, arg)
		}
	}
	return unpermuted
}

func (m *model) permutedSetElements(arg *parser.Expression) ([]value.Value, bool) {
	if arg.Kind != "Apply" || arg.Text != "Permutations" || len(arg.Args) != 1 || arg.Args[0].Kind != "Identifier" {
		return nil, false
	}
	set, err := m.eval(arg.Args[0], nil, nil)
	if err != nil || set.Kind() != value.SetKind {
		return nil, false
	}
	elements, err := set.Elements()
	return elements, err == nil
}

func movesAny(elements []value.Value, moved map[string]bool) bool {
	for _, element := range elements {
		name, ok := element.Text()
		if ok && element.Kind() == value.ModelKind && moved[name] {
			return true
		}
	}
	return false
}

func (m *model) symmetrySetName(set *parser.Expression) string {
	name := m.sourceImage(set)
	if inner, ok := strings.CutPrefix(name, "Permutations("); ok {
		name = strings.TrimSuffix(inner, ")")
	}
	overridden := ""
	for _, substitution := range m.sourceConfig.Substitutions {
		if substitution.Module == "" && substitution.Target == name {
			overridden = substitution.Name
		}
	}
	if overridden != "" {
		return overridden
	}
	return name
}

// sourceImage concatenates the source tokens of an expression without
// whitespace or comments, like SANY's human-readable image.
func (m *model) sourceImage(expr *parser.Expression) string {
	r := expr.Location.Range
	unit := m.module
	if source, ok := m.sourceUnits[r.Module]; ok {
		unit = source
	}
	var image strings.Builder
	for _, token := range unit.SourceTokens {
		start := parser.Position{Line: token.Line, Column: token.Column}
		if positionBefore(r.Start, start) && positionBefore(start, r.End) {
			image.WriteString(token.Text)
		}
	}
	if image.Len() == 0 {
		return expr.Text
	}
	return image.String()
}
