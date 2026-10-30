package evaluator

import (
	"sync"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestLazyFilterCapturesStateAndScope(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nResult == LET cutoff == x IN {n \\in Nat : n > cutoff}\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	state := State{"x": two}
	set, err := Evaluate(module, module.Definitions[0].Body, Context{Current: state})
	if err != nil {
		t.Fatal(err)
	}
	state["x"] = three
	found, err := set.Contains(three)
	if err != nil || !found {
		t.Fatalf("captured state changed: %v, %v", found, err)
	}
}

func TestLazyFilterConcurrentMembership(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nAbove(n) == n > x\nResult == {n \\in Nat : Above(n)}\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	set, err := Evaluate(module, module.Definitions[1].Body, Context{Current: State{"x": one}})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			for range 20 {
				found, err := set.Contains(two)
				if err != nil || !found {
					t.Errorf("membership: %v, %v", found, err)
					return
				}
			}
		})
	}
	group.Wait()
}

func TestRecursiveLazyFilterTerminatesWithError(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nD == {n \\in Nat : n \\in D}\nResult == 1 \\in D\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Evaluate(module, module.Definitions[1].Body, Context{}); err == nil {
		t.Fatal("recursive filter was accepted")
	}
}

func TestLazyFilterCapturesOperatorScope(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nMake(Op(_)) == {n \\in Nat : Op(n)}\nResult == Make(LAMBDA n : n > threshold)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	locals := map[string]value.Value{"threshold": two}
	set, err := Evaluate(module, module.Definitions[1].Body, Context{Locals: locals})
	if err != nil {
		t.Fatal(err)
	}
	locals["threshold"] = three
	found, err := set.Contains(three)
	if err != nil || !found {
		t.Fatalf("captured operator scope changed: %v, %v", found, err)
	}
}

func TestLazyFilterConcurrentOperatorCapture(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nMake(Op(_)) == {n \\in Nat : Op(n)}\nResult == Make(LAMBDA n : n > threshold)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	locals := map[string]value.Value{"threshold": one}
	set, err := Evaluate(module, module.Definitions[1].Body, Context{Locals: locals})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	group.Go(func() {
		for range 200 {
			locals["threshold"] = two
			locals["threshold"] = one
		}
	})
	for range 8 {
		group.Go(func() {
			for range 100 {
				found, err := set.Contains(two)
				if err != nil || !found {
					t.Errorf("membership: %v, %v", found, err)
					return
				}
			}
		})
	}
	group.Wait()
}
