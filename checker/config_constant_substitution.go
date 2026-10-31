package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func applyValidatedConfigOverrides(module *parser.Module, cfg config.Config) (*parser.Module, config.Config, error) {
	module, cfg, err := applyConfigOverrides(module, cfg)
	if err != nil {
		return nil, cfg, err
	}
	if err = validateConstantAssignments(module, cfg); err != nil {
		if semantic := validateResolved(module); semantic != nil {
			return nil, cfg, semantic
		}
		return nil, cfg, err
	}
	return module, cfg, nil
}

func validateConstantAssignments(module *parser.Module, cfg config.Config) error {
	if len(module.Constants) == 0 {
		return nil
	}
	assigned := make(map[string]bool, len(cfg.Constants))
	for _, assignment := range cfg.Constants {
		assigned[assignment.Name] = true
	}
	for _, declaration := range module.Constants {
		if !assigned[declaration.Name] {
			return &ConfigurationError{
				Message: fmt.Sprintf("The constant parameter %s is not assigned a value by the configuration file.", declaration.Name),
				Code:    2222,
				Err:     fmt.Errorf("constant %s is not assigned", declaration.Name),
			}
		}
	}
	return nil
}

func (m *model) configSubstitutionCacheExclusions() map[string]bool {
	var excluded map[string]bool
	for _, definition := range m.module.Definitions {
		if definition.ConfigSubstitutionTarget != "" {
			if m.staticConfigSubstitutionTarget(definition.ConfigSubstitutionTarget) {
				continue
			}
			if excluded == nil {
				excluded = make(map[string]bool)
			}
			excluded[definition.Name] = true
			excluded[definition.ConfigSubstitutionTarget] = true
		}
	}
	return excluded
}

func (m *model) staticConfigSubstitutionTarget(name string) bool {
	definition, ok := m.defs[name]
	return ok && m.staticConstantExpression(definition.Body, nil)
}

func (m *model) validateConstantSubstitutions() error {
	visited := false
	for _, definition := range m.module.Definitions {
		if definition.ConfigSubstitutionTarget == "" {
			continue
		}
		visited = true
		context := evaluator.Context{Constants: m.constants, Enabled: m.enabledCallback, UncachedConstants: true}
		if _, err := m.program.Evaluate(definition.Body, context); err != nil {
			return constantSubstitutionError(definition, err)
		}
	}
	if visited {
		m.precomputeEffectfulDefinitions()
	}
	return nil
}

func (m *model) precomputeEffectfulDefinitions() {
	excluded := m.configSubstitutionCacheExclusions()
	// Java TLC precomputes eligible definitions in module declaration order.
	for _, definition := range m.module.Definitions {
		if definition.Kind != "Operator" || len(definition.Params) != 0 || excluded[definition.Name] {
			continue
		}
		if !m.cacheableEffectfulDefinition(definition.Body) && !m.warningDefinition(definition.Body) {
			continue
		}
		m.precomputeEffectfulDefinition(definition.Name)
	}
}

// warningDefinition reports whether a constant-level definition contains an
// EXCEPT, which can raise warning 2144. Java TLC's SpecProcessor evaluates
// every constant definition at startup, so the warning precedes the
// computation of the initial states.
func (m *model) warningDefinition(body *parser.Expression) bool {
	return m.warning != nil && containsExcept(body) && !m.readsState(body, map[string]bool{}) && !m.hasAction(body, map[string]bool{})
}

func containsExcept(expr *parser.Expression) bool {
	found := expr.Kind == "Except"
	walkChildren(expr, func(child *parser.Expression) {
		found = found || child != nil && containsExcept(child)
	})
	return found
}

func (m *model) precomputeEffectfulDefinition(name string) {
	if m.startupAttempts[name] {
		return
	}
	if m.startupAttempts == nil {
		m.startupAttempts = make(map[string]bool)
	}
	m.startupAttempts[name] = true
	if _, err := m.eval(&parser.Expression{Kind: "Identifier", Text: name}, nil, nil); err != nil {
		if m.startupFailures == nil {
			m.startupFailures = make(map[string]bool)
		}
		m.startupFailures[name] = true
	}
}

func (m *model) cacheableEffectfulDefinition(body *parser.Expression) bool {
	if m.readsState(body, map[string]bool{}) || m.hasAction(body, map[string]bool{}) {
		return false
	}
	return m.readsRandom(body, map[string]bool{}) || m.expressionHasEffects(body, map[string]bool{})
}

func constantSubstitutionError(definition parser.Definition, err error) error {
	message := fmt.Sprintf("The configuration file substitutes constant %s with non-constant %s", definition.Name, definition.ConfigSubstitutionTarget)
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		message += " - specifically: " + unboundVariableFailure(missing).Message
	} else if failure, ok := errors.AsType[*evaluator.RuntimeError](err); ok {
		if !failure.Standard {
			message += " - specifically: " + failure.Message
		}
	} else {
		return err
	}
	return &ConfigurationError{Message: message, Code: 2281, Err: err}
}
