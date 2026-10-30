package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func bagBinary(name string, left, right value.Value) (value.Value, error) {
	a, b, err := bagBinaryArguments(name, left, right)
	if err != nil {
		return value.Value{}, err
	}
	switch name {
	case "\\oplus":
		return mergeBags([][]value.Pair{a, b}, "BagCup")
	case "\\ominus":
		return subtractBags(a, b, false)
	default:
		return subtractBags(a, b, true)
	}
}

func bagBinaryArguments(name string, left, right value.Value) ([]value.Pair, []value.Pair, error) {
	if name == "\\sqsubseteq" {
		a, err := bagFunction(name, left)
		if err != nil {
			return nil, nil, err
		}
		b, err := bagFunction(name, right)
		return a, b, err
	}
	display := map[string]string{"\\oplus": "(+)", "\\ominus": "(-)"}[name]
	a, err := bagArgument(display, "first", left, name == "\\oplus")
	if err != nil {
		return nil, nil, err
	}
	b, err := bagArgument(display, "second", right, name == "\\oplus")
	return a, b, err
}

func bagArgument(name, position string, v value.Value, validate bool) ([]value.Pair, error) {
	pairs, err := v.Pairs()
	if err != nil || (validate && !validBag(pairs)) {
		return nil, bagError(fmt.Sprintf("The %s argument of %s should be a bag, but instead it is:\n%s", position, name, v))
	}
	return pairs, nil
}

func mergeBags(bags [][]value.Pair, method string) (value.Value, error) {
	result := make([]value.Pair, 0)
	indices := map[string]int{}
	for _, bag := range bags {
		for _, pair := range bag {
			if err := mergeBagPair(&result, indices, pair, method); err != nil {
				return value.Value{}, err
			}
		}
	}
	return value.Function(result)
}

func mergeBagPair(result *[]value.Pair, indices map[string]int, pair value.Pair, method string) error {
	key, err := pair.Key.Key()
	if err != nil {
		return err
	}
	index, ok := indices[key]
	if !ok {
		indices[key] = len(*result)
		*result = append(*result, pair)
		return nil
	}
	a, aok := (*result)[index].Value.Int()
	b, bok := pair.Value.Int()
	if !aok {
		return bagJavaCast(method, "tlc2.value.impl.Value", (*result)[index].Value)
	}
	if !bok {
		return bagJavaCast(method, "tlc2.value.impl.Value", pair.Value)
	}
	sum, err := value.Integer(int64(a) + int64(b))
	if err != nil {
		return bagError(fmt.Sprintf("Overflow when computing %d+%d", a, b))
	}
	(*result)[index].Value = sum
	return nil
}

func subtractBags(left, right []value.Pair, subset bool) (value.Value, error) {
	result := make([]value.Pair, 0, len(left))
	for _, pair := range left {
		count, err := subtractBagPair(pair, right, subset)
		if err != nil {
			return value.Value{}, err
		}
		if count > 0 && subset {
			return value.Bool(false), nil
		}
		if count > 0 {
			v, _ := value.Integer(int64(count))
			result = append(result, value.Pair{Key: pair.Key, Value: v})
		}
	}
	if subset {
		return value.Bool(true), nil
	}
	return value.Function(result)
}

func subtractBagPair(pair value.Pair, right []value.Pair, subset bool) (int32, error) {
	v, err := bagFind(right, pair.Key)
	if err != nil {
		return 0, err
	}
	a, ok := pair.Value.Int()
	b, otherOK := v.Int()
	method, resultClass := "BagDiff", "tlc2.value.impl.Value"
	if subset {
		method, resultClass = "SqSubseteq", "tlc2.value.IBoolValue"
	}
	if !ok {
		return 0, bagJavaCast(method, resultClass, pair.Value)
	}
	if !otherOK {
		return 0, bagJavaCast(method, resultClass, v)
	}
	return int32(uint32(a) - uint32(b)), nil
}

func bagJavaCast(method, resultClass string, v value.Value) error {
	class := "Value"
	switch v.Kind() {
	case value.StringKind:
		class = "StringValue"
	case value.BooleanKind:
		class = "BoolValue"
	case value.ModelKind:
		class = "ModelValue"
	case value.SetKind:
		class = "SetEnumValue"
	case value.FunctionKind:
		class = "FcnRcdValue"
	}
	qualified := "tlc2.value.impl." + class
	args := "tlc2.value.impl.Value,tlc2.value.impl.Value"
	if method == "BagUnion" {
		args = "tlc2.value.impl.Value"
	}
	return bagError(fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static %s tlc2.module.Bags.%s(%s),\nbut it produced the following error:\nclass %s cannot be cast to class tlc2.value.impl.IntValue (%s and tlc2.value.impl.IntValue are in unnamed module of loader 'app')", resultClass, method, args, qualified, qualified))
}
