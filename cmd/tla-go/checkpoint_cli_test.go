package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckpointCLIPreflightFallback(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data.ndjson")
	if err := os.WriteFile(data, []byte("1\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, source, cfg, marker string }{
		{"substitution", fmt.Sprintf("EXTENDS Json, Sequences\nCONSTANT C\nVARIABLE x\nData == ndJsonDeserialize(%q)\nInit == x = Len(C)\nNext == UNCHANGED x", data), "CONSTANT C <- Data\nINIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE", ""},
		{"effectful unused definition", "EXTENDS TLC\nVARIABLE x\nUnused == PrintT(\"once\")\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE", `"once"`},
		{"empty config", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := t.TempDir()
			spec := filepath.Join(fixture, "M.tla")
			if err := os.WriteFile(spec, []byte("---- MODULE M ----\n"+tc.source+"\n====\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fixture, "M.cfg"), []byte(tc.cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{spec}, &out, &errout)
			if code != 0 {
				t.Fatalf("normal: %d %s %s", code, &out, &errout)
			}
			want := out.String()
			if tc.marker != "" && strings.Count(want, tc.marker) != 1 {
				t.Fatalf("normal effect: %s", want)
			}
			// Json is an optional community module, not bundled in TLC 1.7.4.
			if jar := os.Getenv("TLC_JAR"); jar != "" && tc.name != "substitution" {
				javaOut, err := exec.Command("java", "-jar", jar, "-workers", "1", spec).CombinedOutput()
				if err != nil || (tc.marker != "" && strings.Count(string(javaOut), tc.marker) != 1) {
					t.Fatalf("Java TLC: %v %s", err, javaOut)
				}
			}
			out.Reset()
			errout.Reset()
			code = run(context.Background(), []string{"-checkpoint", "1", spec}, &out, &errout)
			if code != 0 || out.String() != want || (tc.name != "empty config" && !strings.Contains(errout.String(), "checking normally")) {
				t.Fatalf("checkpoint: %d\nnormal: %s\ncheckpoint: %s\n%s", code, want, &out, &errout)
			}
			out.Reset()
			errout.Reset()
			code = run(context.Background(), []string{"-recover", filepath.Join(fixture, "recovery"), spec}, &out, &errout)
			if code == 0 || strings.Contains(errout.String(), "checking normally") || (tc.name != "empty config" && !strings.Contains(errout.String(), "unsupported checkpoint")) || (tc.marker != "" && strings.Contains(out.String(), tc.marker)) {
				t.Fatalf("recovery: %d %s %s", code, &out, &errout)
			}
		})
	}
}

func TestCheckpointCLIRecovery(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5", "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n")
	spec := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x\u0027 = x + 1 /\\ x < 5\nInv == x < 5\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "nested", "states")
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "2")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-checkpoint", "1", "-metadir", root, spec}, &out, &errout)
	if code != 255 || !strings.Contains(errout.String(), "debug checkpoint saved") {
		t.Fatalf("pause %d %s %s", code, out.String(), errout.String())
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "")
	out.Reset()
	errout.Reset()
	code = run(context.Background(), []string{"-recover", saved, spec}, &out, &errout)
	if code != 12 {
		t.Fatalf("recovery %d %s %s", code, out.String(), errout.String())
	}
	resumed := out.String()
	out.Reset()
	errout.Reset()
	code = run(context.Background(), []string{spec}, &out, &errout)
	if code != 12 || resumed != out.String() {
		t.Fatalf("comparison %d\nresumed: %s\nfull: %s\n%s", code, resumed, out.String(), errout.String())
	}
	if err := os.WriteFile(filepath.Join(saved, "tla-go.chkpt"), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	if code = run(context.Background(), []string{"-recover", saved, spec}, &out, &errout); code == 0 || !strings.Contains(errout.String(), "checkpoint") {
		t.Fatalf("corrupt %d %s %s", code, out.String(), errout.String())
	}
}

func TestCheckpointCLIRecoveryPreparationWarning(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nUnused == [<<0>> EXCEPT ![2] = 0]\nInit == x = 0\nNext == /\\ x' = x + 1 /\\ x < 4\n====\n"
	spec := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		output, err := exec.Command("java", "-jar", jar, "-workers", "1", spec).CombinedOutput()
		if err != nil || !bytes.Contains(output, []byte("Warning: The EXCEPT")) {
			t.Fatalf("Java warning: %v %s", err, output)
		}
	}
	root := filepath.Join(dir, "states")
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "1")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-checkpoint", "1", "-metadir", root, spec}, &out, &errout); code != 255 || !strings.Contains(out.String(), "Warning: The EXCEPT") {
		t.Fatalf("fresh: %d %s %s", code, &out, &errout)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "")
	check := func(path string, wantSuccess bool) {
		t.Helper()
		out.Reset()
		errout.Reset()
		code := run(context.Background(), []string{"-recover", path, spec}, &out, &errout)
		if (code == 0) != wantSuccess || strings.Contains(out.String(), "Warning: The EXCEPT") {
			t.Fatalf("recovery: %d %s %s", code, &out, &errout)
		}
	}
	check(filepath.Join(dir, "missing"), false)
	if err := os.WriteFile(spec, []byte(source+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(saved, false)
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	check(saved, true)
	if err := os.WriteFile(filepath.Join(saved, "tla-go.chkpt"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(saved, false)
}

func TestCheckpointProcessRestart(t *testing.T) {
	dir := t.TempDir()
	// The overlay adds a process-test rendezvous after a successful restore. It
	// keeps the finite search from finishing before the parent sends SIGINT.
	driverPath, err := filepath.Abs(filepath.Join("..", "..", "checker", "checkpoint_driver.go"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := os.ReadFile(driverPath)
	if err != nil {
		t.Fatal(err)
	}
	const boundary = "\treturn s.explore()"
	hook := `	if os.Getenv("TLA_GO_TEST_WAIT_FOR_INTERRUPT") == "1" {
		fmt.Println("test: checkpoint restored")
		<-m.ctx.Done()
	}
` + boundary
	if strings.Count(string(driver), boundary) != 1 {
		t.Fatal("checkpoint restore boundary not found uniquely")
	}
	modified := strings.Replace(string(driver), boundary, hook, 1)
	modified = strings.Replace(modified, `"fmt"`, "\"fmt\"\n\t\"os\"", 1)
	replacement := filepath.Join(dir, "checkpoint_driver.go")
	if err := os.WriteFile(replacement, []byte(modified), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{driverPath: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go")
	build := exec.Command("go", "build", "-overlay", overlayPath, "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	spec := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5\n====\n"
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "nested", "states")
	pause := exec.Command(binary, "-tool", "-checkpoint", "1", "-metadir", root, spec)
	pause.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	paused, err := pause.CombinedOutput()
	if err == nil || !bytes.Contains(paused, []byte("debug checkpoint saved")) {
		t.Fatalf("pause: %v %s", err, paused)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	resume := exec.Command(binary, "-tool", "-recover", saved, spec)
	resumed, err := resume.CombinedOutput()
	if resume.ProcessState.ExitCode() != 12 {
		t.Fatalf("resume: %v %s", err, resumed)
	}
	full := exec.Command(binary, "-tool", spec)
	complete, err := full.CombinedOutput()
	if full.ProcessState.ExitCode() != 12 {
		t.Fatalf("full: %v %s", err, complete)
	}
	for _, msg := range []string{"Invariant Inv is violated", "6 states generated, 6 distinct states found", "6: <Next"} {
		if !bytes.Contains(resumed, []byte(msg)) || !bytes.Contains(complete, []byte(msg)) {
			t.Fatalf("missing %q: %s / %s", msg, resumed, complete)
		}
	}

	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := exec.Command("java", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-metadir", filepath.Join(dir, "java-states"), spec)
		javaOutput, javaErr := java.CombinedOutput()
		if java.ProcessState == nil || java.ProcessState.ExitCode() != 12 {
			t.Fatalf("Java: %v %s", javaErr, javaOutput)
		}
		for _, msg := range []string{"Invariant Inv is violated", "6 states generated, 6 distinct states found", "6: <Next"} {
			if !bytes.Contains(javaOutput, []byte(msg)) || !bytes.Contains(resumed, []byte(msg)) {
				t.Fatalf("Java comparison missing %q: %s / %s", msg, javaOutput, resumed)
			}
		}
	}
	successSource := strings.Replace(source, "Inv == x < 5", "Inv == x <= 5", 1)
	if err := os.WriteFile(spec, []byte(successSource), 0o600); err != nil {
		t.Fatal(err)
	}
	successRoot := filepath.Join(dir, "success")
	successPause := exec.Command(binary, "-checkpoint", "1", "-metadir", successRoot, spec)
	successPause.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	if output, err := successPause.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) {
		t.Fatalf("success pause: %v %s", err, output)
	}
	successEntries, err := os.ReadDir(successRoot)
	if err != nil || len(successEntries) != 1 {
		t.Fatalf("success metadata: %v %v", successEntries, err)
	}
	successResume := exec.Command(binary, "-recover", filepath.Join(successRoot, successEntries[0].Name()), spec)
	successOutput, err := successResume.CombinedOutput()
	if err != nil || !bytes.Contains(successOutput, []byte("6 states generated, 6 distinct states found")) {
		t.Fatalf("success resume: %v %s", err, successOutput)
	}
	successFull := exec.Command(binary, spec)
	fullOutput, err := successFull.CombinedOutput()
	if err != nil || !bytes.Contains(fullOutput, []byte("6 states generated, 6 distinct states found")) {
		t.Fatalf("success full: %v %s", err, fullOutput)
	}
	slowSource := strings.ReplaceAll(successSource, "5", "2000")
	if err := os.WriteFile(spec, []byte(slowSource), 0o600); err != nil {
		t.Fatal(err)
	}
	interruptRoot := filepath.Join(dir, "interrupt")
	save := exec.Command(binary, "-checkpoint", "1", "-metadir", interruptRoot, spec)
	save.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	if output, err := save.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) {
		t.Fatalf("interrupt setup: %v %s", err, output)
	}
	interruptEntries, err := os.ReadDir(interruptRoot)
	if err != nil || len(interruptEntries) != 1 {
		t.Fatalf("interrupt metadata: %v %v", interruptEntries, err)
	}
	interruptDir := filepath.Join(interruptRoot, interruptEntries[0].Name())
	interruptCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	interrupted := exec.CommandContext(interruptCtx, binary, "-recover", interruptDir, spec)
	interrupted.Env = append(os.Environ(), "TLA_GO_TEST_WAIT_FOR_INTERRUPT=1")
	var interruptErr bytes.Buffer
	interrupted.Stderr = &interruptErr
	stdout, err := interrupted.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := interrupted.Start(); err != nil {
		t.Fatal(err)
	}
	// Always reap the process, including when the rendezvous or signal fails.
	t.Cleanup(func() {
		if interrupted.ProcessState == nil {
			_ = interrupted.Process.Kill()
			_ = interrupted.Wait()
		}
	})
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("waiting for restored checkpoint: %v", err)
		}
		if strings.TrimSpace(line) == "test: checkpoint restored" {
			break
		}
	}
	if err := interrupted.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := interrupted.Wait(); err == nil || interruptCtx.Err() != nil || !strings.Contains(interruptErr.String(), "context canceled") {
		t.Fatalf("interrupted recovery: %v, context: %v, stderr: %s", err, interruptCtx.Err(), &interruptErr)
	}
	restarted := exec.Command(binary, "-recover", interruptDir, spec)
	if output, err := restarted.CombinedOutput(); err != nil || !bytes.Contains(output, []byte("2001 states generated, 2001 distinct states found")) {
		t.Fatalf("restart: %v %s", err, output)
	}
	if err := os.WriteFile(spec, []byte(source+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatch := exec.Command(binary, "-recover", saved, spec)
	output, err := mismatch.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("identity mismatch")) {
		t.Fatalf("mismatch: %v %s", err, output)
	}
	missing := exec.Command(binary, "-recover", filepath.Join(dir, "missing"), spec)
	if output, err = missing.CombinedOutput(); err == nil {
		t.Fatalf("missing: %s", output)
	}
}

func TestCheckpointUnsupportedFallsBackAndRecoverRejects(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	spec := filepath.Join(dir, "M.tla")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-checkpoint", "1", "-metadir", filepath.Join(dir, "states"), spec}, &out, &errout); code != 0 || !strings.Contains(errout.String(), "checking normally without checkpoints") {
		t.Fatalf("fallback: %d %s %s", code, out.String(), errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := run(context.Background(), []string{"-recover", filepath.Join(dir, "missing"), spec}, &out, &errout); code == 0 {
		t.Fatalf("unsupported recovery: %d %s %s", code, out.String(), errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := run(context.Background(), []string{"-recover", "", spec}, &out, &errout); code == 0 {
		t.Fatalf("empty recovery accepted: %s", out.String())
	}
}

func TestCheckpointLibraryRootProcess(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "tla-go")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	lib := filepath.Join(dir, "lib")
	if err := os.Mkdir(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\n====\n"
	if err := os.WriteFile(filepath.Join(lib, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	// The named config is resolved in the library; -deadlock modifies the effective config.
	if err := os.WriteFile(filepath.Join(lib, "Custom.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-DTLA-Library=" + lib, "-config", "Custom", "-deadlock"}
	runProcess := func(extra ...string) ([]byte, error) {
		cmd := exec.Command(binary, append(append(append([]string{}, args...), extra...), "M.tla")...)
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}
	full, err := runProcess()
	if err != nil || !bytes.Contains(full, []byte("6 states generated, 6 distinct states found")) {
		t.Fatalf("normal: %v %s", err, full)
	}
	root := filepath.Join(dir, "states")
	cmd := exec.Command(binary, append(append(append([]string{}, args...), "-checkpoint", "1", "-metadir", root), "M.tla")...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	if output, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) {
		t.Fatalf("save: %v %s", err, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	recovered, err := runProcess("-recover", filepath.Join(root, entries[0].Name()))
	if err != nil || !bytes.Contains(recovered, []byte("6 states generated, 6 distinct states found")) {
		t.Fatalf("recover: %v %s", err, recovered)
	}
	// A CWD module must take precedence over the library for both paths.
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(strings.ReplaceAll(source, "x < 5", "x < 2")), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runProcess("-recover", filepath.Join(root, entries[0].Name())); err == nil || !bytes.Contains(output, []byte("identity mismatch")) {
		t.Fatalf("cwd precedence: %v %s", err, output)
	}
}

func TestCheckpointSameSecondProcesses(t *testing.T) {
	dir := t.TempDir()
	// Freeze only the allocation clock via a build overlay; all runs execute the real CLI.
	original, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	frozen := strings.Replace(string(original), "makeMetaDir(root, time.Now())", "makeMetaDir(root, time.Unix(1700000000, 0))", 1)
	if frozen == string(original) {
		t.Fatal("allocation clock not found")
	}
	replacement := filepath.Join(dir, "cli.go")
	if err := os.WriteFile(replacement, []byte(frozen), 0o600); err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{absolute: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlayFile := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go")
	if output, err := exec.Command("go", "build", "-overlay", overlayFile, "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	root := filepath.Join(dir, "states")
	spec := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const count = 4
	cmds := make([]*exec.Cmd, count)
	outputs := make([]bytes.Buffer, count)
	for i := range cmds {
		cmds[i] = exec.Command(binary, "-checkpoint", "1", "-metadir", root, spec)
		cmds[i].Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
		cmds[i].Stdout, cmds[i].Stderr = &outputs[i], &outputs[i]
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err == nil || !strings.Contains(outputs[i].String(), "debug checkpoint saved") {
			t.Fatalf("run %d: %v %s", i, err, &outputs[i])
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != count {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "tla-go.chkpt")); err != nil {
			t.Fatal(err)
		}
	}
	// A successful consecutive run cleans up only its own metadata.
	done := exec.Command(binary, "-checkpoint", "1", "-metadir", root, spec)
	if output, err := done.CombinedOutput(); err != nil {
		t.Fatalf("complete: %v %s", err, output)
	}
	after, err := os.ReadDir(root)
	if err != nil || len(after) != count {
		t.Fatalf("other metadata removed: %v %v", after, err)
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "tla-go.chkpt")); err != nil {
			t.Fatal(err)
		}
	}

	// The implicit states root follows the same collision and ownership rules.
	defaultRun := exec.Command(binary, "-checkpoint", "1", "M.tla")
	defaultRun.Dir = dir
	defaultRun.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	if output, err := defaultRun.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) || bytes.Contains(output, []byte("checking normally without checkpoints")) {
		t.Fatalf("default root: %v %s", err, output)
	}
	finalEntries, err := os.ReadDir(root)
	if err != nil || len(finalEntries) != count+1 {
		t.Fatalf("default metadata: %v %v", finalEntries, err)
	}
}

func TestCheckpointRecoveryScheduleProcess(t *testing.T) {
	dir := t.TempDir()
	driverPath, err := filepath.Abs(filepath.Join("..", "..", "checker", "checkpoint_driver.go"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := os.ReadFile(driverPath)
	if err != nil {
		t.Fatal(err)
	}
	// Advance only the scheduler's clock past every tested interval. This makes
	// elapsed-time writes deterministic without altering checkpoint contents.
	old := "return !time.Now().Before(d.next)"
	if strings.Count(string(driver), old) != 1 {
		t.Fatal("scheduler clock not found uniquely")
	}
	replacement := filepath.Join(dir, "driver.go")
	if err := os.WriteFile(replacement, []byte(strings.Replace(string(driver), old, "return !time.Now().Add(48 * time.Hour).Before(d.next)", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{driverPath: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go")
	if output, err := exec.Command("go", "build", "-overlay", overlayPath, "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	spec := filepath.Join(dir, "M.tla")
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "states")
	save := exec.Command(binary, "-checkpoint", "1", "-metadir", root, spec)
	save.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=2")
	if output, err := save.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) {
		t.Fatalf("save: %v %s", err, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	original, err := os.ReadFile(filepath.Join(root, entries[0].Name(), "tla-go.chkpt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		args          []string
		debug, writes bool
	}{
		{"zero", []string{"-checkpoint", "0"}, false, false},
		{"zero with debug", []string{"-checkpoint", "0"}, true, false},
		{"overflow to zero", []string{"-checkpoint", "268435456"}, true, false},
		{"omitted", nil, false, true},
		{"positive", []string{"-checkpoint", "1"}, false, true},
		{"wrapped positive", []string{"-checkpoint", "71583"}, false, true},
		{"positive with debug", []string{"-checkpoint", "1"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recovery := t.TempDir()
			path := filepath.Join(recovery, "tla-go.chkpt")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			oldTime := time.Unix(1700000000, 0)
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-recover", recovery}, tc.args...)
			cmd := exec.Command(binary, append(args, spec)...)
			if tc.debug {
				cmd.Env = append(os.Environ(), "TLA_GO_DEBUG_CHECKPOINT_AFTER=3")
			}
			output, runErr := cmd.CombinedOutput()
			if tc.debug && tc.writes {
				if runErr == nil || !bytes.Contains(output, []byte("debug checkpoint saved")) {
					t.Fatalf("debug recovery: %v %s", runErr, output)
				}
			} else if runErr != nil || !bytes.Contains(output, []byte("6 states generated, 6 distinct states found")) {
				t.Fatalf("recover: %v %s", runErr, output)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if changed := !bytes.Equal(original, after); changed != tc.writes {
				t.Fatalf("snapshot changed = %t, want %t", changed, tc.writes)
			}
			if changed := !info.ModTime().Equal(oldTime); changed != tc.writes {
				t.Fatalf("mtime changed = %t, want %t", changed, tc.writes)
			}
		})
	}
}

func TestCheckpointInputsDefaultAndDisabled(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	for _, tc := range []struct {
		name     string
		explicit bool
		interval time.Duration
		want     time.Duration
	}{
		{"omitted", false, 0, 30*time.Minute + 42*time.Millisecond},
		{"zero", true, 0, 0},
		{"positive", true, time.Minute, time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := options{path: filepath.Join(dir, "M.tla"), checkpoint: tc.explicit, checkpointInterval: tc.interval}
			_, got, _, err := checkpointInputs(opts)
			if err != nil || got != tc.want {
				t.Fatalf("interval = %v, err = %v; want %v", got, err, tc.want)
			}
		})
	}
}
