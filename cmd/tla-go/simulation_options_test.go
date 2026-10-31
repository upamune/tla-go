package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimulationOptions(t *testing.T) {
	for _, tc := range []struct {
		args          []string
		traces, depth int
		seed          int64
	}{
		{[]string{"-simulate", "num=2", "-depth", "0", "-seed", "7", "M.tla"}, 2, 0, 7},
		{[]string{"-depth", "5", "-seed", "-1", "-simulate", "num=0", "M.tla"}, 1, 5, -1},
		{[]string{"-simulate", "num=-2", "-seed", "0", "M.tla"}, 1, 100, 0},
		{[]string{"-simulate", "-depth", "-1", "-seed", "0", "M.tla"}, int(^uint(0) >> 1), int(^uint(0) >> 1), 0},
	} {
		opts, err := parseOptions(tc.args)
		if err != nil || !opts.simulate || opts.simulationTraces != tc.traces || opts.simulationDepth != tc.depth || opts.seed != tc.seed {
			t.Errorf("parse %q: %+v, %v", tc.args, opts, err)
		}
	}
	for _, args := range [][]string{
		{"-simulate", "num=oops", "M.tla"},
		{"-simulate", "file=trace", "M.tla"},
		{"-simulate", "num=1,file=trace", "M.tla"},
		{"-simulate", "-depth", "-2", "M.tla"},
		{"-simulate", "-dump", "dot,actionlabels", "out.dot", "M.tla"},
		{"-simulate", "-debug-liveness", "M.tla"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestSimulationContinueOption(t *testing.T) {
	for _, args := range [][]string{
		{"-continue", "-simulate", "num=1", "-seed", "1", "M.tla"},
		{"-simulate", "num=1", "-continue", "-seed", "1", "M.tla"},
	} {
		opts, err := parseOptions(args)
		if err != nil || !opts.continuation || !opts.simulate {
			t.Fatalf("parse %q: %+v, %v", args, opts, err)
		}
	}
}

func TestModelContinueOption(t *testing.T) {
	opts, err := parseOptions([]string{"-continue", "M.tla"})
	if err != nil || !opts.continuation || opts.simulate {
		t.Fatalf("parse model checking continuation: %+v, %v", opts, err)
	}
}

func TestSimulationZeroDepthAndCount(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	for _, tc := range []struct{ num, depth, progress string }{
		{"2", "0", "1 states generated"}, {"0", "1", "2 states generated"},
	} {
		var out, errout bytes.Buffer
		code := run(context.Background(), []string{"-tool", "-simulate", "num=" + tc.num, "-depth", tc.depth, "-seed", "1", filepath.Join(dir, "M.tla")}, &out, &errout)
		if code != 0 || !strings.Contains(out.String(), tc.progress) {
			t.Fatalf("exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
		}
	}
}

func TestSimulationRejectsUnsupportedRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == /\\ UNCHANGED x /\\ TLCGet(\"config\").mode = \"simulate\"", "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-simulate", "num=1", "-seed", "1", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code == 0 || !strings.Contains(out.String(), "TLCGet(config) in simulation is not yet supported") {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
	}
}

func TestSimulationHumanOutput(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-simulate", "num=1", "-seed", "1", "-depth", "1", filepath.Join(dir, "M.tla")}, &out, &errout)
	for _, want := range []string{"Running Random Simulation with seed 1", "Computed 1 initial states", "2 states generated"} {
		if code != 0 || !strings.Contains(out.String(), want) {
			t.Fatalf("exit %d; missing %q; stdout %s; stderr %s", code, want, out.String(), errout.String())
		}
	}
}
