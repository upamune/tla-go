package checker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCheckpointConcurrentProcesses(t *testing.T) {
	if path := os.Getenv("TLA_CHECKPOINT_PROCESS_PATH"); path != "" {
		s := checkpointFixture()
		f, err := checkpointOpenContext(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		fmt.Println("ready")
		in := bufio.NewScanner(os.Stdin)
		if !in.Scan() {
			t.Fatal("missing start")
		}
		old, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkpointDecode(old, s.Identity); err != nil {
			t.Fatal(err)
		}
		for i := range 40 {
			s.Nodes[1].Action = "Next"
			if i%2 != 0 {
				s.Nodes[1].Action = "Other"
			}
			if err := checkpointWrite(path, s); err != nil {
				t.Fatal(err)
			}
			if _, err := checkpointRead(path, s.Identity); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Println("done")
		return
	}
	path := filepath.Join(t.TempDir(), "checkpoint")
	s := checkpointFixture()
	other := checkpointFixture()
	other.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type child struct {
		cmd     *exec.Cmd
		input   io.WriteCloser
		signals <-chan string
		drained <-chan error
		stdout  *bytes.Buffer
		stderr  *bytes.Buffer
	}
	children := make([]child, 3)
	// Always reap every child, including on a failed barrier.
	defer func() {
		cancel()
		for i, c := range children {
			if c.cmd != nil {
				readErr := <-c.drained
				err := c.cmd.Wait()
				if err != nil || readErr != nil || t.Failed() {
					t.Logf("child %d: wait: %v, stdout read: %v\nstdout:\n%s\nstderr:\n%s", i, err, readErr, c.stdout.String(), c.stderr.String())
					if (err != nil || readErr != nil) && !t.Failed() {
						t.Fail()
					}
				}
			}
		}
	}()
	for i := range children {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckpointConcurrentProcesses$")
		cmd.Env = append(os.Environ(), "TLA_CHECKPOINT_PROCESS_PATH="+path)
		input, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		signals := make(chan string, 2)
		drained := make(chan error, 1)
		go func() {
			defer close(signals)
			reader := bufio.NewReader(io.TeeReader(output, stdout))
			for {
				line, err := reader.ReadString('\n')
				if line == "ready\n" || line == "done\n" {
					signals <- line[:len(line)-1]
				}
				if err != nil {
					if err == io.EOF {
						err = nil
					}
					drained <- err
					return
				}
			}
		}()
		children[i] = child{cmd, input, signals, drained, stdout, stderr}
	}

	for _, c := range children {
		if signal := <-c.signals; signal != "ready" {
			t.Fatalf("child readiness: %q", signal)
		}
	}
	// All children hold an old read handle while this replacement happens.
	if err := checkpointWrite(path, other); err != nil {
		t.Fatal(err)
	}
	held, err := checkpointOpenContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	for _, c := range children {
		if _, err := fmt.Fprintln(c.input, "start"); err != nil {
			t.Fatal(err)
		}
		_ = c.input.Close()
	}
	// Keep the parent reader open until every child has completed its writes.
	for _, c := range children {
		if signal := <-c.signals; signal != "done" {
			t.Errorf("child completion: %q", signal)
		}
	}
	data, err := io.ReadAll(held)
	if err != nil {
		t.Fatal(err)
	}
	got, err := checkpointDecode(data, s.Identity)
	if err != nil || !reflect.DeepEqual(got, other) {
		t.Errorf("held read: %v", err)
	}
	got, err = checkpointRead(path, s.Identity)
	if err != nil || (!reflect.DeepEqual(got, s) && !reflect.DeepEqual(got, other)) {
		t.Errorf("final read: %v", err)
	}
	for i, c := range children {
		readErr := <-c.drained
		err := c.cmd.Wait()
		children[i].cmd = nil
		if err != nil || readErr != nil {
			t.Errorf("child %d: wait: %v, stdout read: %v\nstdout:\n%s\nstderr:\n%s", i, err, readErr, c.stdout.String(), c.stderr.String())
		}
	}
}
