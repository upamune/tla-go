package checker

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func checkpointFixture() checkpointSnapshot {
	n, _ := value.Integer(-2147483648)
	return checkpointSnapshot{
		Identity: checkpointIdentity{Model: sha256.Sum256([]byte("model")), Config: sha256.Sum256([]byte("config"))}, Head: 1,
		Nodes:   []checkpointNode{{State: evaluator.State{"x": n, "b": value.Bool(false)}, Parent: -1, Depth: 1}, {State: evaluator.State{"x": n, "b": value.Bool(true)}, Parent: 0, Action: "Next", ActionRange: parser.Range{Module: "M", Start: parser.Position{Line: 2, Column: 3}, End: parser.Position{Line: 2, Column: 9}}, Depth: 2}},
		Visited: map[string]bool{"\xff\x00": true, "second": true}, Counters: checkpointCounters{Generated: 4, Distinct: 2, Queued: 1, Depth: 2, Initial: 1, OutDegrees: [32]int{0, 1}},
	}
}

func TestCheckpointRejectsImpossibleExpansionBoundary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*checkpointSnapshot)
	}{
		{"unexpanded parent", func(s *checkpointSnapshot) { s.Head = 0; s.Counters.Queued = 2; s.Counters.OutDegrees = [32]int{} }},
		{"missing outdegree sample", func(s *checkpointSnapshot) { s.Counters.OutDegrees = [32]int{} }},
		{"extra outdegree sample", func(s *checkpointSnapshot) { s.Counters.OutDegrees[0]++ }},
		{"too few generated", func(s *checkpointSnapshot) { s.Counters.Generated = 1 }},
		{"wrong depth", func(s *checkpointSnapshot) { s.Counters.Depth = 3 }},
		{"depth below deepest node", func(s *checkpointSnapshot) { s.Counters.Depth = 1 }},
		{"too few initial", func(s *checkpointSnapshot) { s.Counters.Initial = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := checkpointFixture()
			tc.mutate(&s)
			if _, err := checkpointEncode(s); err == nil {
				t.Fatal("accepted impossible expansion boundary")
			}
		})
	}
}

func TestCheckpointRoundTrip(t *testing.T) {
	s := checkpointFixture()
	data, err := checkpointEncode(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := checkpointDecode(data, s.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("round trip: got %#v, want %#v", got, s)
	}
	again, err := checkpointEncode(s)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("encoding is not deterministic", err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint")
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	got, err = checkpointRead(path, s.Identity)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("file round trip: %v", err)
	}
}

func TestCheckpointEmptySearchDepth(t *testing.T) {
	identity := checkpointFixture().Identity
	for _, tc := range []struct {
		name             string
		generated, depth int
		valid            bool
	}{
		{"before initial generation", 0, 0, true},
		{"completed without initial states", 0, 1, true},
		{"excluded initial states", 1, 1, true},
		{"generated at depth zero", 1, 0, false},
		{"negative depth", 0, -1, false},
		{"depth beyond initial level", 0, 2, false},
		{"excluded states beyond initial level", 1, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := checkpointSnapshot{Identity: identity, Nodes: []checkpointNode{}, Visited: map[string]bool{}, Counters: checkpointCounters{Generated: tc.generated, Initial: tc.generated, Depth: tc.depth}}
			data, err := checkpointEncode(s)
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted invalid depth")
				}
				if _, err := checkpointFromWire(checkpointWire{Identity: identity, Counters: s.Counters}); err == nil {
					t.Fatal("decoded invalid depth")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := checkpointDecode(data, identity)
			if err != nil || !reflect.DeepEqual(got, s) {
				t.Fatalf("round trip: got %+v, error %v; want %+v", got, err, s)
			}
		})
	}
}

func TestCheckpointEmptySearchCheckerEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, init, constraint string
		generated              int
	}{
		{"no initial states", `x \in {}`, "", 0},
		{"excluded initial state", "x = 0", "CONSTRAINT Exclude", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == " + tc.init + "\nNext == UNCHANGED x\nExclude == x # 0\n====\n"
			cfg := "INIT Init NEXT Next " + tc.constraint + " CHECK_DEADLOCK FALSE\n"
			result, err := Check(source, cfg)
			if err != nil || result.Status != Success || result.GeneratedStates != tc.generated || result.DistinctStates != 0 || result.QueuedStates != 0 || result.Depth != 1 {
				t.Fatalf("checker: %+v, %v", result, err)
			}
			s := checkpointSnapshot{Identity: checkpointFixture().Identity, Nodes: []checkpointNode{}, Visited: map[string]bool{}, Counters: checkpointCounters{Generated: result.GeneratedStates, Initial: tc.generated, Depth: result.Depth}}
			data, err := checkpointEncode(s)
			if err != nil {
				t.Fatal(err)
			}
			got, err := checkpointDecode(data, s.Identity)
			if err != nil || !reflect.DeepEqual(got, s) {
				t.Fatalf("round trip: got %+v, error %v; want %+v", got, err, s)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), tc.generated, 0, 1)
			}
		})
	}
}

