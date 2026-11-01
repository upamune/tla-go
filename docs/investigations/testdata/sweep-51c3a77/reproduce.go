// Command reproduce runs the bounded SimpleMultiProc and EWD840MC2 follow-ups.
// Prepare the translated models, binaries, CodePlex model and Java wrapper as
// described in simple-multiproc.md before running this file.
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	root = envOr("TLA_SWEEP51_DIR", "/tmp/tla-sweep51-investigation")
	jar  = envOr("TLA_GO_JUNIT_JAR", "/tmp/tla2tools-v1.7.4.jar")
)

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

type scenario struct {
	label, dir, executable string
	args                   []string
	timeout                time.Duration
	want                   int
}

func main() {
	for _, side := range []string{"9f", "51", "java"} {
		if err := run(simple(side)); err != nil {
			fail(err)
		}
	}
	if err := run(codeplex()); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func simple(side string) scenario {
	dir := filepath.Join(root, "simple", side)
	name := filepath.Join(root, "tla-"+side)
	var args []string
	if side == "java" {
		name, args = "java", wrapper()
	}
	opts := []string{"-tool", "-deadlock", "-fp", "0"}
	if side == "java" {
		opts = append(opts, "-coverage", "1")
	}
	opts = append(opts, "-workers", "1", "-checkpoint", "0", "-dump", "dot", filepath.Join(dir, "graph.dot"), "SimpleMultiProc")
	return scenario{label: side, dir: dir, executable: name, args: append(args, opts...), timeout: 75 * time.Second}
}

func codeplex() scenario {
	dir := filepath.Join(root, "codeplex")
	args := append(wrapper(), "-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", filepath.Join(dir, "graph.dot"), "EWD840MC2")
	return scenario{label: "codeplex-java", dir: dir, executable: "java", args: args, timeout: 45 * time.Second, want: 13}
}

func wrapper() []string {
	return []string{"-cp", jar, filepath.Join(root, "JUnitTLC.java")}
}

func run(s scenario) error {
	graph := filepath.Join(s.dir, "graph.dot")
	if err := refuseGraphs(s.dir); err != nil {
		return err
	}
	out, err := os.OpenFile(filepath.Join(s.dir, "run.log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.executable, s.args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = s.dir, out, out
	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)
	if ctx.Err() != nil {
		return fmt.Errorf("%s timed out after %s: %w", s.label, elapsed, ctx.Err())
	}
	if err := checkExit(runErr, s.want); err != nil {
		return fmt.Errorf("%s: %w", s.label, err)
	}
	hash, err := fileHash(graph)
	if err != nil {
		return err
	}
	fmt.Printf("%s exit=%d timeout=false wall=%s graph_sha256=%x\n", s.label, s.want, elapsed, hash)
	return nil
}

func refuseGraphs(dir string) error {
	for _, name := range []string{"graph.dot", "graph_liveness.dot"} {
		graphFile := filepath.Join(dir, name)
		if _, err := os.Lstat(graphFile); err == nil {
			return fmt.Errorf("refusing to overwrite %s", graphFile)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func checkExit(err error, want int) error {
	if err == nil && want == 0 {
		return nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == want {
		return nil
	}
	return fmt.Errorf("exit: got %v, want %d", err, want)
}

func fileHash(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
