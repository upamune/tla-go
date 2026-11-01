package evaluator

import (
	"math"
	"testing"
)

func TestDivisionModuleBoundaries(t *testing.T) {
	for _, tc := range []struct{ a, b, want int64 }{
		{-1, 2, 0}, {-3, 2, -2}, {math.MinInt32, -1, math.MinInt32},
	} {
		got, err := naturalDivide(tc.a, tc.b)
		n, ok := got.Int()
		if err != nil || !ok || int64(n) != tc.want {
			t.Fatalf("Naturals %d div %d = %v, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	got, err := divide(-1, 2)
	n, ok := got.Int()
	if err != nil || !ok || n != -1 {
		t.Fatalf("Integers -1 div 2 = %v, %v", got, err)
	}
	if _, err := divide(math.MinInt32, -1); err == nil {
		t.Fatal("Integers division overflow accepted")
	}
}

func TestPowerBoundaries(t *testing.T) {
	for _, tc := range []struct{ a, b, want int64 }{
		{-2, 31, math.MinInt32}, {0, 1, 0}, {1, math.MaxInt32, 1}, {-1, math.MaxInt32, -1},
	} {
		got, err := power(tc.a, tc.b)
		n, ok := got.Int()
		if err != nil || !ok || int64(n) != tc.want {
			t.Fatalf("%d ^ %d = %v, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
}
