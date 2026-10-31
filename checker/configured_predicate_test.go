package checker

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestConfiguredConstantPredicate(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS TLC
VARIABLE x
ASSUME PrintT("assume")
Init == /\ PrintT("init") /\ x=0
Next == x'=x
Good == /\ PrintT("good") /\ TRUE
Bad == /\ PrintT("bad") /\ FALSE
====`
	for _, tc := range []struct {
		name, cfg string
		want      []string
		failure   bool
	}{
		{"true", "INVARIANT Good PROPERTY Good CONSTRAINT Good ACTION_CONSTRAINT Good", []string{`"good"`, `"bad"`, `"assume"`, `"init"`}, false},
		{"false", "INVARIANT Bad PROPERTY Good", []string{`"good"`, `"bad"`}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next "+tc.cfg, Options{OnPrint: func(line string) { events = append(events, line) }})
			var configErr *ConfigurationError
			if tc.failure {
				if !errors.As(err, &configErr) || configErr.Code != 2230 {
					t.Fatalf("result %+v error %v; want configuration error 2230", result, err)
				}
			} else if err != nil || result.Status != Success {
				t.Fatalf("result %+v error %v", result, err)
			}
			if !reflect.DeepEqual(events, tc.want) {
				t.Fatalf("events %q; want %q", events, tc.want)
			}
		})
	}
}
