package checker

// This codec is deliberately private and is not connected to a running search.
// A caller must freeze the search at an expansion boundary before constructing
// a snapshot. It must also identify all model dependencies and relevant options.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

const (
	checkpointMagic   = "TLAGOCK1"
	checkpointMaxSize = 64 << 20
)

type (
	checkpointIdentity struct{ Model, Config [32]byte }
	checkpointNode     struct {
		State       evaluator.State
		Parent      int
		Action      string
		ActionRange parser.Range
		Depth       int
	}
)

type checkpointCounters struct {
	Generated, Distinct, Queued, Depth, Initial int
	OutDegrees                                  [32]int
}
type checkpointSnapshot struct {
	Identity checkpointIdentity
	Nodes    []checkpointNode
	Head     int
	// Keys are the exact keys in search.visited, including binary state keys.
	Visited  map[string]bool
	Counters checkpointCounters
}
type checkpointValue struct {
	Kind    string
	Number  int32
	Boolean bool
}
type checkpointWireNode struct {
	State       map[string]checkpointValue
	Parent      int
	Action      string
	ActionRange parser.Range
	Depth       int
}
type checkpointWire struct {
	Identity checkpointIdentity
	Nodes    []checkpointWireNode
	Head     int
	Visited  [][]byte
	Counters checkpointCounters
}

func checkpointEncode(s checkpointSnapshot) ([]byte, error) {
	if err := validateCheckpoint(s); err != nil {
		return nil, err
	}
	w := checkpointWire{Identity: s.Identity, Head: s.Head, Visited: checkpointKeys(s.Visited), Counters: s.Counters, Nodes: make([]checkpointWireNode, len(s.Nodes))}
	for i, node := range s.Nodes {
		state, err := checkpointEncodeState(node.State)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", i, err)
		}
		w.Nodes[i] = checkpointWireNode{State: state, Parent: node.Parent, Action: node.Action, ActionRange: node.ActionRange, Depth: node.Depth}
	}
	payload, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	if len(payload) > checkpointMaxSize {
		return nil, fmt.Errorf("checkpoint exceeds size limit")
	}
	data := make([]byte, 16, 16+len(payload)+sha256.Size)
	copy(data, checkpointMagic)
	binary.BigEndian.PutUint64(data[8:], uint64(len(payload)))
	data = append(data, payload...)
	sum := sha256.Sum256(data)
	return append(data, sum[:]...), nil
}

