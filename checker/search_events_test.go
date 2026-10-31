package checker

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSearchInitialEvents(t *testing.T) {
	cases := []struct {
		name, body, cfg string
		want            []SearchEvent
	}{
		{"duplicate initial", "Init == (x=0) \\/ (x=0)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", []SearchEvent{{Kind: InitialStart}, {Kind: InitialComplete, GeneratedStates: 2, DistinctStates: 1}}},
		{"initial failure", "Init == x=0\nNext == UNCHANGED x\nInv == x=1", "INIT Init\nNEXT Next\nINVARIANT Inv\n", []SearchEvent{{Kind: InitialStart}}},
		{"assumption failure", "ASSUME FALSE\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", nil},
		{"no variables", "ASSUME TRUE", "", []SearchEvent{{Kind: InitialStart}, {Kind: InitialComplete}}},
		{"no variables or initial predicate", "Spec == [][TRUE]_<<>>", "SPECIFICATION Spec\n", []SearchEvent{{Kind: InitialStart}, {Kind: InitialComplete}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []SearchEvent
			variables := "VARIABLE x\n"
			if strings.HasPrefix(tc.name, "no variables") {
				variables = ""
			}
			source := "---- MODULE M ----\n" + variables + tc.body + "\n===="
			_, err := CheckWithOptions(context.Background(), source, tc.cfg, Options{OnSearch: func(event SearchEvent) { got = append(got, event) }})
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("events %+v, error %v; want %+v", got, err, tc.want)
			}
		})
	}
}
