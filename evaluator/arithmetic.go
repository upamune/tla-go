package evaluator

import (
	"fmt"
	"math"

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
		return checkedBinaryArithmetic(a+b, a, b, "+")
	case "-":
		return checkedBinaryArithmetic(a-b, a, b, "-")
	case "*":
		return checkedBinaryArithmetic(a*b, a, b, "*")
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
		return value.Value{}, &RuntimeError{Message: "The second argument of \\div is 0.", Legacy: "division by zero", Standard: true, ToolCode: 2179, TrackSource: true}
	}
	if a == math.MinInt32 && b == -1 {
		return value.Value{}, arithmeticOverflow("-2147483648 \\div -1")
	}
	n := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		n--
	}
	return value.Integer(n)
}

// Naturals.Divide uses Java int division and tests the quotient's sign.
// Integers.Divide instead checks the operands and detects the MIN_VALUE case.
func naturalDivide(a, b int64) (value.Value, error) {
	if b == 0 {
		return divide(a, b)
	}
	q := int64(int32(a / b))
	if q < 0 && int32(q*b) != int32(a) {
		q--
	}
	return value.Integer(q)
}

func modulo(a, b int64) (value.Value, error) {
	if b <= 0 {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The second argument of %% should be a positive number, but instead it is:\n%d", b), Legacy: "modulo requires a positive divisor", Standard: true, ToolCode: 2169, TrackSource: true}
	}
	return value.Integer((a%b + b) % b)
}

func checkedBinaryArithmetic(n, a, b int64, operator string) (value.Value, error) {
	v, err := value.Integer(n)
	if err != nil {
		return value.Value{}, arithmeticOverflow(fmt.Sprintf("%d%s%d", a, operator, b))
	}
	return v, nil
}

func arithmeticOverflow(expression string) error {
	return &RuntimeError{Message: "Overflow when computing " + expression, Standard: true, ToolCode: 2178, TrackSource: true}
}

func power(a, b int64) (value.Value, error) {
	if b < 0 {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The second argument of ^ should be a natural number, but instead it is:\n%d", b), Standard: true, ToolCode: 2169, TrackSource: true}
	}
	if a == 0 && b == 0 {
		return value.Value{}, &RuntimeError{Message: "0^0 is undefined.", Standard: true, ToolCode: 2180, TrackSource: true}
	}
	base, exponent := a, b
	result := int64(1)
	for b > 0 {
		var err error
		result, a, b, err = powerStep(result, a, b, base, exponent)
		if err != nil {
			return value.Value{}, err
		}
	}
	return value.Integer(result)
}

func powerStep(result, a, b, base, exponent int64) (int64, int64, int64, error) {
	var err error
	if b&1 != 0 {
		result, err = powerProduct(result, a, base, exponent)
		if err != nil {
			return 0, 0, 0, err
		}
	}
	b >>= 1
	if b > 0 {
		a, err = powerProduct(a, a, base, exponent)
	}
	return result, a, b, err
}

func powerProduct(a, b, base, exponent int64) (int64, error) {
	result := a * b
	if result < math.MinInt32 || result > math.MaxInt32 {
		return 0, arithmeticOverflow(fmt.Sprintf("%d^%d", base, exponent))
	}
	return result, nil
}
