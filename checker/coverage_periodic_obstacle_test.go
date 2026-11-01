package checker

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestCoverageReportPreservesUpdatesAndSnapshots(t *testing.T) {
	first := parser.Range{Module: "M", Start: parser.Position{Line: 1}}
	second := parser.Range{Module: "M", Start: parser.Position{Line: 2}}
	body := parser.Range{Module: "M", Start: parser.Position{Line: 3}}
	propertyExpr := &parser.Expression{Location: parser.Location{Range: first}}
	property := &coverageNode{expr: propertyExpr, count: 1, cost: 2}
	c := coverageCollector{
		actions: []CoverageAction{{Name: "A", Expressions: []CoverageExpression{
			{Range: first, Count: 1}, {Range: second, Count: 1},
		}}, {Name: "P", Property: true}},
		byName:      map[string]int{"A": 0, "P": 1},
		bodies:      map[string]*parser.Expression{"A": {Kind: "And", Location: parser.Location{Range: body}}},
		expressions: map[string]map[parser.Range]int{"A": {first: 0, second: 1}},
		properties:  map[string]*coverageNode{"P": property},
	}
	before := c.report()
	wantBefore := []CoverageAction{
		{Name: "A", Expressions: []CoverageExpression{{Range: body, Count: 1}}},
		{Name: "P", Property: true, Expressions: []CoverageExpression{{Range: first, Count: 1, Cost: 2}}},
	}
	if !reflect.DeepEqual(before, wantBefore) || !reflect.DeepEqual(c.report(), before) {
		t.Fatalf("unexpected or unstable report: %+v", before)
	}
	g := generator{model: &model{coverage: &c}}
	g.cover(&parser.Expression{Location: parser.Location{Range: second}}, "A")
	c.generated("A", false)
	c.distinct("A", false)
	property.observe(propertyExpr)
	property.cost++
	after := c.report()
	wantAfter := []CoverageAction{
		{Name: "A", Generated: 1, Distinct: 1, Expressions: []CoverageExpression{{Range: first, Count: 1}, {Range: second, Count: 2}}},
		{Name: "P", Property: true, Expressions: []CoverageExpression{{Range: first, Count: 2, Cost: 3}}},
	}
	if !reflect.DeepEqual(after, wantAfter) || !reflect.DeepEqual(c.report(), after) {
		t.Fatalf("unexpected or unstable report after updates: %+v", after)
	}
	if !reflect.DeepEqual(before, wantBefore) {
		t.Fatalf("earlier report changed: %+v", before)
	}
	after[0].Expressions[1].Count = 99
	after[1].Expressions[0].Cost = 99
	if !reflect.DeepEqual(c.report(), wantAfter) {
		t.Fatal("mutating a report changed the collector")
	}
}