func checkpointKeys(visited map[string]bool) [][]byte {
	keys := make([]string, 0, len(visited))
	for key := range visited {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([][]byte, len(keys))
	for i, key := range keys {
		out[i] = []byte(key)
	}
	return out
}

func checkpointEncodeState(state evaluator.State) (map[string]checkpointValue, error) {
	if state == nil {
		return nil, fmt.Errorf("nil state")
	}
	out := make(map[string]checkpointValue, len(state))
	for name, v := range state {
		if n, ok := v.Int(); ok {
			out[name] = checkpointValue{Kind: "int", Number: n}
			continue
		}
		if b, ok := v.Bool(); ok {
			out[name] = checkpointValue{Kind: "bool", Boolean: b}
			continue
		}
		return nil, fmt.Errorf("unsupported value for %q (kind %d)", name, v.Kind())
	}
	return out, nil
}

func checkpointDecode(data []byte, identity checkpointIdentity) (checkpointSnapshot, error) {
	payload, err := checkpointPayload(data)
	if err != nil {
		return checkpointSnapshot{}, err
	}
	var w checkpointWire
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return checkpointSnapshot{}, fmt.Errorf("invalid checkpoint payload: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return checkpointSnapshot{}, fmt.Errorf("trailing checkpoint payload")
	}
	if w.Identity != identity || identity.Model == [32]byte{} || identity.Config == [32]byte{} {
		return checkpointSnapshot{}, fmt.Errorf("checkpoint identity mismatch")
	}
	return checkpointFromWire(w)
}

func checkpointPayload(data []byte) ([]byte, error) {
	if len(data) < 16+sha256.Size || string(data[:8]) != checkpointMagic {
		return nil, fmt.Errorf("invalid checkpoint header")
	}
	size := binary.BigEndian.Uint64(data[8:16])
	if size > checkpointMaxSize || size != uint64(len(data)-16-sha256.Size) {
		return nil, fmt.Errorf("invalid checkpoint length")
	}
	sum := sha256.Sum256(data[:len(data)-sha256.Size])
	if !bytes.Equal(sum[:], data[len(data)-sha256.Size:]) {
		return nil, fmt.Errorf("checkpoint checksum mismatch")
	}
	return data[16 : len(data)-sha256.Size], nil
}

func checkpointFromWire(w checkpointWire) (checkpointSnapshot, error) {
	var empty checkpointSnapshot
	s := checkpointSnapshot{Identity: w.Identity, Head: w.Head, Visited: make(map[string]bool, len(w.Visited)), Counters: w.Counters, Nodes: make([]checkpointNode, len(w.Nodes))}
	for _, key := range w.Visited {
		if s.Visited[string(key)] {
			return empty, fmt.Errorf("duplicate visited key")
		}
		s.Visited[string(key)] = true
	}
	for i, node := range w.Nodes {
		state, err := checkpointDecodeState(node.State)
		if err != nil {
			return empty, fmt.Errorf("node %d: %w", i, err)
		}
		s.Nodes[i] = checkpointNode{State: state, Parent: node.Parent, Action: node.Action, ActionRange: node.ActionRange, Depth: node.Depth}
	}
	if err := validateCheckpoint(s); err != nil {
		return empty, err
	}
	return s, nil
}

func checkpointDecodeState(state map[string]checkpointValue) (evaluator.State, error) {
	if state == nil {
		return nil, fmt.Errorf("nil state")
	}
	out := make(evaluator.State, len(state))
	for name, v := range state {
		decoded, err := checkpointDecodeValue(v)
		if err != nil {
			return nil, err
		}
		out[name] = decoded
	}
	return out, nil
}

func checkpointDecodeValue(v checkpointValue) (value.Value, error) {
	switch v.Kind {
	case "int":
		if v.Boolean {
			return value.Value{}, fmt.Errorf("invalid integer")
		}
		return value.Integer(int64(v.Number))
	case "bool":
		if v.Number != 0 {
			return value.Value{}, fmt.Errorf("invalid boolean")
		}
		return value.Bool(v.Boolean), nil
	default:
		return value.Value{}, fmt.Errorf("unsupported value kind %q", v.Kind)
	}
}

func validateCheckpoint(s checkpointSnapshot) error {
	if err := validateCheckpointIdentity(s.Identity); err != nil {
		return err
	}
	if err := validateCheckpointQueue(s); err != nil {
		return err
	}
	if s.Visited == nil {
		return fmt.Errorf("invalid queue or visited set")
	}
	if err := validateCheckpointCounters(s); err != nil {
		return err
	}
	if err := validateCheckpointVisited(s.Visited); err != nil {
		return err
	}
	return validateCheckpointNodes(s)
}

func validateCheckpointNodes(s checkpointSnapshot) error {
	roots, depth, lastParent := 0, 0, -1
	for i, n := range s.Nodes {
		if err := validateCheckpointNode(s.Nodes, i, n); err != nil {
			return err
		}
		if err := validateCheckpointParent(n.Parent, lastParent, s.Head); err != nil {
			return err
		}
		if n.Parent < 0 {
			roots++
		} else {
			lastParent = n.Parent
		}
		depth = max(depth, n.Depth)
	}
	// An empty search starts at depth zero. Visiting even an excluded initial
	// state or finishing successfully without initial states reports depth one.
	if roots > s.Counters.Initial || !validCheckpointDepth(s, depth) {
		return fmt.Errorf("invalid checkpoint depth or initial count")
	}
	return nil
}

func validCheckpointDepth(s checkpointSnapshot, depth int) bool {
	if len(s.Nodes) == 0 {
		return s.Counters.Depth == 1 || s.Counters.Depth == 0 && s.Counters.Generated == 0
	}
	return depth == s.Counters.Depth
}

func validateCheckpointParent(parent, lastParent, head int) error {
	if parent < lastParent || parent >= head {
		return fmt.Errorf("invalid breadth-first parent order")
	}
	return nil
}

func validateCheckpointQueue(s checkpointSnapshot) error {
	if s.Head < 0 || s.Head > len(s.Nodes) {
		return fmt.Errorf("invalid queue")
	}
	return nil
}

func validateCheckpointIdentity(identity checkpointIdentity) error {
	if identity.Model == [32]byte{} || identity.Config == [32]byte{} {
		return fmt.Errorf("missing checkpoint identity")
	}
	return nil
}

func validateCheckpointVisited(visited map[string]bool) error {
	for _, v := range visited {
		if !v {
			return fmt.Errorf("invalid visited entry")
		}
	}
	return nil
}

func validateCheckpointCounters(s checkpointSnapshot) error {
	c := s.Counters
	if c.Generated < c.Distinct || c.Initial < 0 || c.Initial > c.Generated || c.Depth < 0 {
		return fmt.Errorf("invalid checkpoint counters")
	}
	if c.Distinct != len(s.Nodes) || c.Queued != len(s.Nodes)-s.Head || len(s.Visited) != len(s.Nodes) {
		return fmt.Errorf("invalid checkpoint counters")
	}
	return validateCheckpointOutDegrees(c.OutDegrees, s.Head)
}

func validateCheckpointOutDegrees(counts [32]int, head int) error {
	if err := validateCheckpointNonnegativeOutDegrees(counts); err != nil {
		return err
	}
	// A completed expansion contributes exactly one outdegree sample.
	samples := 0
	for _, count := range counts {
		if count > head-samples {
			return fmt.Errorf("invalid checkpoint outdegree count")
		}
		samples += count
	}
	if samples != head {
		return fmt.Errorf("invalid checkpoint outdegree count")
	}
	return nil
}

func validateCheckpointNonnegativeOutDegrees(counts [32]int) error {
	for _, count := range counts {
		if count < 0 {
			return fmt.Errorf("invalid outdegree")
		}
	}
	return nil
}

func validateCheckpointNode(nodes []checkpointNode, i int, n checkpointNode) error {
	if n.State == nil || n.Parent < -1 || n.Parent >= i {
		return fmt.Errorf("invalid node %d", i)
	}
	return validateCheckpointDepth(nodes, n)
}

func validateCheckpointDepth(nodes []checkpointNode, n checkpointNode) error {
	if n.Depth < 1 || (n.Parent == -1 && n.Depth != 1) {
		return fmt.Errorf("invalid root depth")
	}
	if n.Parent >= 0 && n.Depth != nodes[n.Parent].Depth+1 {
		return fmt.Errorf("invalid child depth")
	}
	return nil
}

func checkpointRead(path string, identity checkpointIdentity) (checkpointSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return checkpointSnapshot{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, checkpointMaxSize+16+sha256.Size+1))
	if err != nil {
		return checkpointSnapshot{}, err
	}
	return checkpointDecode(data, identity)
}

func checkpointWrite(path string, s checkpointSnapshot) error {
	data, err := checkpointEncode(s)
	if err != nil {
		return err
	}
	return checkpointAtomicWrite(path, func(f *os.File) error { _, err := f.Write(data); return err })
}

func checkpointAtomicWrite(path string, write func(*os.File) error) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tla-checkpoint-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = write(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return checkpointSyncDir(dir)
}
