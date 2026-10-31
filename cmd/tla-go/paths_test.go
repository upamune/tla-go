package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIMultipleModelsAndGlob(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Alpha.tla":  "---- MODULE Alpha ----\nASSUME FALSE\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====",
		"Alpha.cfg":  "INIT Init NEXT Next",
		"Beta.tla":   "---- MODULE Beta ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1\n====",
		"Beta.cfg":   "INIT Init NEXT Next INVARIANT Inv",
		"Gamma.tla":  "---- MODULE Gamma ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====",
		"Gamma.cfg":  "INIT Init NEXT Next",
		"Helper.tla": "---- MODULE Helper ----\nValue == 1\n====",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "*.tla")}, &out, &errs)
	if code != 10 {
		t.Fatalf("got %d, stdout=%q stderr=%q", code, out.String(), errs.String())
	}
	var checks []string
	for line := range strings.SplitSeq(out.String(), "\n") {
		if strings.HasPrefix(line, "Checking ") {
			checks = append(checks, line)
		}
	}
	want := []string{"Checking Alpha.tla with Alpha.cfg", "Checking Beta.tla with Beta.cfg", "Checking Gamma.tla with Gamma.cfg"}
	if strings.Join(checks, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q", checks)
	}
}

func TestCLIMultipleModelsRejectsSingleConfig(t *testing.T) {
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{"A.tla", "B.tla", "-config", "A.cfg"}, &out, &errs)
	if code != 1 || errs.String() != "Error: -config applies to one specification, but several were given.\n" {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLIModuleNameMatchesFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Renamed.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Other ----\n===="), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Renamed.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{path}, &out, &errs)
	want := "TlaSyntaxError: File name 'Renamed' does not match the name 'Other' of the top level module it contains.\n"
	if code != 150 || errs.String() != want {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLIDeduplicatesSpecAndConfigPaths(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "One.tla")
	cfg := filepath.Join(dir, "One.cfg")
	if err := os.WriteFile(spec, []byte("---- MODULE One ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("INIT Init NEXT Next"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{spec, cfg, spec}, &out, &errs)
	if code != 0 || strings.Count(out.String(), "Checking One.tla with One.cfg") != 1 {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

// The input files are unchanged fixtures from tla-ts commit 19c763f07fda4b098e8a24405301628a424547c0.
func TestCLIUpstreamFixtures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     int
		contains string
	}{
		{"AssumptionFalse", 10, "Error: Assumption line 4, col 20 to line 4, col 24 of module AssumptionFalse is false."},
		{"InitialInvariant", 12, "\nx = 0\n"},
		{"Success", 0, "10 states generated, 6 distinct states found, 0 states left on queue.\nThe depth of the complete state graph search is 3."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", "cli-output", tc.name+".cfg")
			var out, errs bytes.Buffer
			code := run(context.Background(), []string{path}, &out, &errs)
			if code != tc.code || !strings.Contains(out.String(), tc.contains) {
				t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
			}
			checking := "Checking " + tc.name + ".tla with " + tc.name + ".cfg\n"
			if !strings.HasPrefix(out.String(), checking) {
				t.Fatalf("got %q", out.String())
			}
		})
	}
}

func TestCLIUpstreamLivenessStrategy(t *testing.T) {
	path := filepath.Join("testdata", "cli-output", "Lasso.tla")
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{path, "-lncheck", "final"}, &out, &errs)
	if code != 13 || !strings.Contains(out.String(), "Back to state 1") {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	code = run(context.Background(), []string{path, "-lncheck"}, &out, &errs)
	if code != 1 || errs.Len() != 0 || out.String() != "Error: Error: expect a strategy such as final for -lncheck option.\nUsage: java tlc2.TLC [-option] inputfile\n" {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLIUpstreamExactReports(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{
		{"AssumptionFalse", 10}, {"InitialInvariant", 12}, {"Success", 0}, {"Lasso", 13}, {"Lassos", 13}, {"Counter", 11}, {"FailedInvariant", 76}, {"SafetyProperty", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", "cli-output")
			want, err := os.ReadFile(filepath.Join(dir, tc.name+".stdout"))
			if err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errs)
			if code != tc.code || out.String() != string(want) || errs.Len() != 0 {
				t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, out.String(), want, errs.String())
			}
		})
	}
}

func TestCLIUpstreamMultipleVariableTrace(t *testing.T) {
	path := filepath.Join("testdata", "cli-output", "WaterJugs.tla")
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{path}, &out, &errs)
	if code != 12 || !strings.Contains(out.String(), "State 1: <Initial predicate>\n/\\ large = 0\n/\\ small = 0\n") {
		t.Fatalf("got %d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
	label := "<PourLargeIntoSmall line 37, col 7 to line 38, col 32 of module WaterJugs>"
	if !strings.Contains(out.String(), label) {
		t.Fatalf("missing action label %q in %q", label, out.String())
	}
}