// Exercise every runtime writer while reports and the replay flag are read.
// Returned reports own their storage and all runtime counters remain intact.
func TestCoverageConcurrentSnapshots(t *testing.T) {
	r := parser.Range{Module: "M", Start: parser.Position{Line: 1}}
	d := parser.Range{Module: "M", Start: parser.Position{Line: 2}}
	p := &parser.Expression{Location: parser.Location{Range: d}}
	leaf := &coverageNode{expr: p}
	root := &coverageNode{expr: &parser.Expression{Location: parser.Location{Range: r}}, children: []*coverageNode{leaf}}
	root.boundCosts = map[*parser.Expression]*coverageNode{p: leaf}
	expr := &parser.Expression{Text: "=", Location: parser.Location{Range: r}}
	c := &coverageCollector{
		actions:               []CoverageAction{{Name: "A", Expressions: []CoverageExpression{{Range: r}, {Range: d}}}, {Name: "P", Property: true}},
		byName:                map[string]int{"A": 0, "P": 1},
		bodies:                map[string]*parser.Expression{"A": {Kind: "Other"}},
		expressions:           map[string]map[parser.Range]int{"A": {r: 0}},
		guards:                map[string]map[parser.Range]int{"A": {r: 0}},
		assignmentMemberships: map[string]map[parser.Range]int{"A": {r: 0}},
		existentialDomains:    map[string]map[parser.Range]int{"A": {d: 1}},
		properties:            map[string]*coverageNode{"P": root},
	}
	g := generator{model: &model{coverage: c}}
	const n = 1000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range n {
			g.cover(expr, "A")
			c.generated("A", false)
			c.distinct("A", false)
			c.observeProperty(root, p)
			c.observePropertyBound(root, p, false)
			g.coverExistentialDomain(p, "A")
			g.coverGuardSuccess(expr, "A")
			g.coverAssignmentMember(expr, nil, "A", true, 1)
			g.coverAssignmentDomain(expr, "x", nil, "A", true)
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			got := c.report()
			_ = c.existentialEvaluated()
			if len(got) != 2 || len(got[0].Expressions) == 0 {
				t.Error("invalid snapshot")
				return
			}
			// A report owns its memory even while updates continue.
			got[0].Expressions[0].Count = -1
		}
	}()
	wg.Wait()
	if !c.existentialEvaluated() || c.actions[0].Generated != n || c.actions[0].Distinct != n || leaf.count != n || leaf.cost != n {
		t.Fatal("lost updates")
	}
}

func TestCoverageUpdateUnlocksAfterPanic(t *testing.T) {
	c := &coverageCollector{byName: map[string]int{"": 0}, actions: []CoverageAction{{}}, bodies: map[string]*parser.Expression{"": {Kind: "Other"}}}
	g := generator{model: &model{coverage: c}}
	func() { defer func() { _ = recover() }(); g.cover(nil, "") }()
	done := make(chan struct{})
	go func() { c.report(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector remained locked after panic")
	}
}

func TestCoverageSnapshotDoesNotSplitGuardUpdate(t *testing.T) {
	r := parser.Range{Module: "M", Start: parser.Position{Line: 1}}
	child := parser.Range{Module: "M", Start: parser.Position{Line: 2}}
	expr := &parser.Expression{Text: "=", Location: parser.Location{Range: r}}
	c := &coverageCollector{
		actions:     []CoverageAction{{Name: "A", Expressions: []CoverageExpression{{Range: r}, {Range: child, Depth: 1}}}},
		byName:      map[string]int{"A": 0},
		expressions: map[string]map[parser.Range]int{"A": {r: 0}},
		guards:      map[string]map[parser.Range]int{"A": {r: 0}},
		bodies:      map[string]*parser.Expression{"A": {Kind: "Other"}},
	}
	g := generator{model: &model{coverage: c}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 5000 {
			g.cover(expr, "A")
		}
	}()
	for range 5000 {
		got := c.report()
		// Equal parent and child are collapsed by the formatter; a torn read
		// would produce two distinct expressions.
		if len(got[0].Expressions) > 1 {
			t.Fatalf("torn guard update: %+v", got)
		}
	}
	<-done
}

func TestCoverageSnapshotsDuringSearchAndCallback(t *testing.T) {
	const source = `---- MODULE M ----
VARIABLE x
Init == x = 0
A == x' = x
B == x' = x
Next == A \/ B
Inv == x = 0
====`
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE")
	if err != nil {
		t.Fatal(err)
	}
	var m *model
	callbacks := 0
	options := Options{Coverage: true, OnSearch: func(SearchEvent) { callbacks++; _ = m.coverage.report() }}
	m, err = prepareWithOutput(module, cfg, options)
	if err != nil {
		t.Fatal(err)
	}
	m.ctx = context.Background()
	if err = m.configureCoverage(options); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	doneSnapshots := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-doneSnapshots:
				return
			default:
				_ = m.coverage.report()
			}
		}
	}()
	result, err := m.check()
	close(doneSnapshots)
	<-done
	if err != nil || result.Status != Success || callbacks == 0 {
		t.Fatalf("search: %v %v; callbacks: %d", result.Status, err, callbacks)
	}
}
