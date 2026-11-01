package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func validateLetInstanceNames(expr *parser.Expression) error {
	names := make(map[string]bool, len(expr.Definitions)+len(expr.Instances))
	instances := make(map[string]int, len(expr.Instances))
	for _, definition := range expr.Definitions {
		names[definition.Name] = true
	}
	for _, instance := range expr.Instances {
		if instance.Name == "" {
			continue
		}
		if names[instance.Name] {
			return &SemanticError{Err: fmt.Errorf("duplicate LET instance name %s", instance.Name)}
		}
		if _, err := registerInstanceName(instances, instance); err != nil {
			return err
		}
	}
	return nil
}

func registerInstanceName(instances map[string]int, instance parser.Instance) (bool, error) {
	if arity, exists := instances[instance.Name]; exists {
		if arity != len(instance.Params) {
			return false, &SemanticError{Err: fmt.Errorf("instance %s has incompatible arity", instance.Name)}
		}
		return false, nil
	}
	instances[instance.Name] = len(instance.Params)
	return true, nil
}

// A named instance itself occupies SANY's symbol table, even if it exports no
// operator that collides with a surrounding definition or instance.
func (r *instanceResolver) validateOuterInstanceName(instance parser.Instance, locals []parser.Definition, outer []parser.Instance) error {
	if instance.Name == "" {
		return nil
	}
	if arity, found := priorInstanceArity(instance, outer); found {
		if arity != len(instance.Params) {
			return instanceNameConflict(instance.Name)
		}
		return nil
	}
	if definitionNameConflict(instance, r.root.Definitions) || definitionNameConflict(instance, locals) ||
		r.declarationNameConflict(instance, r.root.Constants) || r.declarationNameConflict(instance, r.root.Variables) {
		return instanceNameConflict(instance.Name)
	}
	return nil
}

func priorInstanceArity(current parser.Instance, outer []parser.Instance) (int, bool) {
	for _, previous := range outer {
		if previous.Name == current.Name && instanceAvailableBefore(previous, current) {
			return len(previous.Params), true
		}
	}
	return 0, false
}

func definitionNameConflict(instance parser.Instance, definitions []parser.Definition) bool {
	for _, d := range definitions {
		if d.Name == instance.Name && definitionAvailableBefore(d, instance) {
			return true
		}
	}
	return false
}

func (r *instanceResolver) declarationNameConflict(instance parser.Instance, declarations []parser.Declaration) bool {
	for _, d := range declarations {
		if d.Name == instance.Name && sourceVisibleAtInstance(sourceDeclaration(r.root, d), instance) {
			return true
		}
	}
	return false
}

func instanceNameConflict(name string) error {
	return &SemanticError{Err: fmt.Errorf("Multiply-defined symbol '%s' with incompatible kind or arity", name)}
}

func instanceAvailableBefore(previous, current parser.Instance) bool {
	source := sourceSymbol{module: previous.Location.Range.Module, position: previous.Location.Range.End}
	return strictlyBeforeInstance(source, current)
}

func definitionAvailableBefore(d parser.Definition, current parser.Instance) bool {
	pos := d.Location.Range.End
	if d.Recursive {
		pos = d.Location.Range.Start
	}
	return strictlyBeforeInstance(sourceSymbol{module: d.Location.Range.Module, position: pos}, current)
}

func strictlyBeforeInstance(source sourceSymbol, instance parser.Instance) bool {
	ref := sourceSymbolLocation(instance.Location)
	if source.module == "" || ref.module == "" {
		return false
	}
	if source.module != ref.module {
		return true
	}
	return comparableInstancePositions(source.position, ref.position) && knownPositionAfter(ref.position, source.position)
}
