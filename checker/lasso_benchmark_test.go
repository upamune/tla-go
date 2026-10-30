package checker

import (
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func BenchmarkLassoPropertyReporting(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nReach == <>(x = 5)\nVisit == []<>(x = 2)\nSettle == <>[](x = 0)\nResponse == (x = 1) ~> (x = 9)\nProgress == []<><<x' = x + 1>>_x\nMissing == []<><<x' = 9>>_x\nEvery == \\A i \\in {0,1} : <>[](x = i)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse("SPECIFICATION Spec PROPERTIES Reach Visit Settle Response Progress Missing Every")
	if err != nil {
		b.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		b.Fatal(err)
	}
	trace := make([]TraceStep, 3)
	for i := range trace {
		n, err := value.Integer(int64(i))
		if err != nil {
			b.Fatal(err)
		}
		trace[i].State = evaluator.State{"x": n}
	}
	loop := 0
	violation := Violation{Trace: trace, LoopStart: &loop}
	s := search{model: m}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.violatedLassoNames(&violation); err != nil {
			b.Fatal(err)
		}
	}
}
