package value

import "testing"

func TestStableFingerprintAvoidsDeferredValues(t *testing.T) {
	deferred, err := Subsets(Interval(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		v    Value
		want bool
	}{
		{"number", number(t, 1), true},
		{"set", set(t, number(t, 1), number(t, 2)), true},
		{"function", function(t, Pair{number(t, 1), Bool(true)}), true},
		{"interval", Interval(1, 2_000_000_000), false},
		{"deferred", deferred, false},
		{"nested", function(t, Pair{number(t, 1), deferred}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v.StableFingerprint(); got != tc.want {
				t.Fatalf("got %v; want %v", got, tc.want)
			}
		})
	}
}
