package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func arithmetic(name string, left, right value.Value) (value.Value, error) {
	a, err := integer(left)
	if err != nil {
		return value.Value{}, err
	}
	b, err := integer(right)
	if err != nil {
		return value.Value{}, err
	}
	return arithmeticOperator(name, a, b)
}

func arithmeticOperator(name string, a, b int64) (value.Value, error) {
	switch name {
	case "+":
		return value.Integer(a + b)
	case "-":
		return value.Integer(a - b)
	case "*":
		return value.Integer(a * b)
	case "^":
		return power(a, b)
	case "\\div":
		return divide(a, b)
	case "%":
		return modulo(a, b)
	case "..":
		return value.Interval(int32(a), int32(b)), nil
	}
	return comparison(name, a, b)
}

func comparison(name string, a, b int64) (value.Value, error) {
	switch name {
	case "<":
		return value.Bool(a < b), nil
	case "<=":
		return value.Bool(a <= b), nil
	case ">":
		return value.Bool(a > b), nil
	case ">=":
		return value.Bool(a >= b), nil
	}
	return value.Value{}, fmt.Errorf("unsupported binary operator %s", name)
}

func divide(a, b int64) (value.Value, error) {
	if b == 0 {
		return value.Value{}, &RuntimeError{Message: "The second argument of \\div is 0.", Legacy: "division by zero", Standard: true, ToolCode: 2179}
	}
	n := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		n--
	}
	return value.Integer(n)
}

func modulo(a, b int64) (value.Value, error) {
	if b <= 0 {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The second argument of %% should be a positive number, but instead it is:\n%d", b), Legacy: "modulo requires a positive divisor", Standard: true}
	}
	return value.Integer((a%b + b) % b)
}

func multiplyChecked(a, b int64) (int64, error) {
	v, err := value.Integer(a * b)
	if err != nil {
		return 0, err
	}
	n, _ := v.Int()
	return int64(n), nil
}

func power(a, b int64) (value.Value, error) {
	if b < 0 || a == 0 && b == 0 {
		return value.Value{}, fmt.Errorf("undefined exponentiation")
	}
	result := int64(1)
	for b > 0 {
		var err error
		result, a, b, err = powerStep(result, a, b)
		if err != nil {
			return value.Value{}, err
		}
	}
	return value.Integer(result)
}

func powerStep(result, a, b int64) (int64, int64, int64, error) {
	var err error
	if b&1 != 0 {
		result, err = multiplyChecked(result, a)
		if err != nil {
			return 0, 0, 0, err
		}
	}
	b >>= 1
	if b > 0 {
		a, err = multiplyChecked(a, a)
	}
	return result, a, b, err
}
