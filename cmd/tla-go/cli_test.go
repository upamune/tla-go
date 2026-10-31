package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCLIJSONFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "nested", "result.json")
	source := "---- MODULE M ----\nEXTENDS Json\nASSUME JsonSerialize(" + strconv.Quote(output) + ", [a |-> 3])\nASSUME JsonDeserialize(" + strconv.Quote(output) + ") = [a |-> 3]\n===="
	spec := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{spec}, &out, &errs); code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != `{"a":3}` {
		t.Fatalf("output=%q error=%v", got, err)
	}
}

func TestCLISeedIsVisibleToTLC(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS TLC\nASSUME TLCGet(\"config\").seed = \"-42\"\n===="
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"-seed", "-42", spec}, &out, &errs); code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLIRandomSubsetVersion(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC, Randomization\nASSUME PrintT(RandomSubset(3, 1..20))\n===="
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"tlc-1.7.4", []string{"-seed", "1", spec}, "{6, 8, 17}"},
		{"modern", []string{"-seed", "1", "-modern-random-subsets", spec}, "{7, 13, 15}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			code := run(context.Background(), tc.args, &out, &errs)
			if code != 0 || !strings.Contains(out.String(), "\n"+tc.want+"\n") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
			}
		})
	}
}

func TestModelCLI(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "Counter.tla")
	cfg := filepath.Join(dir, "Counter.cfg")
	source := "---- MODULE Counter ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 2 /\\ x' = x + 1\n===="
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("INIT Init NEXT Next"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		args     []string
		code     int
		contains string
	}{{"deadlock", []string{spec}, 11, "Error: Deadlock reached."}, {"disabled", []string{cfg, "-deadlock"}, 0, "Model checking completed."}, {"custom config", []string{"-config", cfg, spec}, 11, "State 3: <Next"}} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			code := run(context.Background(), tt.args, &out, &errs)
			if code != tt.code || !strings.Contains(out.String(), tt.contains) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
			}
		})
	}
}

func TestCLIRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	cfg := filepath.Join(dir, "M.cfg")
	if err := os.WriteFile(spec, []byte("not a module"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("INIT Init NEXT Next"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{spec}, &out, &errs); code != 150 {
		t.Fatalf("expected syntax error, got %d: %s", code, errs.String())
	}
	if err := os.WriteFile(cfg, []byte("BAD"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errs.Reset()
	if code := run(context.Background(), []string{spec}, &out, &errs); code != 255 {
		t.Fatalf("expected config error, got %d: %s", code, errs.String())
	}
	for _, args := range [][]string{{}, {"-config"}, {"-simulate", "-workers", "2", spec}} {
		out.Reset()
		errs.Reset()
		if code := run(context.Background(), args, &out, &errs); code != 255 {
			t.Fatalf("%v: got %d", args, code)
		}
	}
}

func TestCLILivenessViolation(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	cfg := filepath.Join(dir, "M.cfg")
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nP == <>(x = 1)\n===="
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("INIT Init NEXT Next PROPERTY P"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{spec}, &out, &errs)
	if code != 13 || !strings.Contains(out.String(), "State 2: Stuttering") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLILoadsExtendedModule(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla":      "---- MODULE M ----\nEXTENDS Helper\nVARIABLE x\nInit == x = 0\nNext == x' = Step(x)\n====",
		"M.cfg":      "INIT Init NEXT Next",
		"Helper.tla": "---- MODULE Helper ----\nStep(n) == n\n====",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), "1 distinct states found") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestCLILoadsInstanceModule(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla":      "---- MODULE M ----\nVARIABLE x\nI == INSTANCE Helper WITH y <- x\nInit == x = 0\nNext == I!Step\n====",
		"M.cfg":      "INIT Init NEXT Next",
		"Helper.tla": "---- MODULE Helper ----\nVARIABLE y\nStep == y' = y\n====",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errs bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), "1 distinct states found") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
	}
}
