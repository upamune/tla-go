package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestFrameArenaReusesReleasedFrames(t *testing.T) {
	var arena frameArena
	mark := arena.mark()
	first := arena.valueScope(nil, "x", value.Bool(true))
	arena.release(mark)
	second := arena.valueScope(nil, "y", value.Bool(false))
	if first != second {
		t.Fatal("released frame was not reused")
	}
	if v, ok := second.value("y"); !ok || v != value.Bool(false) {
		t.Fatalf("reused frame kept stale bindings: %v, %v", v, ok)
	}
	if _, ok := second.value("x"); ok {
		t.Fatal("reused frame kept the previous binding")
	}
}

func TestFrameArenaPinnedFramesAreNotReused(t *testing.T) {
	var arena frameArena
	mark := arena.mark()
	pinned := arena.valueScope(nil, "x", value.Bool(true))
	pin(pinned)
	arena.release(mark)
	next := arena.valueScope(nil, "y", value.Bool(false))
	if next == pinned {
		t.Fatal("pinned frame was reused")
	}
	if v, ok := pinned.value("x"); !ok || v != value.Bool(true) {
		t.Fatalf("pinned frame changed: %v, %v", v, ok)
	}
}

func TestFrameArenaSpansChunks(t *testing.T) {
	var arena frameArena
	mark := arena.mark()
	frames := make([]*scope, 0, frameChunk+1)
	for i := range frameChunk + 1 {
		v, _ := value.Integer(int64(i))
		frames = append(frames, arena.valueScope(nil, "x", v))
	}
	for i, frame := range frames {
		if v, _ := frame.value("x"); v != mustInteger(t, i) {
			t.Fatalf("frame %d holds %v", i, v)
		}
	}
	arena.release(mark)
	if arena.mark() != mark {
		t.Fatalf("release returned to %+v, want %+v", arena.mark(), mark)
	}
}

// Function definitions with operator parameters cache their values by the
// operator's scope address, so a LAMBDA's frame must not be lent again.
func TestOperatorScopesInFunctionKeysArePinned(t *testing.T) {
	e := evaluator{program: &Program{}, frames: &frameArena{}}
	frame := e.frames.valueScope(nil, "k", value.Bool(true))
	d := parser.Definition{Kind: "Function", Name: "f", Params: []parser.Declaration{{Name: "Op", Arity: 1}}}
	ops := map[string]operatorRef{"Op": {arity: 1, lambda: &parser.Expression{Kind: "Lambda"}, scope: frame}}
	if _, err := e.operatorCallKey(d, nil, ops, nil); err != nil {
		t.Fatal(err)
	}
	e.frames.release(frameMark{})
	if next := e.frames.valueScope(nil, "k", value.Bool(false)); next == frame {
		t.Fatal("operator scope in a function key was reused")
	}
}

func TestNilFrameArenaAllocatesFrames(t *testing.T) {
	var arena *frameArena
	mark := arena.mark()
	first := arena.valueScope(nil, "x", value.Bool(true))
	arena.release(mark)
	if second := arena.valueScope(nil, "x", value.Bool(true)); first == second || first.lent {
		t.Fatal("a nil arena lent a frame")
	}
}

func mustInteger(t *testing.T, n int) value.Value {
	t.Helper()
	v, err := value.Integer(int64(n))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
