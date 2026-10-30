package evaluator

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/value"
)

func compareJSONKeys(a, b jsonMember) int {
	ai, aok := jsonArrayIndex(a.key)
	bi, bok := jsonArrayIndex(b.key)
	if aok && bok {
		return int(int64(ai) - int64(bi))
	}
	if aok {
		return -1
	}
	if bok {
		return 1
	}
	return 0
}

func jsonArrayIndex(text string) (uint32, bool) {
	if text == "" || len(text) > 1 && text[0] == '0' {
		return 0, false
	}
	n, err := strconv.ParseUint(text, 10, 32)
	return uint32(n), err == nil && n != math.MaxUint32 && strconv.FormatUint(n, 10) == text
}

func jsonInteger(text string) (value.Value, error) {
	text, exponent, negative, ok := jsonNumberParts(text)
	if !ok {
		return value.Integer(0)
	}
	digits, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return value.Integer(0)
	}
	if exponent != 0 {
		jsonShift(digits, exponent)
	}
	if negative {
		digits.Neg(digits)
	}
	digits.Mod(digits, new(big.Int).Lsh(big.NewInt(1), 32))
	return value.Integer(int64(int32(digits.Uint64())))
}

func jsonNumberParts(text string) (string, int64, bool, bool) {
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	exponent := int64(0)
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		var err error
		exponent, err = strconv.ParseInt(text[i+1:], 10, 64)
		if err != nil {
			return "", 0, false, false
		}
		text = text[:i]
	}
	if exponent < -int64(len(text)) {
		return "", 0, false, false
	}
	if i := strings.IndexByte(text, '.'); i >= 0 {
		exponent -= int64(len(text) - i - 1)
		text = text[:i] + text[i+1:]
	}
	if exponent >= 32 || exponent < -int64(len(text)) {
		return "", 0, false, false
	}
	return text, exponent, negative, true
}

func jsonShift(digits *big.Int, exponent int64) {
	if exponent < 0 {
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(-exponent), nil)
		digits.Quo(digits, power)
		return
	}
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(exponent), nil)
	digits.Mul(digits, power)
}
