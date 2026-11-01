package upstreamjunit

import (
	"fmt"
	"slices"
)

// The upstream harness and EmptyTest constructor each supply coverage 1.
// Tool mode exposes a text projection; the harness itself uses a Java recorder.
func emptyInvocation() []string {
	return []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", "empty-bridge.dot", "-coverage", "1", "Empty"}
}

func projectEmpty(args []string, result Result) ([]bridgeRecord, []string, error) {
	if !slices.Equal(args, emptyInvocation()) {
		return nil, nil, fmt.Errorf("unexpected EmptyTest arguments: %v", args)
	}
	if result.TimedOut || result.ExitCode != 0 {
		return nil, nil, fmt.Errorf("EmptyTest process: timeout=%t exit=%d", result.TimedOut, result.ExitCode)
	}
	messages, err := Messages(result.Output)
	if err != nil {
		return nil, nil, err
	}
	if err := validateEmptyMessages(messages); err != nil {
		return nil, nil, err
	}
	records, omissions, err := projectBridge(result.Output)
	if err != nil {
		return nil, nil, err
	}
	omissions = append(omissions, "tool output cannot establish complete recorder types, ordering or hidden calls", "Go setup, dump path, globals, side effects and teardown are not reproduced")
	return records, omissions, nil
}

// An empty model has no coverage messages, even with coverage enabled. These
// are the TLC_COVERAGE_* codes in the pinned Java TLC's EC.java and MP.java.
func validateEmptyMessages(messages map[string][]string) error {
	for _, id := range []string{"2201", "2202", "2221", "2772", "2773", "2774", "2775", "2776", "2777"} {
		if len(messages[id]) != 0 {
			return fmt.Errorf("unexpected empty-model coverage message %s", id)
		}
	}
	for _, id := range []string{"2186", "2199"} {
		for _, body := range messages[id] {
			_, omission := projectMessage(id, body)
			if omission != "" {
				return fmt.Errorf("%s", omission)
			}
		}
	}
	return nil
}
