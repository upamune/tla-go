package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test-only bridge preserves Java SANY and adapts TLC's Java invocation to
// tla-go. It copies library modules into the temporary run directory and omits
// -metadir, which are not yet implemented as CLI interfaces.
func TestMain(m *testing.M) {
	if os.Getenv("TLA_GO_STATECHART_BRIDGE") == "1" {
		os.Exit(statechartBridge(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func statechartBridge(args []string) int {
	folder, index := "", -1
	for i, arg := range args {
		if after, ok := strings.CutPrefix(arg, "-DTLA-Library="); ok {
			folder = after
		}
		if arg == "tlc2.TLC" {
			index = i
		}
	}
	if index < 0 {
		return bridgeJava(args)
	}
	if err := bridgeCopyModules(folder); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 255
	}
	return run(context.Background(), bridgeTLCArgs(args[index+1:]), os.Stdout, os.Stderr)
}

func bridgeJava(args []string) int {
	cmd := exec.Command("java", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 255
	}
	return 0
}

func bridgeTLCArgs(args []string) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-metadir" && i+1 < len(args) {
			i++
			continue
		}
		result = append(result, args[i])
	}
	return result
}

func bridgeCopyModules(folder string) error {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tla") {
			continue
		}
		if _, err := os.Stat(entry.Name()); err == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(folder, entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(entry.Name(), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func TestJavaTLCStatechartHybrid(t *testing.T) {
	root, jar := os.Getenv("STATECHART_DIR"), os.Getenv("TLC_JAR")
	if root == "" || jar == "" {
		t.Skip("set STATECHART_DIR and TLC_JAR to run statechart differential tests")
	}
	bridge, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	corpus := filepath.Join(root, "scripts", "test", "corpus")
	entries, err := os.ReadDir(corpus)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		count++
		t.Run(entry.Name(), func(t *testing.T) {
			compareStatechartHybrid(t, root, jar, bridge, filepath.Join(corpus, entry.Name()))
		})
	}
	if count == 0 {
		t.Fatal("statechart corpus contains no models")
	}
}

func compareStatechartHybrid(t *testing.T, root, jar, bridge, fixture string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	name := filepath.Base(fixture)
	out := filepath.Join(t.TempDir(), "output")
	args := []string{filepath.Join(root, "scripts", "statechart.ts"), filepath.Join(fixture, name+".tla"), "--out", out}
	if view := filepath.Join(fixture, "view.mjs"); fileExists(view) {
		args = append(args, "--view", view)
	}
	cmd := exec.CommandContext(ctx, "bun", args...)
	cmd.Env = append(os.Environ(), "JAVA="+bridge, "TLA2TOOLS="+jar, "TLA_GO_STATECHART_BRIDGE=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil {
		t.Fatalf("statechart: %v\n%s", err, output)
	}
	if want := statechartExit(t, fixture); cmd.ProcessState.ExitCode() != want {
		t.Fatalf("exit %d, want %d\n%s", cmd.ProcessState.ExitCode(), want, output)
	}
	for _, file := range []string{"chart.mmd", "report.md"} {
		compareStatechartFile(t, filepath.Join(fixture, file), filepath.Join(out, file))
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func statechartExit(t *testing.T, fixture string) int {
	t.Helper()
	file := filepath.Join(fixture, "verdict-1.exit")
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		return 0
	}
	if code == 11 || code == 12 || code == 13 {
		return 1
	}
	return 2
}

func compareStatechartFile(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, wantErr := os.ReadFile(wantPath)
	got, gotErr := os.ReadFile(gotPath)
	if errors.Is(wantErr, os.ErrNotExist) && errors.Is(gotErr, os.ErrNotExist) {
		return
	}
	if wantErr != nil || gotErr != nil || string(want) != string(got) {
		t.Fatalf("%s differs (expected error %v, actual error %v)\nwant:\n%s\ngot:\n%s", filepath.Base(wantPath), wantErr, gotErr, want, got)
	}
}
