package evaluator

import (
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type unchangedOrdering struct {
	strings  []string
	onIntern func(string)
}

func (o *unchangedOrdering) Compare(a, b value.Value) (int, error) { return value.Compare(a, b) }
func (o *unchangedOrdering) Intern(s string) {
	o.strings = append(o.strings, s)
	if o.onIntern != nil {
		o.onIntern(s)
	}
}

func TestUnchangedVariablesMatchesObservedEvaluation(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y\nvars == <<x,y>>\nCheck == UNCHANGED vars\n====")
	if err != nil {
		t.Fatal(err)
	}
	p := New(m)
	expr := m.Definitions[1].Body
	one, _ := value.Integer(1)
	for _, tc := range []struct {
		name       string
		current    State
		next       State
		want       string
		wantError  string
		wantIntern []string
	}{
		{"same", State{"x": value.String("a"), "y": value.String("b")}, State{"x": value.String("a"), "y": value.String("b")}, "TRUE", "", []string{"a", "b", "a", "b"}},
		{"different first", State{"x": value.String("a"), "y": value.String("b")}, State{"x": value.String("c"), "y": value.String("d")}, "FALSE", "", []string{"a", "b", "c", "d"}},
		{"different last", State{"x": value.String("a"), "y": value.String("b")}, State{"x": value.String("a"), "y": value.String("c")}, "FALSE", "", []string{"a", "b", "a", "c"}},
		{"incompatible", State{"x": one, "y": value.String("b")}, State{"x": value.Bool(true), "y": value.String("c")}, "", "non-integer", []string{"b", "c"}},
		{"missing next state", State{"x": value.String("a"), "y": value.String("b")}, nil, "", "prime without a next state", []string{"a", "b"}},
		{"missing next variable", State{"x": value.String("a"), "y": value.String("b")}, State{"x": value.String("a")}, "", "variable y is unassigned", []string{"a", "b", "a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var observed []string
			ctx := Context{Current: tc.current, Next: tc.next}
			fastOrder := &unchangedOrdering{}
			p.SetOrdering(fastOrder)
			fast, fastErr := p.Evaluate(expr, ctx)
			slowOrder := &unchangedOrdering{}
			p.SetOrdering(slowOrder)
			ctx.OnEvaluate = func(e *parser.Expression) { observed = append(observed, e.Kind) }
			slow, slowErr := p.Evaluate(expr, ctx)
			if errorText(fastErr) != errorText(slowErr) || fast.String() != slow.String() {
				t.Fatalf("fast = %v, %v; observed = %v, %v", fast, fastErr, slow, slowErr)
			}
			if tc.wantError != "" && (fastErr == nil || !strings.Contains(fastErr.Error(), tc.wantError)) {
				t.Fatalf("error %v, want %q", fastErr, tc.wantError)
			}
			if tc.want != "" && (fastErr != nil || fast.String() != tc.want) {
				t.Fatalf("result %v, error %v, want %s", fast, fastErr, tc.want)
			}
			if !reflect.DeepEqual(fastOrder.strings, tc.wantIntern) || !reflect.DeepEqual(fastOrder.strings, slowOrder.strings) {
				t.Fatalf("intern fast %q, observed %q, want %q", fastOrder.strings, slowOrder.strings, tc.wantIntern)
			}
			if len(observed) == 0 || observed[0] != "Unchanged" {
				t.Fatalf("missing evaluation events: %v", observed)
			}
			if tc.name == "same" {
				want := []string{"Unchanged", "Identifier", "Tuple", "Identifier", "Identifier", "Identifier", "Tuple", "Identifier", "Identifier"}
				if !reflect.DeepEqual(observed, want) {
					t.Fatalf("events %v, want %v", observed, want)
				}
			}
		})
	}
}

func TestUnchangedVariablesOrderingMayMutateState(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y\nvars == <<x,y>>\nCheck == UNCHANGED vars\n====")
	if err != nil {
		t.Fatal(err)
	}
	p := New(m)
	expr := m.Definitions[1].Body
	ctx := Context{Current: State{"x": value.String("a"), "y": value.String("b")}, Next: State{"x": value.String("a"), "y": value.String("b")}}
	order := &unchangedOrdering{onIntern: func(s string) {
		if s == "a" {
			ctx.Current["y"] = value.String("changed")
		}
	}}
	p.SetOrdering(order)
	e := evaluator{program: p, scope: &p.root, variables: p.variables, context: ctx}
	if _, ok, _ := e.unchangedVariables(expr); ok {
		t.Fatal("an observable string intern must use ordinary evaluation")
	}
	v, err := p.Evaluate(expr, ctx)
	want := []string{"a", "changed", "a", "b"}
	if err != nil || v.String() != "FALSE" || !reflect.DeepEqual(order.strings, want) {
		t.Fatalf("got %s, error %v, interned %q; want FALSE, %q", v, err, order.strings, want)
	}
}

func TestUnchangedVariablesPreservesShadowingAndEffects(t *testing.T) {
	for _, tc := range []struct {
		name, definition, want string
		prints                 []string
	}{
		{"local shadows direct tuple", "UNCHANGED <<x,y>>", "TRUE", nil},
		{"alias resolves in its own scope", "UNCHANGED vars", "FALSE", nil},
		{"side effect before and after prime", `UNCHANGED <<PrintT("effect"),y>>`, "TRUE", []string{`"effect"`, `"effect"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS TLC\nVARIABLES x, y\nvars == <<x,y>>\nCheck == " + tc.definition + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			one, _ := value.Integer(1)
			two, _ := value.Integer(2)
			ctx := Context{Current: State{"x": one, "y": one}, Next: State{"x": two, "y": one}, Locals: map[string]value.Value{"x": one}}
			var printed []string
			ctx.OnPrint = func(s string) { printed = append(printed, s) }
			v, err := New(m).Evaluate(m.Definitions[1].Body, ctx)
			if err != nil || v.String() != tc.want || !reflect.DeepEqual(printed, tc.prints) {
				t.Fatalf("got %s, error %v, printed %q; want %s, %q", v, err, printed, tc.want, tc.prints)
			}
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestUnchangedVariablesReturnsFalseOnComparisonError(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y\nvars == <<x,y>>\nCheck == UNCHANGED vars\n====")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := value.Filter(value.Interval(1, 4), func(v value.Value) (bool, error) {
		n, _ := v.Int()
		return n > 2, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	for _, tc := range []struct {
		name string
		set  value.Value
	}{
		{"infinite set", value.NaturalNumbers()},
		{"deferred filter", filtered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := Context{Current: State{"x": one, "y": tc.set}, Next: State{"x": one, "y": one}}
			p := New(m)
			fast, fastErr := p.Evaluate(m.Definitions[1].Body, ctx)
			ctx.OnEvaluate = func(*parser.Expression) {}
			slow, slowErr := p.Evaluate(m.Definitions[1].Body, ctx)
			if fast.String() != "FALSE" || slow.String() != "FALSE" || fastErr == nil || errorText(fastErr) != errorText(slowErr) {
				t.Fatalf("fast=%v, %v; slow=%v, %v; want FALSE and the same error", fast, fastErr, slow, slowErr)
			}
		})
	}
}

func TestUnchangedVariablesValueMatrix(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y\nvars == <<x,y>>\nCheck == UNCHANGED vars\n====")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	set, _ := value.Set([]value.Value{one, two})
	tuple, _ := value.Tuple([]value.Value{one, two})
	filtered, err := value.Filter(value.Interval(1, 4), func(v value.Value) (bool, error) {
		n, _ := v.Int()
		return n > 2, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	values := []value.Value{one, two, value.Bool(true), value.String("x"), value.Interval(1, 2), set, tuple, filtered, value.NaturalNumbers()}
	p := New(m)
	for _, a := range values {
		for _, b := range values {
			for _, c := range values {
				for _, d := range values {
					ctx := Context{Current: State{"x": a, "y": b}, Next: State{"x": c, "y": d}}
					fast, fastErr := p.Evaluate(m.Definitions[1].Body, ctx)
					ctx.OnEvaluate = func(*parser.Expression) {}
					slow, slowErr := p.Evaluate(m.Definitions[1].Body, ctx)
					if fast.String() != slow.String() || errorText(fastErr) != errorText(slowErr) {
						t.Fatalf("states %v,%v -> %v,%v: fast=%v,%v slow=%v,%v", a, b, c, d, fast, fastErr, slow, slowErr)
					}
				}
			}
		}
	}
}

func TestUnchangedVariablesEligibility(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y\nvars == <<x,y>>\nCheck == UNCHANGED vars\n====")
	if err != nil {
		t.Fatal(err)
	}
	p := New(m)
	expr := m.Definitions[1].Body
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	ctx := Context{Current: State{"x": one, "y": two}, Next: State{"x": one, "y": two}}
	e := evaluator{program: p, scope: &p.root, variables: p.variables, context: ctx}
	if same, ok, err := e.unchangedVariables(expr); !ok || !same || err != nil {
		t.Fatalf("ordinary variable tuple: same=%t ok=%t err=%v", same, ok, err)
	}
	e.context.Current["x"] = value.String("a")
	e.context.Next["x"] = value.String("b")
	if same, ok, err := e.unchangedVariables(expr); !ok || same || err != nil {
		t.Fatalf("strings without an ordering: same=%t ok=%t err=%v", same, ok, err)
	}
	e.scope = &scope{parent: &p.root, values: map[string]value.Value{"x": three}}
	if _, ok, _ := e.unchangedVariables(expr); ok {
		t.Fatal("a shadowing local must use ordinary evaluation")
	}
	e.scope = &p.root
	e.context.Lexical = &LexicalScope{enabledScopes: []string{"instance"}}
	e.context.UncachedArguments = true
	copyExpr := *expr
	copyExpr.EnabledBindings = []parser.EnabledBinding{{Scope: "instance", Name: "x"}}
	if _, ok, _ := e.unchangedVariables(&copyExpr); ok {
		t.Fatal("an enabled binding must use ordinary evaluation")
	}
}
