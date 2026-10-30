package checker

import (
	"fmt"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// lowerSpecification extracts an initial predicate, a next-state action and
// named weak or strong fairness actions from a conjunction.
func lowerSpecification(module *parser.Module, cfg config.Config) (config.Config, []fairnessCondition, error) {
	if cfg.Specification == "" {
		return cfg, nil, nil
	}
	if err := validateSpecConfig(cfg); err != nil {
		return cfg, nil, err
	}
	spec := findSpecification(module, cfg.Specification)
	parts := specificationParts{module: module}
	if spec == nil || !parts.collect(spec, map[string]bool{}) {
		return cfg, nil, fmt.Errorf("unsupported SPECIFICATION form")
	}
	parts.defaultEmptyInit()
	if parts.init == "" || parts.next == "" {
		return cfg, nil, fmt.Errorf("unsupported SPECIFICATION form")
	}
	cfg.Init = parts.init
	cfg.Next = parts.next
	cfg.Specification = ""
	return cfg, parts.fairness, nil
}

func (p *specificationParts) defaultEmptyInit() {
	if p.init == "" && len(p.module.Variables) == 0 {
		p.init = p.addWrapper("init", &parser.Expression{Kind: "Identifier", Text: "TRUE"})
	}
}

func validateSpecConfig(cfg config.Config) error {
	if cfg.Init != "" || cfg.Next != "" {
		return fmt.Errorf("SPECIFICATION cannot be combined with INIT or NEXT")
	}
	return nil
}

func findSpecification(module *parser.Module, name string) *parser.Expression {
	for _, d := range module.Definitions {
		if d.Name == name && d.Kind == "Operator" && len(d.Params) == 0 {
			return d.Body
		}
	}
	return nil
}

func validSubscript(module *parser.Module, expr *parser.Expression) bool {
	names := map[string]bool{}
	variables := map[string]bool{}
	definitions := map[string]*parser.Expression{}
	for _, variable := range module.Variables {
		variables[variable.Name] = true
	}
	for _, d := range module.Definitions {
		if d.Kind == "Operator" && len(d.Params) == 0 {
			definitions[d.Name] = d.Body
		}
	}
	if !collectSubscript(expr, names, variables, definitions, map[string]bool{}) {
		return false
	}
	return len(names) == len(variables)
}

func collectSubscript(expr *parser.Expression, names, variables map[string]bool, definitions map[string]*parser.Expression, seen map[string]bool) bool {
	if expr.Kind == "Identifier" {
		return subscriptIdentifier(expr.Text, names, variables, definitions, seen)
	}
	if expr.Kind != "Tuple" {
		return false
	}
	for _, element := range expr.Args {
		if !collectSubscript(element, names, variables, definitions, seen) {
			return false
		}
	}
	return true
}

func subscriptIdentifier(name string, names, variables map[string]bool, definitions map[string]*parser.Expression, seen map[string]bool) bool {
	if variables[name] {
		if names[name] {
			return false
		}
		names[name] = true
		return true
	}
	expr, ok := definitions[name]
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return collectSubscript(expr, names, variables, definitions, seen)
}
