package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *instanceResolver) nestedProofScope(module, parent *parser.Module) (parser.Module, error) {
	check := nestedScope(module, parent)
	if moduleUsesProofModules(module) {
		if err := r.includeParentInstanceModules(&check, parent); err != nil {
			return parser.Module{}, err
		}
	}
	return check, nil
}

// A nested proof can name modules imported by any instance in its parent,
// including an instance appearing later in the source. Load those modules only
// when the nested module actually refers to MODULE facts.
func (r *instanceResolver) includeParentInstanceModules(target, parent *parser.Module) error {
	for _, instance := range parentProofInstances(parent) {
		if isNestedProofModule(parent, instance.Module) {
			continue
		}
		module, err := r.instanceModule(instance)
		if err != nil {
			return err
		}
		addProofModules(target, module.Name)
		addProofModules(target, module.ProofModules...)
		addProofModules(target, module.Extends...)
	}
	return nil
}

func isNestedProofModule(parent *parser.Module, name string) bool {
	for _, nested := range parent.Modules {
		if nested.Name == name {
			return true
		}
	}
	return false
}

func moduleUsesProofModules(module *parser.Module) bool {
	for _, directive := range module.ProofDirectives {
		if factsContainModules(directive.Facts) {
			return true
		}
	}
	for _, theorem := range module.Theorems {
		if proofUsesModules(theorem.Proof) {
			return true
		}
	}
	return false
}

func proofUsesModules(proof *parser.Proof) bool {
	if proof == nil {
		return false
	}
	if factsContainModules(proof.Facts) {
		return true
	}
	for _, step := range proof.Steps {
		if step.Directive != nil && factsContainModules(step.Directive.Facts) {
			return true
		}
		if proofUsesModules(step.Proof) {
			return true
		}
	}
	return false
}

func factsContainModules(facts []*parser.Expression) bool {
	for _, fact := range facts {
		if fact.Kind == "ModuleFact" {
			return true
		}
	}
	return false
}