func TestCheckpointRejectsInvalid(t *testing.T) {
	s := checkpointFixture()
	data, err := checkpointEncode(s)
	if err != nil {
		t.Fatal(err)
	}
	for i := range len(data) {
		if _, err := checkpointDecode(data[:i], s.Identity); err == nil {
			t.Fatalf("accepted truncation at %d", i)
		}
	}
	for _, i := range []int{0, 8, 16, len(data) - 1} {
		broken := bytes.Clone(data)
		broken[i] ^= 0x80
		if _, err := checkpointDecode(broken, s.Identity); err == nil {
			t.Fatalf("accepted corruption at %d", i)
		}
	}
	wrong := s.Identity
	wrong.Config[0] ^= 1
	if _, err := checkpointDecode(data, wrong); err == nil {
		t.Fatal("accepted different config")
	}
	wrong = s.Identity
	wrong.Model[0] ^= 1
	if _, err := checkpointDecode(data, wrong); err == nil {
		t.Fatal("accepted different model")
	}
	s.Nodes[0].State["x"] = value.Model("unsupported")
	if _, err := checkpointEncode(s); err == nil {
		t.Fatal("accepted unsupported value")
	}
	s = checkpointFixture()
	s.Nodes[1].Parent = 1
	if _, err := checkpointEncode(s); err == nil {
		t.Fatal("accepted invalid parent")
	}
	s = checkpointFixture()
	s.Head = 2
	if _, err := checkpointEncode(s); err == nil {
		t.Fatal("accepted inconsistent queue")
	}
}

func TestCheckpointInterruptedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	s := checkpointFixture()
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("interrupted")
	err := checkpointAtomicWrite(path, func(f *os.File) error { _, _ = f.Write([]byte("partial")); return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
	got, err := checkpointRead(path, s.Identity)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("previous checkpoint lost: %v", err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file left behind: %v %v", files, err)
	}
}

func TestCheckpointReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	first := checkpointFixture()
	if err := checkpointWrite(path, first); err != nil {
		t.Fatal(err)
	}
	second := checkpointFixture()
	second.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, second); err != nil {
		t.Fatal(err)
	}
	got, err := checkpointRead(path, second.Identity)
	if err != nil || !reflect.DeepEqual(got, second) {
		t.Fatalf("replacement: got %+v, error %v", got, err)
	}
}

func TestCheckpointConcurrentReadersAndWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	s := checkpointFixture()
	other := checkpointFixture()
	other.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i := range 30 {
				snapshot := s
				if i%2 != 0 {
					snapshot = other
				}
				if err := checkpointWrite(path, snapshot); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range 100 {
				got, err := checkpointRead(path, s.Identity)
				if err != nil || (!reflect.DeepEqual(got, s) && !reflect.DeepEqual(got, other)) {
					t.Errorf("read: %v", err)
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestCheckpointRejectsIncompatiblePayload(t *testing.T) {
	s := checkpointFixture()
	data, err := checkpointEncode(s)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := checkpointPayload(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []struct{ old, new string }{
		{`"Kind":"int"`, `"Kind":"set"`},
		{`"Head":1`, `"Head":99`},
		{`"Head":1`, `"Head":1,"FutureField":true`},
	} {
		changed := bytes.Replace(payload, []byte(replacement.old), []byte(replacement.new), 1)
		if bytes.Equal(changed, payload) {
			t.Fatal("fixture did not match", replacement.old)
		}
		header := make([]byte, 16)
		copy(header, checkpointMagic)
		binary.BigEndian.PutUint64(header[8:], uint64(len(changed)))
		altered := append(header, changed...)
		sum := sha256.Sum256(altered)
		altered = append(altered, sum[:]...)
		if _, err := checkpointDecode(altered, s.Identity); err == nil {
			t.Fatalf("accepted %s", replacement.new)
		}
	}
}
