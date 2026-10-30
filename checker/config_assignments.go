package checker

import (
	"fmt"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

type configAssignmentGroup struct {
	name, module string
	values       []config.Assignment
}

// Definitions produced here use private constants for configuration values;
// this also handles model values and sets without parsing them as TLA+ syntax.
func applyConfigAssignments(module *parser.Module, cfg config.Config) (*parser.Module, config.Config, error) {
	groups, keep := groupConfigAssignments(module, cfg)
	scoped, keep := groupScopedAssignments(keep)
	if len(groups) == 0 && len(scoped) == 0 {
		cfg.Constants = keep
		return module, cfg, nil
	}
	if err := validateResolved(module); err != nil {
		return nil, cfg, err
	}
	copyModule := *module
	copyModule.Definitions = append([]parser.Definition(nil), module.Definitions...)
	copyModule.Constants = append([]parser.Declaration(nil), module.Constants...)
	original := cfg
	cfg.Constants = keep
	if err := applyScopedAssignments(&copyModule, &cfg, original, scoped); err != nil {
		return nil, cfg, err
	}
	for _, group := range groups {
		if err := applyConfigAssignmentGroup(&copyModule, &cfg, group); err != nil {
			return nil, cfg, err
		}
	}
	return &copyModule, cfg, nil
}

func groupConfigAssignments(module *parser.Module, cfg config.Config) ([]configAssignmentGroup, []config.Assignment) {
	var groups []configAssignmentGroup
	var keep []config.Assignment
	indices := map[string]int{}
	for _, assignment := range cfg.Constants {
		if assignment.Module == "" && hasUnscopedSubstitution(cfg, assignment.Name) {
			continue
		}
		if !needsConfigDefinition(module, assignment) {
			keep = append(keep, assignment)
			continue
		}
		index, ok := indices[assignment.Name]
		if !ok {
			index = len(groups)
			indices[assignment.Name] = index
			groups = append(groups, configAssignmentGroup{name: assignment.Name})
		}
		groups[index].values = append(groups[index].values, assignment)
	}
	return groups, keep
}

func needsConfigDefinition(module *parser.Module, assignment config.Assignment) bool {
	if assignment.Module != "" {
		return false
	}
	for _, definition := range module.Definitions {
		if definition.Name == assignment.Name {
			return true
		}
	}
	for _, declaration := range module.Constants {
		if declaration.Name == assignment.Name {
			return declaration.Arity > 0 || assignment.Args != nil
		}
	}
	return configStandardValue(module, assignment.Name)
}

func configStandardValue(module *parser.Module, name string) bool {
	if name == "Nat" {
		return hasNaturalModule(module.Extends)
	}
	if name == "Int" {
		return isScopedStandardValue(module, config.Substitution{Name: name, Module: "Integers"})
	}
	return false
}

func applyConfigAssignmentGroup(module *parser.Module, cfg *config.Config, group configAssignmentGroup) error {
	definition, ok := configSymbol(module, group.name)
	if !ok {
		definition = parser.Definition{Kind: "Operator", Name: group.name}
	}
	definition, err := makeAssignedDefinition(module, cfg, definition, group, 0)
	if err != nil {
		return err
	}
	installConfigDefinition(module, definition)
	return nil
}

func makeAssignedDefinition(module *parser.Module, cfg *config.Config, definition parser.Definition, group configAssignmentGroup, prefix int) (parser.Definition, error) {
	definition = configAssignmentDefinition(definition)
	definition.ConfigConstant = false
	rows := configTableRows(group.values)
	if len(rows) > 0 {
		body, err := configTableBody(module, cfg, definition.Params[prefix:], rows)
		if err != nil {
			return definition, fmt.Errorf("operator %s: %w", group.name, err)
		}
		definition.Body = body
	} else {
		definition.Body = addConfigValue(module, cfg, group.values[len(group.values)-1].Value)
		definition.ConfigConstant = len(definition.Params) > 0
	}
	return definition, nil
}

func configAssignmentDefinition(definition parser.Definition) parser.Definition {
	definition.Kind = "Operator"
	definition.Bounds = nil
	definition.Recursive = false
	definition.Location.Position = parser.Position{}
	definition.Params = append([]parser.Declaration(nil), definition.Params...)
	for i := range definition.Params {
		definition.Params[i].Name = fmt.Sprintf("!config-param!%d", i)
	}
	return definition
}

func configTableRows(assignments []config.Assignment) []config.Assignment {
	var rows []config.Assignment
	for _, assignment := range assignments {
		if assignment.Args != nil {
			rows = append(rows, assignment)
		}
	}
	return rows
}

func installConfigDefinition(module *parser.Module, definition parser.Definition) {
	for i, d := range module.Definitions {
		if d.Name == definition.Name {
			module.Definitions[i] = definition
			return
		}
	}
	for i, d := range module.Constants {
		if d.Name == definition.Name {
			module.Constants = append(module.Constants[:i], module.Constants[i+1:]...)
			break
		}
	}
	module.Definitions = append(module.Definitions, definition)
}
