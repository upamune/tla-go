package checker

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// SANY checks temporal operator levels in every definition, even unused ones,
// before TLC interprets the model configuration.
func resolveTemporalExpressions(module *parser.Module) (*parser.Module, error) {
	resolved, err := resolveSubexpressions(module)
	if err != nil {
		return nil, err
	}
	return resolved, validateTemporalLevels(resolved)
}

func validateTemporalLevels(module *parser.Module) error {
	m := &model{module: module, defs: make(map[string]parser.Definition, len(module.Definitions))}
	for _, definition := range module.Definitions {
		m.defs[definition.Name] = definition
	}
	for _, definition := range module.Definitions {
		if err := m.temporalDefinition(definition); err != nil {
			return err
		}
	}
	for _, assumption := range module.Assumptions {
		if err := m.temporalLevelExpression(assumption.Expression); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) temporalDefinition(def parser.Definition) error {
	if !m.temporalDefinitionShadows(def) {
		if err := m.temporalDefinitionBoundsReadOnly(def.Bounds); err != nil {
			return err
		}
		return m.temporalLevelExpression(def.Body)
	}
	local := *m
	local.defs = maps.Clone(m.defs)
	for _, param := range def.Params {
		delete(local.defs, param.Name)
	}
	if err := local.temporalDefinitionBounds(def.Bounds); err != nil {
		return err
	}
	return local.temporalLevelExpression(def.Body)
}

func (m *model) temporalDefinitionShadows(def parser.Definition) bool {
	for _, param := range def.Params {
		if _, ok := m.defs[param.Name]; ok {
			return true
		}
	}
	return temporalBoundsShadow(m.defs, def.Bounds)
}

func (m *model) temporalDefinitionBoundsReadOnly(bounds []parser.Bound) error {
	for _, bound := range bounds {
		if bound.Set != nil {
			if err := m.temporalLevelExpression(bound.Set); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *model) temporalDefinitionBounds(bounds []parser.Bound) error {
	for _, bound := range bounds {
		if bound.Set != nil {
			if err := m.temporalLevelExpression(bound.Set); err != nil {
				return err
			}
		}
		for _, name := range bound.Names {
			delete(m.defs, name)
		}
	}
	return nil
}

func (m *model) temporalLevelExpression(expr *parser.Expression) error {
	if expr == nil {
		return nil
	}
	local := m.temporalScope(expr)
	if err := local.temporalLevelHead(expr); err != nil {
		return &SemanticError{Err: err}
	}
	for _, def := range expr.Definitions {
		if err := local.temporalDefinition(def); err != nil {
			return err
		}
	}
	var failure error
	walkTemporalChildren(expr, func(child *parser.Expression) {
		if failure == nil {
			failure = local.temporalLevelExpression(child)
		}
	})
	return failure
}

func walkTemporalChildren(expr *parser.Expression, visit func(*parser.Expression)) {
	if len(expr.Definitions) == 0 {
		walkChildren(expr, visit)
		return
	}
	children := *expr
	children.Definitions = nil
	walkChildren(&children, visit)
}

func (m *model) temporalScope(expr *parser.Expression) *model {
	if !m.temporalScopeChanges(expr) {
		return m
	}
	local := *m
	local.defs = maps.Clone(m.defs)
	removeTemporalBounds(local.defs, expr.Bounds)
	removeTemporalNames(local.defs, expr)
	for _, def := range expr.Definitions {
		local.defs[def.Name] = def
	}
	return &local
}

func (m *model) temporalScopeChanges(expr *parser.Expression) bool {
	if len(expr.Definitions) > 0 || temporalBoundsShadow(m.defs, expr.Bounds) {
		return true
	}
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		for _, name := range expr.Names {
			if _, ok := m.defs[name]; ok {
				return true
			}
		}
	}
	return false
}

func temporalBoundsShadow(defs map[string]parser.Definition, bounds []parser.Bound) bool {
	for _, bound := range bounds {
		for _, name := range bound.Names {
			if _, ok := defs[name]; ok {
				return true
			}
		}
	}
	return false
}

func removeTemporalBounds(defs map[string]parser.Definition, bounds []parser.Bound) {
	for _, bound := range bounds {
		for _, name := range bound.Names {
			delete(defs, name)
		}
	}
}

func removeTemporalNames(defs map[string]parser.Definition, expr *parser.Expression) {
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		for _, name := range expr.Names {
			delete(defs, name)
		}
	}
}

func (m *model) temporalLevelHead(expr *parser.Expression) error {
	if err := m.temporalValueArguments(expr); err != nil {
		return err
	}
	switch expr.Kind {
	case "Always", "Eventually":
		return m.temporalUnaryLevel(expr)
	case "Apply":
		if err := m.temporalInfixLevel(expr); err != nil {
			return err
		}
		return m.temporalBooleanLevel(expr)
	case "And", "Or":
		return m.temporalBooleanLevel(expr)
	case "Forall", "Exists":
		return m.temporalQuantifierLevel(expr)
	}
	return nil
}

func (m *model) temporalUnaryLevel(expr *parser.Expression) error {
	arg := expr.Args[0]
	if expr.Kind == "Always" && arg.Kind == "BoxAction" || expr.Kind == "Eventually" && arg.Kind == "AngleAction" {
		return nil
	}
	if m.isRawActionLevel(arg) {
		if expr.Kind == "Always" {
			return fmt.Errorf("[] followed by action not of form [A]_v")
		}
		return fmt.Errorf("<> followed by action not of form <<A>>_v")
	}
	return nil
}

func (m *model) temporalInfixLevel(expr *parser.Expression) error {
	if expr.Text != "~>" && expr.Text != "-+->" {
		return nil
	}
	if slices.ContainsFunc(expr.Args, m.isRawActionLevel) {
		return fmt.Errorf("Action used where only temporal formula or state predicate allowed")
	}
	return nil
}

func (m *model) temporalQuantifierLevel(expr *parser.Expression) error {
	if len(expr.Args) == 0 || !m.hasTemporal(expr.Args[0], map[string]bool{}) {
		return nil
	}
	for _, bound := range expr.Bounds {
		if bound.Set != nil && m.isRawActionLevel(bound.Set) {
			return fmt.Errorf("Action-level bound of quantified temporal formula")
		}
	}
	return nil
}

func (m *model) isRawActionLevel(expr *parser.Expression) bool {
	return !m.hasTemporal(expr, map[string]bool{}) && m.hasAction(expr, map[string]bool{})
}
