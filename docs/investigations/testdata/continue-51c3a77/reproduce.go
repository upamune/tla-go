// Command reproduce runs the bounded Continue model-checking experiment.
// Prepare the binaries, source model and Java wrapper as described in the
// accompanying investigation before invoking this file.
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	root   = envOr("TLA_SWEEP51_DIR", "/tmp/tla-sweep51-investigation")
	source = filepath.Join(envOr("TLA_GO_JUNIT_ROOT", "/tmp/tlaplus-java-v1.7.4"), "tlatools", "org.lamport.tlatools", "test-model")
	jar    = envOr("TLA_GO_JUNIT_JAR", "/tmp/tla2tools-v1.7.4.jar")
)

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	for n := 1; n <= 8; n++ {
		for _, side := range []string{"java", "9f", "51"} {
			if err := run(side, n); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}
}

func run(side string, n int) error {
	dir := filepath.Join(root, "runs", fmt.Sprintf("%s-%02d", side, n))
	if err := prepare(dir); err != nil {
		return err
	}
	graph := filepath.Join(dir, "graph.dot")
	name, args := command(side, graph)
	output, err := os.Create(filepath.Join(dir, "output.log"))
	if err != nil {
		return err
	}
	defer output.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, output, output
	start := time.Now()
	err = cmd.Run()
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s %d: %w (context: %v)", side, n, err, ctx.Err())
	}
	data, err := os.ReadFile(graph)
	if err != nil {
		return err
	}
	fmt.Printf("%s %d exit=0 timeout=false wall=%s raw=%x\n", side, n, elapsed, sha256.Sum256(data))
	return nil
}

func prepare(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"Continue.tla", "Continue.cfg"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func command(side, graph string) (string, []string) {
	name := filepath.Join(root, "tla-"+side)
	var args []string
	if side == "java" {
		name = "java"
		args = []string{"-cp", jar, filepath.Join(root, "JUnitTLC.java")}
	}
	opts := []string{"-tool", "-deadlock", "-fp", "0"}
	if side == "java" {
		opts = append(opts, "-coverage", "1")
	}
	opts = append(opts, "-workers", "3", "-checkpoint", "0", "-dump", "dot", graph, "-continue", "Continue")
	return name, append(args, opts...)
}
