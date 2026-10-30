package value

import (
	"errors"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestNestedSubsetErrorKeepsSources(t *testing.T) {
	innerSource := &parser.Expression{Location: parser.Location{Range: parser.Range{Module: "M", Start: parser.Position{Line: 2, Column: 10}, End: parser.Position{Line: 2, Column: 20}}}}
	outerSource := &parser.Expression{Location: parser.Location{Range: parser.Range{Module: "M", Start: parser.Position{Line: 2, Column: 3}, End: parser.Position{Line: 2, Column: 21}}}}
	inner, err := SubsetsWithSource(Interval(1, 36), innerSource)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := SubsetsWithSource(inner, outerSource)
	if err != nil {
		t.Fatal(err)
	}
	_, err = outer.Key()
	failure, ok := errors.AsType[*FingerprintError](err)
	if !ok || failure.Error() != "Overflow when computing the number of elements in:\nSUBSET 1..36" {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "1) line 2, col 10 to line 2, col 20 of module M\n0) line 2, col 3 to line 2, col 21 of module M\n"
	if failure.Trace() != want || outer.String() != "SUBSET SUBSET 1..36" {
		t.Fatalf("trace %q, value %q", failure.Trace(), outer.String())
	}
}

func TestIntervalMembershipRejectsTypedModelValue(t *testing.T) {
	interval := Interval(1, 4)
	if found, err := interval.Contains(Model("A")); err != nil || found {
		t.Fatalf("untyped model: %v, %v", found, err)
	}
	if _, err := interval.Contains(Model("a_1")); err == nil || !strings.Contains(err.Error(), "a_1\nis in the integer interval 1..4") {
		t.Fatalf("typed model: %v", err)
	}
	if found, err := Interval(3, 2).Contains(Model("a_1")); err != nil || found {
		t.Fatalf("empty interval: %v, %v", found, err)
	}
}

func TestSubsetIterationDoesNotAttributeVisitorFailure(t *testing.T) {
	source := &parser.Expression{Location: parser.Location{Range: parser.Range{Module: "M", Start: parser.Position{Line: 1, Column: 1}}}}
	set, err := SubsetsWithSource(Interval(1, 2), source)
	if err != nil {
		t.Fatal(err)
	}
	want := &UnexpectedError{Message: "visitor failed"}
	err = set.Each(func(Value) (bool, error) { return false, want })
	if err != want {
		t.Fatalf("visitor error changed: %v", err)
	}
}
