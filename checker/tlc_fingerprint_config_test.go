package checker

import (
	"context"
	"testing"
)

func TestTLCConfigReportsFingerprintPolynomial(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS TLC
VARIABLE x
Init == x = 0
Next == /\ UNCHANGED x /\ PrintT(TLCGet("config").fingerprint)
====
`
	for _, tc := range []struct {
		index int
		want  string
	}{
		{0, `"-7992595565342442794"`},
		{130, `"-7929249073132661339"`},
	} {
		var output []string
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{
			Fingerprint: tc.index,
			OnPrint:     func(line string) { output = append(output, line) },
		})
		if err != nil || result.Status != Success || len(output) != 1 || output[0] != tc.want {
			t.Errorf("fingerprint %d: result %+v, output %q, error %v; want %s", tc.index, result, output, err, tc.want)
		}
	}
}
