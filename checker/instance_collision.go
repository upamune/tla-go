package checker

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

type instanceBinding struct {
	arity int
	kind  string
}

// SANY keeps the first visible binding when an INSTANCE imports an operator
// with the same name and arity. The new operator still exists for references
// inside its own module, so it needs a distinct internal name.
func (r *instanceResolver) instanceBindings(result, module *parser.Module, instance parser.Instance, index int) (map[string]string, error) {
	names := instanceDefinitionNames(module, instance, index)
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		if !strings.HasPrefix(name, "!") {
			wanted[name] = true
		}
	}
	existing := r.existingInstanceBindings(result, instance, wanted)
	for _, d := range module.Definitions {
		name := names[d.Name]
		binding, found := existing[name]
		if !found {
			continue
		}
		if binding.arity != len(d.Params)+len(instance.Params) || binding.kind != r.instanceDefinitionKind(d, module) {
			return nil, &SemanticError{Err: fmt.Errorf("Multiply-defined symbol '%s' with incompatible kind or arity", name)}
		}
		names[d.Name] = fmt.Sprintf("!instance-binding!%d!%s", r.session.nextInstance, d.Name)
	}
	return names, nil
}

func (r *instanceResolver) existingInstanceBindings(result *parser.Module, instance parser.Instance, wanted map[string]bool) map[string]instanceBinding {
	existing := make(map[string]instanceBinding)
	for _, module := range []*parser.Module{result, r.root} {
		r.addExistingDefinitions(existing, wanted, module.Definitions, instance)
	}
	r.addExistingDeclarations(existing, wanted, r.root.Constants, instance)
	r.addExistingDeclarations(existing, wanted, r.root.Variables, instance)
	return existing
}

func (r *instanceResolver) addExistingDefinitions(existing map[string]instanceBinding, wanted map[string]bool, definitions []parser.Definition, instance parser.Instance) {
	for _, d := range definitions {
		if wanted[d.Name] && definitionVisibleAtInstance(d, instance) {
			if _, ok := existing[d.Name]; !ok {
				existing[d.Name] = instanceBinding{arity: len(d.Params), kind: r.instanceDefinitionKind(d, nil)}
			}
		}
	}
}

func (r *instanceResolver) addExistingDeclarations(existing map[string]instanceBinding, wanted map[string]bool, declarations []parser.Declaration, instance parser.Instance) {
	for _, decl := range declarations {
		if wanted[decl.Name] && sourceVisibleAtInstance(sourceDeclaration(r.root, decl), instance) {
			existing[decl.Name] = instanceBinding{kind: "declaration"}
		}
	}
}

func definitionVisibleAtInstance(d parser.Definition, instance parser.Instance) bool {
	source := sourceSymbolLocation(d.Location)
	if len(d.Origins) > 0 {
		source = sourceSymbol{module: d.InstanceRange.Module, position: d.InstanceRange.Start}
	}
	return sourceVisibleAtInstance(source, instance)
}

func sourceVisibleAtInstance(source sourceSymbol, instance parser.Instance) bool {
	ref := sourceSymbolLocation(instance.Location)
	if source.module == "" || ref.module == "" {
		return false
	}
	if source.module != ref.module {
		return true
	}
	a, b := source.position, ref.position
	if !comparableInstancePositions(a, b) {
		return false
	}
	return !knownPositionAfter(a, b)
}

func comparableInstancePositions(a, b parser.Position) bool {
	return a.Line != 0 && b.Line != 0 && (a.Line != b.Line || a.Column != 0 && b.Column != 0)
}

func (r *instanceResolver) instanceDefinitionKind(d parser.Definition, module *parser.Module) string {
	if moduleHasProofName(module, d.Name) || moduleHasProofName(r.session.sourceUnits[d.Location.Range.Module], d.Name) {
		return "proof"
	}
	for _, origin := range d.Origins {
		if moduleHasProofName(r.session.sourceUnits[origin.Module], origin.Name) {
			return "proof"
		}
	}
	return "definition"
}

func moduleHasProofName(module *parser.Module, name string) bool {
	if module == nil {
		return false
	}
	for _, theorem := range module.Theorems {
		if theorem.Name == name {
			return true
		}
	}
	for _, assumption := range module.Assumptions {
		if assumption.Name == name {
			return true
		}
	}
	return false
}
