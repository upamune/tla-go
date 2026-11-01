package checker

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *resolver) expression(expr *parser.Expression, scope map[string]symbol) error {
	if expr == nil {
		return fmt.Errorf("missing expression")
	}
	previous := r.reference
	r.reference = sourceSymbolLocation(expr.Location)
	defer func() { r.reference = previous }()
	if expr.Kind == "AssumeProve" {
		proofResolver := *r
		proofResolver.proofScope = true
		if err := proofResolver.assumeProveExpression(expr, scope); err != nil {
			return &SemanticError{Err: err}
		}
		return nil
	}
	if err := validateRecordFields(expr); err != nil {
		return err
	}
	if err := r.expressionHead(expr, scope); err != nil {
		return err
	}
	local, err := r.expressionScope(expr, scope)
	if err != nil {
		return err
	}
	return r.expressionChildren(expr, local)
}

func (r *resolver) expressionHead(expr *parser.Expression, scope map[string]symbol) error {
	if expr.BuiltinTLC || expr.BuiltinNatural || expr.BuiltinStandard != "" {
		return nil
	}
	if expr.Kind == "ModuleFact" {
		return r.checkProofModule(expr.Text)
	}
	return r.expressionName(expr, scope)
}

func (r *resolver) expressionName(expr *parser.Expression, scope map[string]symbol) error {
	if (expr.Kind == "Identifier" || expr.Kind == "Apply") && r.assumeProve[expr.Text] {
		if _, shadowed := scope[expr.Text]; !shadowed {
			return fmt.Errorf("ASSUME/PROVE used where an expression is required")
		}
	}
	switch expr.Kind {
	case "Identifier":
		return r.checkName(expr.Text, 0, scope)
	case "Apply":
		return r.checkName(expr.Text, len(expr.Args), scope)
	case "OperatorReference", "Subexpression", "Lambda", "ProofStepReference":
		return fmt.Errorf("unsupported expression kind %s at line %d, column %d", expr.Kind, expr.Location.Line, expr.Location.Column)
	}
	return nil
}

func (r *resolver) expressionScope(expr *parser.Expression, scope map[string]symbol) (map[string]symbol, error) {
	if len(expr.Instances) > 0 {
		return nil, fmt.Errorf("LET instances are not yet supported")
	}
	if len(expr.Bounds) == 0 && len(expr.Definitions) == 0 && expr.Kind != "UnboundedQuantifier" {
		return scope, nil
	}
	local := cloneScope(scope)
	if err := r.localDefinitions(expr.Definitions, expr.RecursiveDeclarations, local); err != nil {
		return nil, err
	}
	for _, bound := range expr.Bounds {
		if err := r.bound(bound, local); err != nil {
			return nil, err
		}
	}
	return r.unboundedScope(expr, local)
}

func (r *resolver) unboundedScope(expr *parser.Expression, scope map[string]symbol) (map[string]symbol, error) {
	if expr.Kind == "UnboundedQuantifier" {
		if err := r.bound(parser.Bound{Names: expr.Names}, scope); err != nil {
			return nil, err
		}
	}
	return scope, nil
}

func (r *resolver) localDefinitions(definitions []parser.Definition, recursive []parser.Declaration, scope map[string]symbol) error {
	declared := cloneScope(scope)
	for _, d := range definitions {
		// Instance expansion creates internal names that cannot occur in source.
		if _, ok := declared[d.Name]; !strings.HasPrefix(d.Name, "!") && (ok || r.declaredBefore(d.Name)) {
			return fmt.Errorf("Multiply-defined symbol '%s' at line %d, column %d", d.Name, d.Location.Line, d.Location.Column)
		}
		declared[d.Name] = definitionSymbol(d)
	}
	for _, d := range definitions {
		visible := cloneScope(scope)
		addEarlierLocals(visible, definitions, d)
		addRecursiveLocals(visible, definitions, recursive, d.Location.Position)
		if err := r.definition(d, visible); err != nil {
			return err
		}
	}
	for _, d := range definitions {
		scope[d.Name] = definitionSymbol(d)
	}
	return nil
}

func addEarlierLocals(scope map[string]symbol, definitions []parser.Definition, current parser.Definition) {
	for _, d := range definitions {
		if d.Name != current.Name && localDefinitionVisible(d, current) {
			scope[d.Name] = definitionSymbol(d)
		}
	}
}

func localDefinitionVisible(definition, current parser.Definition) bool {
	a, b := definition.Location.Position, current.Location.Position
	if a.Line == 0 || b.Line == 0 {
		return true
	}
	if a.Line == b.Line && (a.Column == 0 || b.Column == 0) {
		return true
	}
	if a == b {
		return len(definition.Origins) > 0 || strings.HasPrefix(definition.Name, "!")
	}
	return knownPositionAfter(b, a)
}

func addRecursiveLocals(scope map[string]symbol, definitions []parser.Definition, declarations []parser.Declaration, position parser.Position) {
	for _, declaration := range declarations {
		if knownPositionAfter(declaration.Location.Position, position) {
			continue
		}
		for _, definition := range definitions {
			if definition.Name == declaration.Name {
				scope[definition.Name] = definitionSymbol(definition)
				break
			}
		}
	}
}

func (r *resolver) bound(bound parser.Bound, scope map[string]symbol) error {
	if bound.Set != nil {
		if err := r.expression(bound.Set, scope); err != nil {
			return err
		}
	}
	for _, name := range bound.Names {
		if _, ok := scope[name]; ok {
			return fmt.Errorf("duplicate bound name %s", name)
		}
		if r.declaredBefore(name) {
			return fmt.Errorf("bound name %s conflicts with a module symbol", name)
		}
		scope[name] = symbol{}
	}
	return nil
}

func (r *resolver) expressionChildren(expr *parser.Expression, scope map[string]symbol) error {
	if err := r.expressionArguments(expr, scope); err != nil {
		return err
	}
	for _, field := range expr.Fields {
		if err := r.expression(field.Value, scope); err != nil {
			return err
		}
	}
	if err := r.arms(expr.Arms, scope); err != nil {
		return err
	}
	return r.updateChildren(expr.Updates, scope)
}

func (r *resolver) arms(arms []parser.CaseArm, scope map[string]symbol) error {
	for _, arm := range arms {
		if err := r.expression(arm.Condition, scope); err != nil {
			return err
		}
		if err := r.expression(arm.Body, scope); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) updateChildren(updates []parser.Update, scope map[string]symbol) error {
	for _, update := range updates {
		if err := r.updatePath(update.Path, scope); err != nil {
			return err
		}
		if err := r.expression(update.Value, scope); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) updatePath(path []parser.PathStep, scope map[string]symbol) error {
	for _, step := range path {
		for _, arg := range step.Args {
			if err := r.expression(arg, scope); err != nil {
				return err
			}
		}
	}
	return nil
}
