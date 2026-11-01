package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProfilesProcess(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tla-go")
	if output, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = IF x < 100 THEN x + 1 ELSE 0\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) { return exec.Command(bin, args...).CombinedOutput() }
	plain, err := run(model)
	if err != nil {
		t.Fatalf("plain: %v %s", err, plain)
	}
	tr, cpu, heap := filepath.Join(dir, "trace.out"), filepath.Join(dir, "cpu.out"), filepath.Join(dir, "heap.out")
	output, err := run("-debug-trace", tr, "-debug-cpu-profile", cpu, "-debug-heap-profile", heap, model)
	if err != nil {
		t.Fatalf("profiled: %v %s", err, output)
	}
	if !bytes.Equal(plain, output) {
		t.Fatalf("model output differs:\n%s\n%s", plain, output)
	}
	for _, path := range []string{cpu, heap} {
		if b, err := exec.Command("go", "tool", "pprof", "-top", bin, path).CombinedOutput(); err != nil {
			t.Fatalf("pprof %s: %v %s", path, err, b)
		}
	}
	traceOutput, err := exec.Command("go", "tool", "trace", "-d=parsed", tr).CombinedOutput()
	if err != nil {
		t.Fatalf("trace: %v %s", err, traceOutput)
	}
	for _, phase := range []string{"parsing and config", "model initialization", "exploration and liveness"} {
		if !bytes.Contains(traceOutput, []byte(phase)) {
			t.Errorf("missing trace phase %q", phase)
		}
	}
	if output, err := run("-debug-trace", model, model); err == nil {
		t.Fatalf("overwrote input: %s", output)
	}
	if b, _ := os.ReadFile(model); !bytes.Contains(b, []byte("MODULE M")) {
		t.Fatal("model changed")
	}
	same := filepath.Join(dir, "same.out")
	if output, err := run("-debug-trace", same, "-debug-cpu-profile", same, model); err == nil {
		t.Fatalf("duplicate path accepted: %s", output)
	}
	if _, err := os.Stat(same); !os.IsNotExist(err) {
		t.Fatalf("startup file not removed: %v", err)
	}
	for _, args := range [][]string{{"-debug-trace"}, {"-debug-cpu-profile", ""}, {"-debug-heap-profile", filepath.Join(dir, "missing", "x"), model}} {
		if output, err := run(args...); err == nil {
			t.Errorf("invalid options accepted %v: %s", args, output)
		}
	}
	violationConfig := filepath.Join(dir, "violation.cfg")
	if err := os.WriteFile(violationConfig, []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x < 2\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plainFailure, plainErr := run("-config", violationConfig, model)
	profiledFailure, profileErr := run("-debug-heap-profile", filepath.Join(dir, "violation.pprof"), "-config", violationConfig, model)
	if plainErr == nil || profileErr == nil || plainErr.Error() != profileErr.Error() || !bytes.Equal(plainFailure, profiledFailure) {
		t.Fatalf("violation differs: %v %q / %v %q", plainErr, plainFailure, profileErr, profiledFailure)
	}
	earlyHeap := filepath.Join(dir, "early.pprof")
	if _, err := run("-debug-heap-profile", earlyHeap, filepath.Join(dir, "missing.tla")); err == nil {
		t.Fatal("missing model accepted")
	}
	if b, err := exec.Command("go", "tool", "pprof", "-top", bin, earlyHeap).CombinedOutput(); err != nil {
		t.Fatalf("early heap: %v %s", err, b)
	}
	failed := filepath.Join(dir, "failed.trace")
	if _, err := run("-debug-trace", failed, filepath.Join(dir, "missing.tla")); err == nil {
		t.Fatal("missing model accepted")
	}
	if output, err := exec.Command("go", "tool", "trace", "-d=parsed", failed).CombinedOutput(); err != nil || !strings.Contains(string(output), "parsing and config") {
		t.Fatalf("failure trace: %v %s", err, output)
	}
	if runtime.GOOS != "windows" {
		slow := filepath.Join(dir, "Slow.tla")
		if err := os.WriteFile(slow, []byte("---- MODULE Slow ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Slow.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		canceled := filepath.Join(dir, "cancel.trace")
		cancelHeap := filepath.Join(dir, "cancel.pprof")
		cmd := exec.Command(bin, "-debug-trace", canceled, "-debug-heap-profile", cancelHeap, slow)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if st, err := os.Stat(canceled); err == nil && st.Size() > 0 {
				break
			}
			if time.Now().After(deadline) {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatal("trace did not start")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Fatal("interrupt did not stop process")
		}
		if output, err := exec.Command("go", "tool", "pprof", "-top", bin, cancelHeap).CombinedOutput(); err != nil {
			t.Fatalf("interrupted heap: %v %s", err, output)
		}
		if output, err := exec.Command("go", "tool", "trace", "-d=parsed", canceled).CombinedOutput(); err != nil {
			t.Fatalf("interrupted trace: %v %s", err, output)
		}
	}
}

func TestProfileOutputSafety(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tla-go")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) error {
		c := exec.Command(bin, append(args, model)...)
		c.Dir = dir
		return c.Run()
	}
	states := filepath.Join(dir, "states")
	if err := os.Mkdir(states, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(states, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-debug-trace", "same", "-userFile", "./same"},
		{"-debug-trace", "dump.dump", "-dump", "dump"},
		{"-cleanup", "-debug-trace", "states/trace"},
		{"-metadir", "states", "-debug-trace", "states/trace"},
		{"-cleanup", "-metadir", "states", "-debug-trace", "outside", "-dump", "${metadir}/../../outside"},
	} {
		if err := run(args...); err == nil {
			t.Errorf("accepted %v", args)
		}
		if b, err := os.ReadFile(sentinel); err != nil || string(b) != "keep" {
			t.Fatalf("cleanup occurred: %q %v", b, err)
		}
	}
	if err := os.Symlink("target", filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := run("-debug-trace", "target", "-userFile", "alias"); err == nil {
		t.Error("dangling symlink collision accepted")
	}
	existing := filepath.Join(dir, "existing")
	if err := os.WriteFile(existing, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(existing, filepath.Join(dir, "hardlink")); err != nil {
		t.Fatal(err)
	}
	if err := run("-debug-trace", "existing", "-userFile", "hardlink"); err == nil {
		t.Error("hardlink collision accepted")
	}
	if b, _ := os.ReadFile(existing); string(b) != "original" {
		t.Fatalf("file truncated: %q", b)
	}
	if err := run("-cleanup", "-debug-trace", "safe.trace"); err != nil {
		t.Fatalf("safe cleanup: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove sentinel: %v", err)
	}
	if out, err := exec.Command("go", "tool", "trace", "-d=parsed", filepath.Join(dir, "safe.trace")).CombinedOutput(); err != nil {
		t.Fatalf("trace: %v %s", err, out)
	}
}

func TestProfileDynamicWriterProtection(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "trace")
	if err := os.WriteFile(profile, []byte("profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Link(profile, alias); err != nil {
		t.Fatal(err)
	}
	opts := options{tracePath: profile}
	if err := protectProfileOutput(opts, alias); err == nil {
		t.Fatal("hardlink writer accepted")
	}
	if err := simulationTraceFile(filepath.Join(dir, "run"), traceStyle{}, opts)(0, nil); err != nil {
		t.Fatal(err)
	}
	opts.tracePath = filepath.Join(dir, "run_0_1")
	if err := os.Symlink(opts.tracePath, filepath.Join(dir, "run_0_2")); err != nil {
		t.Fatal(err)
	}
	if err := simulationTraceFile(filepath.Join(dir, "run"), traceStyle{}, opts)(2, nil); err == nil {
		t.Fatal("simulation alias accepted")
	}
	if b, err := os.ReadFile(profile); err != nil || string(b) != "profile" {
		t.Fatalf("profile changed: %q %v", b, err)
	}
}

func TestProfileProtectsJSONWriter(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tla-go")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	profile := filepath.Join(dir, "trace")
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(profile, alias); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Json\nASSUME JsonSerialize(" + strconv.Quote(alias) + ", [a |-> 3])\n====\n"
	if err := os.WriteFile(model, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-debug-trace", profile, model)
	if output, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("conflicts with writer")) {
		t.Fatalf("JSON write was not rejected: %v %s", err, output)
	}
	if output, err := exec.Command("go", "tool", "trace", "-d=parsed", profile).CombinedOutput(); err != nil {
		t.Fatalf("trace corrupted: %v %s", err, output)
	}
}

// TestHeapProfileRetainsSearch records every allocation in test-only binaries.
// The focused inuse_space must contain a substantial retained search graph;
// a small graph and a capture deferred until shutdown are negative controls.
func TestHeapProfileRetainsSearch(t *testing.T) {
	dir := t.TempDir()
	source, err := os.ReadFile("profiles.go")
	if err != nil {
		t.Fatal(err)
	}
	build := func(name string, late bool) string {
		t.Helper()
		contents := string(source) + "\nfunc init() { runtime.MemProfileRate = 1 }\n"
		if late {
			old := "return heapcapture.With(ctx, p.captureHeap)"
			if strings.Count(contents, old) != 1 {
				t.Fatal("heap capture hook changed")
			}
			contents = strings.Replace(contents, old, "return heapcapture.With(ctx, func() {})", 1)
		}
		replacement := filepath.Join(dir, name+".go")
		if err := os.WriteFile(replacement, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		original, err := filepath.Abs("profiles.go")
		if err != nil {
			t.Fatal(err)
		}
		overlay, err := json.Marshal(struct{ Replace map[string]string }{map[string]string{original: replacement}})
		if err != nil {
			t.Fatal(err)
		}
		overlayPath := filepath.Join(dir, name+".json")
		if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, name)
		if out, err := exec.Command("go", "build", "-overlay", overlayPath, "-o", bin, ".").CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		return bin
	}
	bin, lateBin := build("live", false), build("late", true)
	model := filepath.Join(dir, "Graph.tla")
	if err := os.WriteFile(model, []byte("---- MODULE Graph ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = IF x < 20000 THEN x + 1 ELSE 0\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Graph.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "Small.tla")
	if err := os.WriteFile(second, []byte("---- MODULE Small ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Small.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := func(name, binary string, models ...string) int64 {
		t.Helper()
		heap := filepath.Join(dir, name+".pprof")
		args := append([]string{"-debug-heap-profile", heap}, models...)
		if out, err := exec.Command(binary, args...).CombinedOutput(); err != nil {
			t.Fatalf("check: %v %s", err, out)
		}
		out, err := exec.Command("go", "tool", "pprof", "-top", "-sample_index=inuse_space", "-focus=checker[.]\\(\\*search\\)[.]add$", "-nodefraction=0", "-unit=bytes", binary, heap).CombinedOutput()
		if err != nil {
			t.Fatalf("pprof: %v %s", err, out)
		}
		match := regexp.MustCompile(`Showing nodes accounting for ([0-9]+)B?`).FindSubmatch(out)
		if match == nil {
			t.Fatalf("cannot read focused inuse_space: %s", out)
		}
		n, err := strconv.ParseInt(string(match[1]), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	const minimumGraphBytes = 100000
	live := profile("first", bin, model, second)
	small := profile("small", bin, second)
	late := profile("late", lateBin, model, second)
	t.Logf("search.add inuse_space: first=%d small=%d late=%d bytes", live, small, late)
	if live < minimumGraphBytes {
		t.Errorf("first graph missing: %d bytes", live)
	}
	if small >= minimumGraphBytes {
		t.Errorf("small graph unexpectedly passed: %d bytes", small)
	}
	if late >= minimumGraphBytes {
		t.Errorf("late capture unexpectedly passed: %d bytes", late)
	}
}

func TestHeapProfileWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heap")
	p, err := startProfiles(options{heapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.heap.file.Close(); err != nil {
		t.Fatal(err)
	}
	p.captureHeap()
	if err := p.close(); err == nil {
		t.Fatal("write error lost")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("partial heap remains: %v", err)
	}
}
