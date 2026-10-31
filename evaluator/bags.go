package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var bagArities = map[string]int{
	"IsABag": 1, "BagCardinality": 1, "BagIn": 2, "CopiesIn": 2,
	"\\oplus": 2, "\\ominus": 2, "BagUnion": 1, "\\sqsubseteq": 2,
	"BagOfAll": 2, "SubBag": 1, "BagToSet": 1, "SetToBag": 1,
}

func isBagOperator(name string) bool { _, ok := bagArities[name]; return ok }

func (e *evaluator) bagOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != bagArities[expr.Text] {
		return value.Value{}, fmt.Errorf("%s expects %d arguments", expr.Text, bagArities[expr.Text])
	}
	if expr.Text == "BagOfAll" {
		return e.bagOfAll(expr.Args)
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return e.program.bagValue(expr.Text, args)
}

func (p *Program) bagValue(name string, args []value.Value) (value.Value, error) {
	switch name {
	case "IsABag":
		pairs, err := args[0].Pairs()
		return value.Bool(err == nil && validBag(pairs)), nil
	case "BagCardinality":
		return bagCardinality(args[0])
	case "BagIn", "CopiesIn":
		return bagLookup(name, args[0], args[1])
	case "\\oplus", "\\ominus", "\\sqsubseteq":
		return bagBinary(name, args[0], args[1])
	default:
		return p.bagCollection(name, args[0])
	}
}

func validBag(pairs []value.Pair) bool {
	for _, pair := range pairs {
		n, ok := pair.Value.Int()
		if !ok || n <= 0 {
			return false
		}
	}
	return true
}

func bagError(message string) error {
	return &RuntimeError{Message: message, Standard: true}
}

func bagFunction(name string, v value.Value) ([]value.Pair, error) {
	pairs, err := v.Pairs()
	if err != nil {
		return nil, bagError(fmt.Sprintf("Applying %s to the following value,\nwhich is not a function with a finite domain:\n%s", name, v))
	}
	return pairs, nil
}

func bagCardinality(v value.Value) (value.Value, error) {
	pairs, err := bagFunction("BagCardinality", v)
	if err != nil {
		return value.Value{}, err
	}
	if !validBag(pairs) {
		return value.Value{}, bagError(fmt.Sprintf("Applying BagCardinality to the following value,\nwhich is not a bag:\n%s", v))
	}
	var sum int32
	for _, pair := range pairs {
		n, _ := pair.Value.Int()
		v, err := value.Integer(int64(sum) + int64(n))
		if err != nil {
			return value.Value{}, bagError(fmt.Sprintf("Overflow when computing %d+%d", sum, n))
		}
		sum, _ = v.Int()
	}
	return value.Integer(int64(sum))
}

func bagLookup(name string, element, bag value.Value) (value.Value, error) {
	pairs, err := bagFunction(name, bag)
	if err != nil {
		return value.Value{}, err
	}
	count, err := bagFind(pairs, element)
	if err != nil {
		return value.Value{}, err
	}
	n, ok := count.Int()
	if !ok {
		return value.Value{}, bagError(fmt.Sprintf("The second argument of %s should be a bag, but instead it is:\n%s", name, bag))
	}
	if name == "BagIn" {
		return value.Bool(n > 0), nil
	}
	return count, nil
}

func bagFind(pairs []value.Pair, key value.Value) (value.Value, error) {
	for _, pair := range pairs {
		equal, err := value.Equal(pair.Key, key)
		if err != nil {
			return value.Value{}, err
		}
		if equal {
			return pair.Value, nil
		}
	}
	return value.Integer(0)
}
