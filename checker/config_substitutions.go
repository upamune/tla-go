package checker

import (
	"fmt"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// applyConfigSubstitutions expands configuration aliases after linking modules,
// but before lowering SPECIFICATION so aliases also affect Init and Next.
func applyConfigSubstitutions(module *parser.Module, cfg, original config.Config) (*parser.Module, config.Config, error) {
	if err := validateResolved(module); err != nil {
		return nil, cfg, err
	}
	copyModule := *module
	copyModule.Definitions = append([]parser.Definition(nil), module.Definitions...)
	copyModule.Constants = append([]parser.Declaration(nil), module.Constants...)
	scoped, err := scopedConfigSubstitutions(&copyModule, cfg, original)
	if err != nil {
		return nil, cfg, err
	}
	unscoped := unscopedSubstitutions(cfg)
	if err := validateUnscopedSubstitutions(&copyModule, cfg, unscoped); err != nil {
		return nil, cfg, err
	}
	applySubstitutionReplacements(&copyModule, scoped, unscoped)
	if err := validateConfigSubstitutionCycles(&copyModule, append(unscoped, scopedAsSubstitutions(scoped)...)); err != nil {
		return nil, cfg, err
	}
	cfg.Substitutions = nil
	return &copyModule, cfg, nil
}

func validateUnscopedSubstitutions(module *parser.Module, cfg config.Config, substitutions []config.Substitution) error {
	for _, sub := range substitutions {
		if err := validateConfigSubstitution(module, cfg, sub); err != nil {
			return err
		}
	}
	return nil
}

func applySubstitutionReplacements(module *parser.Module, scoped []scopedReplacement, unscoped []config.Substitution) {
	for _, replacement := range scoped {
		replaceScopedDefinition(module, replacement)
	}
	for _, sub := range unscoped {
		replaceConfigDefinition(module, sub)
	}
}

func validateConfigSubstitutionCycles(module *parser.Module, substitutions []config.Substitution) error {
	for _, sub := range substitutions {
		if configDependsOn(module, sub.Target, sub.Name, map[string]bool{}) {
			return fmt.Errorf("cyclic substitution for %s", sub.Name)
		}
	}
	return nil
}

func validateConfigSubstitution(module *parser.Module, cfg config.Config, sub config.Substitution) error {
	if err := validateConfigSubstitutionConflicts(cfg, sub); err != nil {
		return err
	}
	source, ok := configSymbol(module, sub.Name)
	if !ok {
		return fmt.Errorf("substitution source %s is undefined or unsupported", sub.Name)
	}
	target, ok := configSymbol(module, sub.Target)
	if !ok {
		return fmt.Errorf("substitution target %s is undefined or unsupported", sub.Target)
	}
	if configAssignsDefinition(module, cfg, sub.Target) {
		return fmt.Errorf("substitution target %s has a configuration assignment, which is not yet supported", sub.Target)
	}
	return validateConfigSubstitutionTypes(module, sub, source, target)
}

func configAssignsDefinition(module *parser.Module, cfg config.Config, name string) bool {
	if isConstantDeclaration(module, name) {
		return false
	}
	for _, assignment := range cfg.Constants {
		if assignment.Name == name {
			return true
		}
	}
	return false
}

func validateConfigSubstitutionTypes(module *parser.Module, sub config.Substitution, source, target parser.Definition) error {
	if len(source.Params) != len(target.Params) {
		return fmt.Errorf("substitution %s <- %s has different arity", sub.Name, sub.Target)
	}
	if hasOperatorParameters(source.Params) || hasOperatorParameters(target.Params) {
		return fmt.Errorf("substitution of higher-order operators is not yet supported")
	}
	if isConstantDeclaration(module, sub.Name) && configTargetReadsState(module, target) {
		return fmt.Errorf("substitution target %s is not constant", sub.Target)
	}
	return nil
}

func validateConfigSubstitutionConflicts(cfg config.Config, sub config.Substitution) error {
	count := 0
	for _, other := range cfg.Substitutions {
		if other.Name == sub.Target {
			return fmt.Errorf("substitution target %s is itself substituted", sub.Target)
		}
		if other.Name == sub.Name && other.Module == "" {
			count++
		}
	}
	if count > 1 {
		return fmt.Errorf("duplicate substitution for %s", sub.Name)
	}
	for _, assignment := range cfg.Constants {
		if assignment.Name == sub.Name {
			return fmt.Errorf("substitution and assignment for %s are not yet supported together", sub.Name)
		}
	}
	return nil
}

func hasOperatorParameters(params []parser.Declaration) bool {
	for _, param := range params {
		if param.Arity > 0 {
			return true
		}
	}
	return false
}

func configDependsOn(module *parser.Module, name, source string, seen map[string]bool) bool {
	if name == source {
		return true
	}
	if seen[name] {
		return false
	}
	seen[name] = true
	definition, ok := configSymbol(module, name)
	if !ok || definition.Body == nil {
		return false
	}
	return configExpressionDependsOn(module, definition.Body, source, seen)
}

func configExpressionDependsOn(module *parser.Module, expr *parser.Expression, source string, seen map[string]bool) bool {
	if (expr.Kind == "Identifier" || expr.Kind == "Apply") && configDependsOn(module, expr.Text, source, seen) {
		return true
	}
	depends := false
	walkChildren(expr, func(child *parser.Expression) {
		if !depends && configExpressionDependsOn(module, child, source, seen) {
			depends = true
		}
	})
	return depends
}

func configSymbol(module *parser.Module, name string) (parser.Definition, bool) {
	for _, definition := range module.Definitions {
		if definition.Name == name {
			return definition, true
		}
	}
	for _, declaration := range module.Constants {
		if declaration.Name == name {
			return configConstantDefinition(declaration), true
		}
	}
	return parser.Definition{}, false
}

func configConstantDefinition(declaration parser.Declaration) parser.Definition {
	d := parser.Definition{Kind: "Operator", Name: declaration.Name, Location: declaration.Location}
	for i := 0; i < declaration.Arity; i++ {
		d.Params = append(d.Params, parser.Declaration{Name: fmt.Sprintf("__config_arg_%d", i)})
	}
	return d
}

func isConstantDeclaration(module *parser.Module, name string) bool {
	for _, declaration := range module.Constants {
		if declaration.Name == name {
			return true
		}
	}
	return false
}

func configTargetReadsState(module *parser.Module, target parser.Definition) bool {
	m := model{defs: map[string]parser.Definition{}, variables: map[string]bool{}}
	for _, definition := range module.Definitions {
		m.defs[definition.Name] = definition
	}
	for _, variable := range module.Variables {
		m.variables[variable.Name] = true
	}
	return target.Body != nil && (m.readsState(target.Body, map[string]bool{}) || m.hasAction(target.Body, map[string]bool{}))
}

func replaceConfigDefinition(module *parser.Module, sub config.Substitution) {
	source, _ := configSymbol(module, sub.Name)
	source = configReplacement(source, sub.Target, 0)
	for i, d := range module.Definitions {
		if d.Name == sub.Name {
			module.Definitions[i] = source
			return
		}
	}
	for i, d := range module.Constants {
		if d.Name == sub.Name {
			module.Constants = append(module.Constants[:i], module.Constants[i+1:]...)
			break
		}
	}
	module.Definitions = append(module.Definitions, source)
}

func configReplacement(source parser.Definition, target string, prefix int) parser.Definition {
	source.Kind = "Operator"
	source.ConfigConstant = false
	source.Bounds = nil
	source.Recursive = false
	// A configuration can refer to a later definition; source validation above
	// has already enforced the original module's declaration order.
	source.Location.Position = parser.Position{}
	source.Params = append([]parser.Declaration(nil), source.Params...)
	for i := range source.Params {
		source.Params[i].Name = fmt.Sprintf("!config-param!%d", i)
	}
	source.Body = configCall(target, source.Params[prefix:])
	return source
}

func configCall(name string, params []parser.Declaration) *parser.Expression {
	expr := &parser.Expression{Kind: "Identifier", Text: name}
	if len(params) > 0 {
		expr.Kind = "Apply"
		for _, param := range params {
			expr.Args = append(expr.Args, &parser.Expression{Kind: "Identifier", Text: param.Name})
		}
	}
	return expr
}
