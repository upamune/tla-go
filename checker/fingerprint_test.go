package checker

import (
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestTLCFingerprintSelection(t *testing.T) {
	cases := []struct {
		body, configuration string
		want                bool
	}{
		{"A == \"ǩ\"", "", true},
		{"A == 0\nASSUME \"ǩ\" = \"ǩ\"", "", false},
		{"A == 0", "CONSTANT a = \"ǩ\"", true},
		{"A == \"ǩ\"", "CONSTANT a = M", false},
	}
	for _, tc := range cases {
		module, err := parser.ParseModule("---- MODULE M ----\n" + tc.body + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Parse(tc.configuration)
		if err != nil {
			t.Fatal(err)
		}
		if got := useTLCFingerprint(module, cfg); got != tc.want {
			t.Errorf("%s / %s: got %v, want %v", tc.body, tc.configuration, got, tc.want)
		}
	}
}
