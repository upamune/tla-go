package checker

import (
	"maps"

	"github.com/upamune/tla-go/evaluator"
)

// workPath carries both recursion detection and an optional lexical boundary.
type workPath struct {
	names         map[string]bool
	scope         *evaluator.LexicalScope
	hasScope      bool
	functionDepth int
}

func (p *workPath) withFunction() *workPath {
	result := workPath{}
	if p != nil {
		result = *p
	}
	result.functionDepth++
	return &result
}

func (p *workPath) functionLimitReached() bool {
	return p != nil && p.functionDepth >= 4096
}

func (p *workPath) hasLexicalScope() bool { return p != nil && p.hasScope }

func (p *workPath) contains(name string) bool { return p != nil && p.names[name] }

func (p *workPath) withName(name string) *workPath {
	result := workPath{}
	if p != nil {
		result = *p
		result.names = maps.Clone(p.names)
	}
	if result.names == nil {
		result.names = map[string]bool{}
	}
	result.names[name] = true
	return &result
}

func (p *workPath) withScope(scope *evaluator.LexicalScope) *workPath {
	result := workPath{}
	if p != nil {
		result = *p
	}
	result.scope, result.hasScope = scope, true
	return &result
}
