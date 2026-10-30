package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func groupScopedAssignments(assignments []config.Assignment) ([]configAssignmentGroup, []config.Assignment) {
	var groups []configAssignmentGroup
	var keep []config.Assignment
	indices := map[[2]string]int{}
	for _, assignment := range assignments {
		if assignment.Module == "" {
			keep = append(keep, assignment)
			continue
		}
		key := [2]string{assignment.Module, assignment.Name}
		index, ok := indices[key]
		if !ok {
			index = len(groups)
			indices[key] = index
			groups = append(groups, configAssignmentGroup{name: assignment.Name, module: assignment.Module})
		}
		groups[index].values = append(groups[index].values, assignment)
	}
	return groups, keep
}

func applyScopedAssignments(module *parser.Module, cfg *config.Config, original config.Config, groups []configAssignmentGroup) error {
	if len(groups) == 0 {
		return nil
	}
	standard := config.Config{Substitutions: append([]config.Substitution(nil), original.Substitutions...)}
	for _, group := range groups {
		standard.Substitutions = append(standard.Substitutions, config.Substitution{Module: group.module, Name: group.name})
	}
	if err := addScopedStandardDefinitions(module, standard); err != nil {
		return err
	}
	if err := validateScopedAssignmentSources(module, groups); err != nil {
		return err
	}
	return applyScopedAssignmentDefinitions(module, cfg, original, groups)
}

func validateScopedAssignmentSources(module *parser.Module, groups []configAssignmentGroup) error {
	for _, group := range groups {
		if !hasScopedSource(module, config.Substitution{Name: group.name, Module: group.module}) {
			return fmt.Errorf("scoped assignment %s in %s is not yet supported", group.name, group.module)
		}
	}
	return nil
}

func applyScopedAssignmentDefinitions(module *parser.Module, cfg *config.Config, original config.Config, groups []configAssignmentGroup) error {
	for i, definition := range module.Definitions {
		if hasUnscopedAssignment(original, definition.Name) || hasUnscopedSubstitution(original, definition.Name) {
			continue
		}
		group, prefix, ok := scopedAssignmentGroup(definition, groups)
		if !ok {
			continue
		}
		if prefix > len(definition.Params) {
			return fmt.Errorf("invalid instance parameter count for %s", definition.Name)
		}
		updated, err := makeAssignedDefinition(module, cfg, definition, group, prefix)
		if err != nil {
			return err
		}
		module.Definitions[i] = updated
	}
	return nil
}

func scopedAssignmentGroup(definition parser.Definition, groups []configAssignmentGroup) (configAssignmentGroup, int, bool) {
	for _, origin := range slices.Backward(definitionOrigins(definition)) {
		for _, group := range groups {
			if group.module == origin.Module && group.name == origin.Name {
				return group, origin.Prefix, true
			}
		}
	}
	return configAssignmentGroup{}, 0, false
}

func hasUnscopedAssignment(cfg config.Config, name string) bool {
	for _, assignment := range cfg.Constants {
		if assignment.Module == "" && assignment.Name == name {
			return true
		}
	}
	return false
}

func hasScopedAssignment(cfg config.Config, origin parser.DefinitionOrigin) bool {
	for _, assignment := range cfg.Constants {
		if assignment.Module == origin.Module && assignment.Name == origin.Name {
			return true
		}
	}
	return false
}
