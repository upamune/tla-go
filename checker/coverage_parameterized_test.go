package checker

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestCoverageParameterizedCallOccurrencesSharePredicateRoot(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nA(n) == x'=n\nB(n) == x'=n\nNext == A(1) \\/ A(2) \\/ B(1)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithSources(module, cfg, Options{}, map[string]*parser.Module{"M": module})
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.newCoverage()
	if err != nil {
		t.Fatal(err)
	}
	branches := m.defs["Next"].Body.Args
	first, second, third := c.parameterCalls[branches[0]], c.parameterCalls[branches[1]], c.parameterCalls[branches[2]]
	if first == nil || first != second || third == nil || first == third || branches[0] == branches[1] {
		t.Fatalf("expected distinct occurrences with shared A root and separate B root: %p %p %p", first, second, third)
	}
}

func TestCoverageParameterizedSnapshotDuringAction(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nA(n) == x'=n\nNext == A(1) \\/ A(2)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithSources(module, cfg, Options{}, map[string]*parser.Module{"M": module})
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.newCoverage()
	if err != nil {
		t.Fatal(err)
	}
	m.coverage = c
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	zero, _ := value.Integer(0)
	g := generator{model: m, current: evaluator.State{"x": zero}}
	count := 0
	g.emit = func(_ evaluator.State, action string, _ *parser.Expression) (bool, error) {
		c.generated(action, false)
		c.distinct(action, false)
		count++
		if count == 1 {
			close(entered)
			<-release
		}
		return true, nil
	}
	go func() {
		_, err := g.solveNext(m.defs["Next"].Body, m.partialState())
		done <- err
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("action ended before snapshot: %v", err)
	}
	before := c.report()
	if len(before) != 2 || before[1].Name != "A" || before[1].Generated != 1 || before[1].Distinct != 1 || len(before[1].Expressions) != 1 || before[1].Expressions[0].Count != 1 {
		close(release)
		<-done
		t.Fatalf("snapshot during first call: %+v", before)
	}
	retained := cloneCoverage(before)
	before[1].Expressions[0].Count = -1
	if !reflect.DeepEqual(c.report(), retained) {
		close(release)
		<-done
		t.Fatal("snapshot aliases live collector")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after := c.report()
	if len(after) != 2 || after[1].Generated != 2 || after[1].Distinct != 2 || len(after[1].Expressions) != 1 || after[1].Expressions[0].Count != 2 {
		t.Fatalf("snapshot after both calls: %+v", after)
	}
	if retained[1].Expressions[0].Count != 1 {
		t.Fatalf("previous snapshot changed: %+v", retained)
	}
}

func TestCoverageParameterizedRejectsMissingSourceTokens(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nA(n) == x'=n\nNext == A(1) \\/ A(2)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(module)
	if err != nil {
		t.Fatal(err)
	}
	var projected parser.Module
	if err := json.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithSources(&projected, cfg, Options{}, map[string]*parser.Module{"M": &projected})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.newCoverage(); err == nil {
		t.Fatal("expected unsupported coverage without declaration tokens")
	}
}
