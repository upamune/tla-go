package evaluator

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// callFrame is a scope with one binding and, for a lazy argument, its cache.
type callFrame struct {
	scope    scope
	binding  [1]binding
	argument primeArgument
}

const frameChunk = 256

// frameArena lends call frames in stack order: a frame is released when the
// operator call or bound element that took it finishes, after any nested
// evaluation. Each top-level evaluation owns an arena. Values that outlive a
// call capture snapshots, which use none, instead of its scopes, so released
// frames are unreachable. The only lasting reference is an operator scope
// address in a function cache key; pin keeps such a frame from being reused.
// A nil arena allocates every frame.
type frameArena struct {
	chunks      [][]callFrame
	chunk, used int
}

type frameMark struct{ chunk, used int }

func (a *frameArena) mark() frameMark {
	if a == nil {
		return frameMark{}
	}
	return frameMark{a.chunk, a.used}
}

func (a *frameArena) release(m frameMark) {
	if a != nil {
		a.chunk, a.used = m.chunk, m.used
	}
}

func (a *frameArena) take() *callFrame {
	if a == nil {
		return new(callFrame)
	}
	if a.chunk < len(a.chunks) && a.used == frameChunk {
		a.chunk++
		a.used = 0
	}
	if a.chunk == len(a.chunks) {
		a.chunks = append(a.chunks, make([]callFrame, frameChunk))
	}
	if a.chunks[a.chunk][a.used].scope.pinned {
		// The pinned frame keeps its chunk; frames still in use there stay valid.
		a.chunks[a.chunk] = make([]callFrame, frameChunk)
	}
	frame := &a.chunks[a.chunk][a.used]
	a.used++
	*frame = callFrame{}
	frame.scope.lent = true
	return frame
}

// pin marks a lent frame so that its arena does not lend it again.
func pin(s *scope) {
	if s != nil && s.lent {
		s.pinned = true
	}
}

func (a *frameArena) localScope(parent *scope, values map[string]value.Value) *scope {
	frame := a.take()
	frame.scope.parent, frame.scope.values = parent, values
	return &frame.scope
}

func (a *frameArena) valueScope(parent *scope, name string, v value.Value) *scope {
	frame := a.take()
	frame.binding[0] = binding{name: name, value: v}
	frame.scope.parent, frame.scope.bound = parent, frame.binding[:]
	return &frame.scope
}

// lazyScope binds the common single lazy parameter of an operator call.
func (a *frameArena) lazyScope(defined *scope, name string, placeholder value.Value, operators map[string]operatorRef, expression *parser.Expression, caller *scope, enabledScopes []string) *scope {
	frame := a.take()
	frame.argument.expression, frame.argument.scope, frame.argument.enabledScopes = expression, caller, enabledScopes
	frame.binding[0] = binding{name: name, value: placeholder, arg: &frame.argument}
	frame.scope.parent, frame.scope.bound, frame.scope.operators = defined, frame.binding[:], operators
	return &frame.scope
}
