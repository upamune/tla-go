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
	// shared paths only name actions; the model keeps one for each sequence.
	shared bool
}

func (p *workPath) withFunction() *workPath {
	result := workPath{}
	if p != nil {
		result = *p
	}
	result.functionDepth++
	result.shared = false
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
	result.shared = false
	return &result
}

type sharedPathKey struct {
	parent *workPath
	name   string
}

// pathWithName extends p by an expanded action name. Paths without a lexical
// boundary or function action are equal in every state, so they are shared.
func (m *model) pathWithName(p *workPath, name string) *workPath {
	if p != nil && !p.shared {
		return p.withName(name)
	}
	key := sharedPathKey{parent: p, name: name}
	if path, ok := m.sharedPaths[key]; ok {
		return path
	}
	path := p.withName(name)
	path.shared = true
	if len(m.sharedPaths) < 4096 {
		if m.sharedPaths == nil {
			m.sharedPaths = make(map[sharedPathKey]*workPath)
		}
		m.sharedPaths[key] = path
	}
	return path
}
