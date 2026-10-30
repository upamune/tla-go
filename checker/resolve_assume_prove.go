package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// NEW declarations are local to the theorem's ASSUME/PROVE and become visible
// in subsequent assumptions and the conclusion, not in their own bound.
func (r *resolver) assumeProveExpression(expr *parser.Expression, scope map[string]symbol) error {
	_, err := r.assumeProveScope(expr, scope)
	return err
}

func (r *resolver) assumeProveScope(expr *parser.Expression, scope map[string]symbol) (map[string]symbol, error) {
	local := cloneScope(scope)
	declared := map[string]bool{}
	for _, clause := range expr.Args {
		if clause.Kind == "NewDeclaration" {
			if err := r.newProofDeclaration(clause, local, declared); err != nil {
				return nil, err
			}
			continue
		}
		if err := r.expression(clause, local); err != nil {
			return nil, err
		}
	}
	return local, nil
}

func (r *resolver) newProofDeclaration(expr *parser.Expression, scope map[string]symbol, declared map[string]bool) error {
	name := expr.Names[0]
	if r.newProofConflicts(name) {
		return fmt.Errorf("NEW declaration %s conflicts with a module symbol", name)
	}
	if _, exists := scope[name]; exists && !declared[name] {
		return fmt.Errorf("NEW declaration %s conflicts with a local symbol", name)
	}
	for _, bound := range expr.Args {
		if err := r.expression(bound, scope); err != nil {
			return err
		}
	}
	scope[name] = symbol{arity: expr.ArgumentCounts[0]}
	declared[name] = true
	return nil
}

func (r *resolver) newProofConflicts(name string) bool {
	if r.declaredBefore(name) {
		return true
	}
	if _, ok := coreOperators[name]; ok {
		return true
	}
	_, ok := r.lookupStandard(name)
	return ok
}
