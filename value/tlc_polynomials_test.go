package value

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// The digest is from Java TLC 1.7.4: for each FP64.Polys index,
// IntValue.gen(42).fingerPrint(FP64.New()), printed as 16 hex digits and LF.
func TestTLCFingerprintAllPolynomials(t *testing.T) {
	h := sha256.New()
	integer, err := Integer(42)
	if err != nil {
		t.Fatal(err)
	}
	for index := range TLCFingerprintPolynomialCount {
		algorithm, err := NewTLCFingerprintAlgorithm(index)
		if err != nil {
			t.Fatal(err)
		}
		got, err := algorithm.Fingerprint([]Value{integer}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%016x\n", got)
	}
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != "119fe89ffd4e94282ba9068bfb4b504e6f79d4f71ff8a3ca144f17aae61c5505" {
		t.Fatalf("fingerprints differ from Java TLC: digest %s", got)
	}
	for _, index := range []int{-1, TLCFingerprintPolynomialCount} {
		if _, err := NewTLCFingerprintAlgorithm(index); err == nil {
			t.Errorf("polynomial %d unexpectedly accepted", index)
		}
	}
}

func TestTLCFingerprintPolynomialsConcurrent(t *testing.T) {
	cases := []struct {
		index int
		want  uint64
	}{
		{0, 0xd9f252e31aec0f7a},
		{1, 0x6e615b32ca68c019},
		{63, 0xce2904bee42caa34},
		{130, 0x1fad82e650910a82},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.index), func(t *testing.T) {
			t.Parallel()
			algorithm, err := NewTLCFingerprintAlgorithm(tc.index)
			if err != nil {
				t.Fatal(err)
			}
			integer, _ := Integer(42)
			for range 100 {
				got, err := algorithm.Fingerprint([]Value{integer}, nil, nil)
				if err != nil || got != tc.want {
					t.Fatalf("fingerprint = %x, error = %v; want %x", got, err, tc.want)
				}
			}
		})
	}
}
