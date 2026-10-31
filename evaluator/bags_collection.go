package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (p *Program) bagCollection(name string, arg value.Value) (value.Value, error) {
	switch name {
	case "BagToSet":
		pairs, err := bagFunction(name, arg)
		if err != nil {
			return value.Value{}, err
		}
		return bagDomain(pairs)
	case "SetToBag":
		return setToBag(arg)
	case "BagUnion":
		return bagUnion(arg, methodBound{p, bagUnionSignature})
	case "SubBag":
		return subBags(arg)
	default:
		return value.Value{}, fmt.Errorf("unknown Bags operator %s", name)
	}
}

func bagDomain(pairs []value.Pair) (value.Value, error) {
	keys := make([]value.Value, len(pairs))
	for i, pair := range pairs {
		keys[i] = pair.Key
	}
	return value.Set(keys)
}

func setToBag(set value.Value) (value.Value, error) {
	elements, err := set.Elements()
	if err != nil {
		return value.Value{}, bagError(fmt.Sprintf("Applying BagToSet to the following value,\nwhich is not a function with a finite domain:\n%s", set))
	}
	one, _ := value.Integer(1)
	pairs := make([]value.Pair, len(elements))
	for i, element := range elements {
		pairs[i] = value.Pair{Key: element, Value: one}
	}
	return value.Function(pairs)
}

func bagUnion(set value.Value, bound methodBound) (value.Value, error) {
	elements, err := set.Elements()
	if err != nil {
		return value.Value{}, bagError(fmt.Sprintf("Applying BagUnion to the following value,\nwhich is not a finite enumerable set:\n%s", set))
	}
	if len(elements) == 0 {
		return value.Function(nil)
	}
	if len(elements) == 1 {
		return elements[0], nil
	}
	bags := make([][]value.Pair, 0, len(elements))
	for _, element := range elements {
		pairs, err := element.Pairs()
		if err != nil {
			return value.Value{}, bagError(fmt.Sprintf("Attempted to apply BagUnion to the following set, whose\nelement is not a bag:\n%s", set))
		}
		bags = append(bags, pairs)
	}
	return mergeBags(bags, "BagUnion", bound)
}

func (e *evaluator) bagOfAll(args []*parser.Expression) (value.Value, error) {
	op, err := e.operatorArgument(args[0], 1)
	if err != nil {
		return value.Value{}, err
	}
	if literal := bagOperatorLiteral(op); literal != nil {
		v, err := e.eval(literal)
		if err != nil {
			return value.Value{}, err
		}
		return value.Value{}, bagError(fmt.Sprintf("The first argument of BagOfAll should be an operator, but instead it is:\n%s", v))
	}
	bag, err := e.eval(args[1])
	if err != nil {
		return value.Value{}, err
	}
	pairs, err := bag.Pairs()
	if err != nil {
		return value.Value{}, bagError(fmt.Sprintf("The second argument of BagOfAll should be a function with a finite domain, but instead it is:\n%s", bag))
	}
	mapped := make([][]value.Pair, 0, len(pairs))
	for _, pair := range pairs {
		key, err := e.invokeOperator(op, []value.Value{pair.Key})
		if err != nil {
			return value.Value{}, err
		}
		mapped = append(mapped, []value.Pair{{Key: key, Value: pair.Value}})
	}
	return mergeBags(mapped, "BagOfAll", methodBound{e.program, bagOfAllSignature})
}

func bagOperatorLiteral(op operatorRef) *parser.Expression {
	var body *parser.Expression
	if op.definition != nil {
		body = op.definition.Body
	} else if op.lambda != nil && len(op.lambda.Args) == 1 {
		body = op.lambda.Args[0]
	}
	if body != nil && (body.Kind == "Number" || body.Kind == "String") {
		return body
	}
	return nil
}

func subBags(bag value.Value) (value.Value, error) {
	pairs, err := bagFunction("SubBag", bag)
	if err != nil {
		return value.Value{}, err
	}
	if err := validateSubBag(bag, pairs); err != nil {
		return value.Value{}, err
	}
	sets := [][]value.Pair{{}}
	for _, pair := range pairs {
		sets, err = appendSubBags(sets, pair)
		if err != nil {
			return value.Value{}, err
		}
	}
	result := make([]value.Value, len(sets))
	for i, pairs := range sets {
		result[i], err = value.Function(pairs)
		if err != nil {
			return value.Value{}, err
		}
	}
	return value.Set(result)
}

func validateSubBag(bag value.Value, pairs []value.Pair) error {
	for _, pair := range pairs {
		count, ok := pair.Value.Int()
		if !ok {
			return bagError(fmt.Sprintf("The first argument of >= should be an integer, but instead it is:\n%s", pair.Value))
		}
		if count <= 0 {
			return bagError(fmt.Sprintf("Attempted to apply a function to %s, which is not in its domain", bag))
		}
	}
	return nil
}

func appendSubBags(existing [][]value.Pair, pair value.Pair) ([][]value.Pair, error) {
	count, _ := pair.Value.Int()
	result := make([][]value.Pair, 0, len(existing)*(int(count)+1))
	for _, entries := range existing {
		result = append(result, entries)
		for n := int64(1); n <= int64(count); n++ {
			v, _ := value.Integer(n)
			next := append(append([]value.Pair(nil), entries...), value.Pair{Key: pair.Key, Value: v})
			result = append(result, next)
		}
	}
	return result, nil
}
