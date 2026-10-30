package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

// SemanticMetadata describes the subset of SANY's semantic output used by
// statechart. Levels follow TLA+'s constant, state, action and temporal levels.
type SemanticMetadata struct {
	Module      string
	Variables   []string
	Constants   []string
	Definitions []DefinitionLevel
}

type DefinitionLevel struct {
	Name  string
	Level int
}

// AnalyzeMetadata resolves a module independently of any TLC configuration.
func AnalyzeMetadata(ctx context.Context, module *parser.Module, loader ModuleLoader) (SemanticMetadata, error) {
	if module == nil {
		return SemanticMetadata{}, fmt.Errorf("missing module")
	}
	session := newInstanceSession(module.Name)
	session.sourceUnits[module.Name] = module
	linked, err := resolveExtensionsScoped(ctx, module, loader, session)
	if err != nil {
		return SemanticMetadata{}, err
	}
	linked, err = resolveInstancesScoped(ctx, linked, loader, session)
	if err != nil {
		return SemanticMetadata{}, err
	}
	linked, err = resolveTemporalExpressions(linked)
	if err != nil {
		return SemanticMetadata{}, err
	}
	if err = validateModules(linked); err != nil {
		return SemanticMetadata{}, err
	}
	if err = validateResolved(linked); err != nil {
		return SemanticMetadata{}, err
	}
	if err = validateMetadataEnabledLevels(linked); err != nil {
		return SemanticMetadata{}, &SemanticError{Err: err}
	}
	return semanticMetadata(ctx, linked)
}

func validateMetadataEnabledLevels(module *parser.Module) error {
	m := &model{module: module, defs: make(map[string]parser.Definition, len(module.Definitions))}
	for _, d := range module.Definitions {
		m.defs[d.Name] = d
	}
	return m.validateEnabledLevels()
}

func semanticMetadata(ctx context.Context, module *parser.Module) (SemanticMetadata, error) {
	result := SemanticMetadata{Module: module.Name}
	scope := newMetadataScope(module)
	for _, variable := range module.Variables {
		result.Variables = append(result.Variables, variable.Name)
	}
	for _, constant := range module.Constants {
		result.Constants = append(result.Constants, constant.Name)
	}
	for _, definition := range module.Definitions {
		if strings.HasPrefix(definition.Name, "!") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return SemanticMetadata{}, err
		}
		level, err := scope.definition(scope.defs[definition.Name], nil, map[*metadataDefinition]bool{})
		if err != nil {
			return SemanticMetadata{}, fmt.Errorf("definition %s: %w", definition.Name, err)
		}
		result.Definitions = append(result.Definitions, DefinitionLevel{Name: metadataDefinitionName(definition), Level: level})
	}
	return result, nil
}

func metadataDefinitionName(definition parser.Definition) string {
	if !definition.BuiltinInstance {
		return definition.Name
	}
	if before, ok := strings.CutSuffix(definition.Name, "<="); ok {
		return before + `\leq`
	}
	if before, ok := strings.CutSuffix(definition.Name, ">="); ok {
		return before + `\geq`
	}
	return definition.Name
}
