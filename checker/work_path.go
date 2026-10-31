package checker

import (
	"github.com/upamune/tla-go/evaluator"
)

// workPath carries both recursion detection and an optional lexical boundary.
type workPath struct {
	names         *expandedName
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

// expandedName lists the action names expanded on a path, innermost first.
type expandedName struct {
	name   string
	parent *expandedName
}

func (p *workPath) contains(name string) bool {
	if p == nil {
		return false
	}
	for current := p.names; current != nil; current = current.parent {
		if current.name == name {
			return true
		}
	}
	return false
}

func (p *workPath) withName(name string) *workPath {
	named := &struct {
		path workPath
		name expandedName
	}{}
	if p != nil {
		named.path = *p
	}
	named.name = expandedName{name: name, parent: named.path.names}
	named.path.names = &named.name
	return &named.path
}

func (p *workPath) withScope(scope *evaluator.LexicalScope) *workPath {
	result := workPath{}
	if p != nil {
		result = *p
	}
	result.scope, result.hasScope = scope, true
	return &result
}
