package evaluator

import "testing"

func TestJSONIntegerTruncationAndWrapping(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int32
	}{
		{"1.9", 1},
		{"-1.9", -1},
		{"3e9", -1294967296},
		{"18446744073709551615", -1},
		{"-4294967297", -1},
		{"1e100000000000000000000", 0},
		{"1e-100000000000000000000", 0},
		{"-0.00012e3", 0},
		{"4294967298.9", 2},
	} {
		v, err := jsonInteger(tc.input)
		got, ok := v.Int()
		if err != nil || !ok || got != tc.want {
			t.Errorf("%s: got %d, %v; want %d", tc.input, got, err, tc.want)
		}
	}
}
