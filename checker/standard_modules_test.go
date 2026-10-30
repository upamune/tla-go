package checker

import (
	"context"
	"errors"
	"io/fs"
	"testing"
)

func TestEmbeddedStandardModulesWithoutLoader(t *testing.T) {
	for _, tc := range []struct {
		name, source, config string
		states               int
	}{
		{"Reals", "---- MODULE M ----\nEXTENDS Reals\nASSUME Real = \"Reals\"\n====\n", "", 0},
		{"RealTime", "---- MODULE M ----\nEXTENDS RealTime\nInit == now = 0\nNext == now' = 1 - now\n====\n", "INIT Init NEXT Next", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Check(tc.source, tc.config)
			if err != nil || result.Status != Success || result.DistinctStates != tc.states {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
}

func TestEmbeddedStandardModuleLoaderErrors(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Reals\nASSUME Real = \"Reals\"\n====\n"
	loader := func(_ context.Context, _ string) (string, error) { return "", fs.ErrPermission }
	_, err := CheckWithLoader(context.Background(), source, "", loader)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("got %v; want the loader error", err)
	}
	loader = func(_ context.Context, _ string) (string, error) { return "", fs.ErrNotExist }
	result, err := CheckWithLoader(context.Background(), source, "", loader)
	if err != nil || result.Status != Success {
		t.Fatalf("embedded fallback: %+v, %v", result, err)
	}
	loader = func(_ context.Context, name string) (string, error) {
		if name == "Reals" {
			return "---- MODULE Reals ----\nReal == \"local\"\n====", nil
		}
		return "", fs.ErrNotExist
	}
	result, err = CheckWithLoader(context.Background(), source, "", loader)
	if err != nil || result.Status != AssumptionViolation {
		t.Fatalf("local source must take precedence: %+v, %v", result, err)
	}
}
