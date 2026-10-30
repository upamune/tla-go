package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) evalBound(expr *parser.Expression) (value.Value, error) {
	switch expr.Kind {
	case "Forall", "Exists":
		return e.quantifier(expr)
	case "Choose":
		return e.choose(expr)
	case "SetFilter", "SetMap":
		return e.setComprehension(expr)
	case "FunctionConstruction":
		return e.functionConstruction(expr)
	case "Except":
		return e.except(expr)
	case "ExceptAt":
		return e.exceptAt()
	case "BoxAction", "AngleAction":
		return e.action(expr)
	}
	return value.Value{}, fmt.Errorf("unsupported expression kind %s", expr.Kind)
}

type boundVisitor func(*evaluator, []value.Value) (bool, error)

func (e *evaluator) walkBounds(bounds []parser.Bound, selected []value.Value, visit boundVisitor) error {
	if len(bounds) == 0 {
		_, err := visit(e, selected)
		return err
	}
	bound := bounds[0]
	if bound.Set == nil {
		return fmt.Errorf("cannot enumerate an unbounded choice")
	}
	domain, err := e.eval(bound.Set)
	if err != nil {
		return err
	}
	return domain.Each(func(item value.Value) (bool, error) {
		child, err := e.bind(bound, item)
		if err != nil {
			return false, err
		}
		stop := false
		err = child.walkBounds(bounds[1:], append(selected, item), func(next *evaluator, values []value.Value) (bool, error) {
			more, err := visit(next, values)
			if !more {
				stop = true
			}
			return more, err
		})
		return !stop, err
	})
}

func (e *evaluator) bind(bound parser.Bound, item value.Value) (*evaluator, error) {
	vals := []value.Value{item}
	if bound.Tuple {
		var err error
		vals, err = item.TupleElements()
		if err != nil {
			return nil, err
		}
	}
	if len(vals) != len(bound.Names) {
		return nil, fmt.Errorf("tuple binding has wrong arity")
	}
	bindings := make(map[string]value.Value, len(vals))
	for i, name := range bound.Names {
		bindings[name] = vals[i]
	}
	child := *e
	child.scope = &scope{parent: e.scope, values: bindings}
	return &child, nil
}

func (e *evaluator) quantifier(expr *parser.Expression) (value.Value, error) {
	result := expr.Kind == "Forall"
	err := e.walkBounds(expr.Bounds, nil, func(child *evaluator, _ []value.Value) (bool, error) {
		b, err := child.condition(expr.Args[0])
		if err != nil {
			return false, err
		}
		if b != result {
			result = b
			return false, nil
		}
		return true, nil
	})
	return value.Bool(result), err
}

func (e *evaluator) choose(expr *parser.Expression) (value.Value, error) {
	var result value.Value
	found := false
	err := e.walkBounds(expr.Bounds, nil, func(child *evaluator, selected []value.Value) (bool, error) {
		b, err := child.condition(expr.Args[0])
		if err != nil {
			return false, err
		}
		if b {
			result = selected[0]
			found = true
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return value.Value{}, err
	}
	if !found {
		location := expr.Location.Range
		message := fmt.Sprintf("Attempted to compute the value of an expression of form\nCHOOSE x \\in S: P, but no element of S satisfied P.\nline %d, col %d to line %d, col %d of module %s", location.Start.Line, location.Start.Column, location.End.Line, location.End.Column, location.Module)
		return value.Value{}, &RuntimeError{Message: message, Legacy: "no value satisfies CHOOSE"}
	}
	return result, nil
}

func (e *evaluator) comprehension(expr *parser.Expression) (value.Value, error) {
	var elements []value.Value
	err := e.walkBounds(expr.Bounds, nil, func(child *evaluator, _ []value.Value) (bool, error) {
		v, err := child.eval(expr.Args[0])
		if err != nil {
			return false, err
		}

		elements = append(elements, v)
		return true, nil
	})
	if err != nil {
		return value.Value{}, err
	}
	return e.sourceSet(elements)
}

func (e *evaluator) functionConstruction(expr *parser.Expression) (value.Value, error) {
	domain, err := e.boundsDomain(expr.Bounds)
	if err != nil {
		return value.Value{}, err
	}
	if e.keepLazy || !domain.IsEnumerable() {
		return e.lazyFunctionOnDomain(domain, expr.Bounds, expr.Args[0], "")
	}
	var pairs []value.Pair
	err = e.walkBounds(expr.Bounds, nil, func(child *evaluator, selected []value.Value) (bool, error) {
		v, err := child.eval(expr.Args[0])
		if err != nil {
			return false, err
		}
		key, err := functionKey(selected)
		if err != nil {
			return false, err
		}
		pairs = append(pairs, value.Pair{Key: key, Value: v})
		return true, nil
	})
	if err != nil {
		return value.Value{}, err
	}
	return value.FunctionWithDomain(pairs, domain)
}

func functionKey(selected []value.Value) (value.Value, error) {
	if len(selected) == 1 {
		return selected[0], nil
	}
	return value.Tuple(selected)
}

func (e *evaluator) functionSet(expr *parser.Expression) (value.Value, error) {
	domain, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	codomain, err := e.eval(expr.Args[1])
	if err != nil {
		return value.Value{}, err
	}
	return value.FunctionSet(domain, codomain)
}
