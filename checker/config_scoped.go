package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

type scopedReplacement struct {
	index  int
	prefix int
	name   string
	target string
}

// A scoped override applies to the original definition and to copies in every
// instance. An intervening instance can itself expose a more specific name.
func scopedConfigSubstitutions(module *parser.Module, cfg, original config.Config) ([]scopedReplacement, error) {
	if err := addScopedStandardDefinitions(module, cfg); err != nil {
		return nil, err
	}
	if err := validateScopedSources(module, cfg); err != nil {
		return nil, err
	}
	var replacements []scopedReplacement
	for i, definition := range module.Definitions {
		sub, prefix, ok := scopedDefinitionOverride(module, cfg, original, definition)
		if !ok || hasUnscopedSubstitution(cfg, definition.Name) || hasUnscopedAssignment(original, definition.Name) {
			continue
		}
		if err := validateScopedReplacement(module, cfg, definition, prefix, sub); err != nil {
			return nil, err
		}
		replacements = append(replacements, scopedReplacement{index: i, prefix: prefix, name: definition.Name, target: sub.Target})
	}
	return replacements, nil
}

func discardIneffectiveScopedAssignments(module *parser.Module, cfg config.Config) config.Config {
	assignments := make([]config.Assignment, 0, len(cfg.Constants))
	for _, assignment := range cfg.Constants {
		if assignment.Module != "" && (assignment.Module == module.Name || isConstantDeclaration(module, assignment.Name)) {
			continue
		}
		assignments = append(assignments, assignment)
	}
	cfg.Constants = assignments
	return cfg
}

func validateScopedSources(module *parser.Module, cfg config.Config) error {
	for _, sub := range cfg.Substitutions {
		if ignoredScopedSubstitution(module, sub) {
			continue
		}
		if !hasScopedSource(module, sub) {
			return fmt.Errorf("scoped substitution %s in %s is not yet supported", sub.Name, sub.Module)
		}
	}
	return nil
}

func ignoredScopedSubstitution(module *parser.Module, sub config.Substitution) bool {
	return sub.Module == "" || sub.Module == module.Name || isConstantDeclaration(module, sub.Name)
}

func hasScopedSource(module *parser.Module, sub config.Substitution) bool {
	for _, d := range module.Definitions {
		for _, origin := range definitionOrigins(d) {
			if origin.Name == sub.Name && origin.Module == sub.Module {
				return true
			}
		}
	}
	return false
}

func scopedDefinitionOverride(module *parser.Module, cfg, original config.Config, definition parser.Definition) (config.Substitution, int, bool) {
	origins := definitionOrigins(definition)
	for _, origin := range slices.Backward(origins) {
		if sub, ok := findScopedOverride(module, cfg, origin); ok {
			return sub, origin.Prefix, true
		}
		if hasScopedAssignment(original, origin) {
			return config.Substitution{}, 0, false
		}
	}
	return config.Substitution{}, 0, false
}

func definitionOrigins(definition parser.Definition) []parser.DefinitionOrigin {
	if len(definition.Origins) > 0 {
		return definition.Origins
	}
	return []parser.DefinitionOrigin{{Module: definition.Location.Range.Module, Name: definition.Name}}
}

func findScopedOverride(module *parser.Module, cfg config.Config, origin parser.DefinitionOrigin) (config.Substitution, bool) {
	for _, sub := range slices.Backward(cfg.Substitutions) {

		if sub.Module == "" || sub.Module == module.Name || isConstantDeclaration(module, sub.Name) {
			continue
		}
		if sub.Module == origin.Module && sub.Name == origin.Name {
			return sub, true
		}
	}
	return config.Substitution{}, false
}

func validateScopedReplacement(module *parser.Module, cfg config.Config, definition parser.Definition, prefix int, sub config.Substitution) error {
	target, ok := configSymbol(module, sub.Target)
	if !ok {
		return fmt.Errorf("substitution target %s is undefined or unsupported", sub.Target)
	}
	if configAssignsDefinition(module, cfg, sub.Target) {
		return fmt.Errorf("substitution target %s has an unsupported configuration assignment", sub.Target)
	}
	if prefix > len(definition.Params) {
		return fmt.Errorf("invalid instance parameter count for %s", definition.Name)
	}
	definition.Params = definition.Params[prefix:]
	return validateConfigSubstitutionTypes(sub, definition, target)
}

func replaceScopedDefinition(module *parser.Module, replacement scopedReplacement) {
	definition := module.Definitions[replacement.index]
	module.Definitions[replacement.index] = configReplacement(definition, replacement.target, replacement.prefix)
}

func unscopedSubstitutions(cfg config.Config) []config.Substitution {
	var substitutions []config.Substitution
	for _, sub := range cfg.Substitutions {
		if sub.Module == "" {
			substitutions = append(substitutions, sub)
		}
	}
	return substitutions
}

func scopedAsSubstitutions(replacements []scopedReplacement) []config.Substitution {
	var substitutions []config.Substitution
	for _, replacement := range replacements {
		substitutions = append(substitutions, config.Substitution{Name: replacement.name, Target: replacement.target})
	}
	return substitutions
}

func hasUnscopedSubstitution(cfg config.Config, name string) bool {
	for _, sub := range cfg.Substitutions {
		if sub.Module == "" && sub.Name == name {
			return true
		}
	}
	return false
}
