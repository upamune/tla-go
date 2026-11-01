package checker

import (
	"fmt"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func BenchmarkCoveragePropertyReportDeep(b *testing.B) {
	for _, depth := range []int{64, 256, 1024} {
		b.Run(fmt.Sprint(depth), func(b *testing.B) {
			root := &coverageNode{expr: &parser.Expression{}, count: 1}
			n := root
			for i := 1; i < depth; i++ {
				child := &coverageNode{expr: &parser.Expression{}, count: i%2 + 1}
				n.children = []*coverageNode{child}
				n = child
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				root.report(nil, 0)
			}
		})
	}
}

func BenchmarkCoveragePropertyReportCollapsed(b *testing.B) {
	root := &coverageNode{expr: &parser.Expression{}, count: 1}
	n := root
	for range 255 {
		child := &coverageNode{expr: &parser.Expression{}, count: 1}
		n.children = []*coverageNode{child}
		n = child
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		root.report(nil, 0)
	}
}
