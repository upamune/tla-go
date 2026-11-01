package checker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// The Java oracle's tableau graph for this sampled trace executes nine
// ENABLED checks and four non-stuttering action bodies, in this order.
func TestLiveTraceJavaFairnessEvaluationUnits(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == /\ PrintT(<<"set", x>>) /\ x'=(x+1)%3
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == <>(x=8)
====`
	m := prepareSimulationTest(t, source, "SPECIFICATION Spec PROPERTY P")
	var printed []string
	m.program = newEvaluatorProgram(m.module, func(s string) { printed = append(printed, s) }, m.tlcRuntime)
	solutions, err := m.liveSolutions()
	if err != nil || len(solutions) != 1 || solutions[0].tableau == nil {
		t.Fatalf("solutions: %+v, %v", solutions, err)
	}
	var states []evaluator.State
	for _, x := range []int64{0, 1, 2, 0, 1} {
		v, err := value.Integer(x)
		if err != nil {
			t.Fatal(err)
		}
		states = append(states, evaluator.State{"x": v})
	}
	trace, err := newLiveTrace(m, states, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = newLiveTraceGraph(trace, &solutions[0]); err != nil {
		t.Fatal(err)
	}
	want := []string{`<<"set", 0>>`, `<<"set", 0>>`, `<<"set", 0>>`, `<<"set", 1>>`, `<<"set", 1>>`, `<<"set", 1>>`, `<<"set", 2>>`, `<<"set", 2>>`, `<<"set", 2>>`, `<<"set", 0>>`, `<<"set", 0>>`, `<<"set", 0>>`, `<<"set", 1>>`}
	if !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %q, want %q", printed, want)
	}
}

// LiveCheck1 evaluates each check again even when the same leaf occurred in
// an earlier graph operation. This also exercises fairness's ENABLED action.
func TestSimulationLivenessFairnessEffects(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == /\ x' = (x + 1) % 2 /\ PrintT(<<x, x'>>)
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == <>(x = 1)
====`
	const cfg = "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	var printed []string
	result, err := CheckWithOptions(context.Background(), source, cfg, Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: true}, OnPrint: func(s string) { printed = append(printed, s) }})
	if err != nil || result.Status != Success {
		t.Fatalf("Go: %+v, %v", result, err)
	}
	want := []string{"<<0, 1>>", "<<1, 0>>", "<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<1, 0>>", "<<1, 0>>", "<<1, 0>>", "<<0, 1>>"}
	if !reflect.DeepEqual(printed, want) || result.GeneratedStates != 4 {
		t.Fatalf("Go prints %q, generated %d; want %q, 4", printed, result.GeneratedStates, want)
	}
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to pinned v1.7.4 jar")
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR sha256 %s is not pinned v1.7.4", got)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("java", "-Xmx256m", "-jar", jar, "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "3", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Java: %v\n%s", err, output)
	}
	var java []string
	for line := range strings.SplitSeq(string(output), "\n") {
		if strings.HasPrefix(line, "<<") {
			java = append(java, line)
		}
	}
	if !strings.Contains(string(output), "Progress: 4 states checked.") {
		t.Fatalf("Java state count:\n%s", output)
	}
	if !reflect.DeepEqual(printed, java) {
		t.Fatalf("Go %q; Java %q\n%s", printed, java, output)
	}
}

// The second graph operation must propagate a new error rather than reuse the
// earlier true result. Results already stored on graph nodes remain intact.
func TestLiveTraceReevaluatesChecksAndPropagatesFailure(t *testing.T) {
	calls := 0
	failure := errors.New("second evaluation")
	leaf := &liveNode{kind: liveState, leaf: &liveLeaf{tag: 1, eval: func(_, _ evaluator.State) (bool, error) {
		calls++
		if calls == 2 {
			return false, failure
		}
		return true, nil
	}}}
	trace := &liveTrace{states: []evaluator.State{nil}, uncached: true}
	graph := &liveTraceGraph{liveTrace: trace, solution: &liveSolution{checkState: []*liveNode{leaf}}}
	first, err := graph.checkState(0)
	if err != nil || !first[0] {
		t.Fatalf("first check: %v, %v", first, err)
	}
	if _, err := graph.checkState(0); !errors.Is(err, failure) {
		t.Fatalf("second check error: %v", err)
	}
	if !first[0] || calls != 2 {
		t.Fatalf("stored result %v, calls %d", first, calls)
	}
}

// Each simulation owns its liveness graph and check results.
func TestSimulationLivenessFairnessEffectsConcurrent(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == /\ x' = (x + 1) % 2 /\ PrintT(<<x, x'>>)
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == <>(x = 1)
====`
	for i := range 4 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			count := 0
			result, err := CheckWithOptions(context.Background(), source, "SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE", Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: true}, OnPrint: func(string) { count++ }})
			if err != nil || result.Status != Success || count != 10 {
				t.Fatalf("result %+v, error %v, prints %d", result, err, count)
			}
		})
	}
}

// Default simulation retains the prior per-slot cache.
func TestLiveTraceRetainsCachedChecks(t *testing.T) {
	calls := 0
	leaf := &liveNode{kind: liveState, leaf: &liveLeaf{tag: 1, eval: func(_, _ evaluator.State) (bool, error) {
		calls++
		return true, nil
	}}}
	trace := &liveTrace{states: []evaluator.State{nil}}
	for range 2 {
		if ok, err := trace.leaf(leaf, 0, -1); err != nil || !ok {
			t.Fatalf("leaf: %v, %v", ok, err)
		}
	}
	if calls != 1 {
		t.Fatalf("got %d evaluations, want 1", calls)
	}
}
