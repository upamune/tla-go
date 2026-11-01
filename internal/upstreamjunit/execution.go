package upstreamjunit

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

//go:embed Execution.java
var executionJava string

type executionEvent struct{ kind, value string }

func parseExecution(output string) ([]executionEvent, error) {
	var events []executionEvent
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "EXECUTION\t") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed execution record: %q", line)
		}
		switch fields[1] {
		case "started", "finished", "ignored", "failure", "assumption", "summary":
		default:
			return nil, fmt.Errorf("unknown execution event %q", fields[1])
		}
		value, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil {
			return nil, err
		}
		events = append(events, executionEvent{fields[1], string(value)})
	}
	return events, scanner.Err()
}

type executionOutcome int

const (
	executionInvalid executionOutcome = iota
	executionPassed
	executionUpstreamAssertion
	executionOtherFailure
)

// classifyBridgeExecution accepts only the complete event sequences emitted for
// this single bridge test. The marker is thrown solely around testSpec itself.
func classifyBridgeExecution(events []executionEvent) executionOutcome {
	const display = "observations(Bridge)"
	if validateExecution(events, display) == nil {
		return executionPassed
	}
	if !validBridgeFailure(events, display) {
		return executionInvalid
	}
	if events[1].value == "" {
		return executionInvalid
	}
	if strings.HasPrefix(events[1].value, "Bridge$UpstreamAssertionFailure\nBridge$UpstreamAssertionFailure") {
		return executionUpstreamAssertion
	}
	return executionOtherFailure
}

func validBridgeFailure(events []executionEvent, display string) bool {
	return len(events) == 4 && events[0] == (executionEvent{"started", display}) && events[1].kind == "failure" && events[2] == (executionEvent{"finished", display}) && events[3] == (executionEvent{"summary", "1,0,1"})
}

// A result is a pass only if JUnit reports exactly one selected test, started
// and finished with no failure, ignore, or failed assumption.
func validateExecution(events []executionEvent, display string) error {
	if len(events) != 3 || events[0] != (executionEvent{"started", display}) || events[1] != (executionEvent{"finished", display}) || events[2] != (executionEvent{"summary", "1,0,0"}) {
		return fmt.Errorf("Java JUnit did not pass exactly %s: %+v", display, events)
	}
	return nil
}

func executeJUnit(classes string, deps []string, dir, basedir, class, method string) ([]executionEvent, error) {
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Dbasedir="+basedir+string(os.PathSeparator), "-Djava.io.tmpdir="+dir, "-cp", cp, "Execution", class, method)
	cmd.Dir = dir
	javatmp.Apply(cmd)
	return runJUnitCommand(ctx, cmd)
}

func runJUnitCommand(ctx context.Context, cmd *exec.Cmd) ([]executionEvent, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("JUnit process timeout: %w: %s", ctx.Err(), output)
		}
		return nil, fmt.Errorf("JUnit process: %w: %s", err, output)
	}
	events, err := parseExecution(string(output))
	if err != nil {
		return nil, err
	}
	return events, nil
}
