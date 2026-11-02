package checker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
)

func TestCheckpointIdentitySourcesAndOptions(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nI == INSTANCE B\nInit == x = 0\nNext == UNCHANGED x\n===="
	cfgSource := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	cfg, err := config.Parse(cfgSource)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{
		"A": "---- MODULE A ----\nEXTENDS C\n====",
		"B": "---- MODULE B ----\nEXTENDS C\nFoo == TRUE\n====",
		"C": "---- MODULE C ----\nBar == TRUE\n====",
	}
	identity := func(root, cfgText string, c config.Config, o Options) checkpointIdentity {
		t.Helper()
		o.Loader = func(_ context.Context, name string) (string, error) {
			s, ok := sources[name]
			if !ok {
				return "", fs.ErrNotExist
			}
			return s, nil
		}
		_, _, id, err := resolveCheckpointIdentity(context.Background(), root, cfgText, c, o)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	base := identity(root, cfgSource, cfg, Options{})
	for range 30 {
		if got := identity(root, cfgSource, cfg, Options{Workers: 1, MaxSetSize: 1000000}); got != base {
			t.Fatal("equivalent inputs differ")
		}
	}
	tests := []struct {
		name   string
		change func() checkpointIdentity
	}{
		{"root", func() checkpointIdentity { return identity(root+"\n", cfgSource, cfg, Options{}) }},
		{"config source", func() checkpointIdentity { return identity(root, cfgSource+"\n", cfg, Options{}) }},
		{"effective override", func() checkpointIdentity {
			c := cfg
			c.CheckDeadlock = true
			return identity(root, cfgSource, c, Options{})
		}},
		{"operator override", func() checkpointIdentity {
			c := cfg
			c.Substitutions = []config.Substitution{{Name: "Foo", Target: "Bar"}}
			return identity(root, cfgSource, c, Options{})
		}},
		{"worker", func() checkpointIdentity { return identity(root, cfgSource, cfg, Options{Workers: 2}) }},
		{"modern TLC statistics", func() checkpointIdentity { return identity(root, cfgSource, cfg, Options{ModernTLCStatistics: true}) }},
	}
	for _, name := range []string{"A", "B", "C"} {
		tests = append(tests, struct {
			name   string
			change func() checkpointIdentity
		}{name, func() checkpointIdentity {
			old := sources[name]
			sources[name] = old + "\n"
			defer func() { sources[name] = old }()
			return identity(root, cfgSource, cfg, Options{})
		}})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.change() == base {
				t.Fatal("changed input has same identity")
			}
		})
	}
}

func TestCheckpointIdentityRejectsInvalidUTF8(t *testing.T) {
	for _, b := range []byte{0xff, 0xfe} {
		bad := string([]byte{b})
		cases := []struct {
			name, root, cfg string
			sources         map[string]string
			effective       config.Config
		}{
			{"model name", bad, "", nil, config.Config{}},
			{"source key", "M", "", map[string]string{bad: "source"}, config.Config{}},
			{"source value", "M", "", map[string]string{"M": bad}, config.Config{}},
			{"config source", "M", bad, nil, config.Config{}},
			{"effective field", "M", "", nil, config.Config{Init: bad}},
			{"effective list", "M", "", nil, config.Config{Invariants: []string{bad}}},
			{"effective nested", "M", "", nil, config.Config{Constants: []config.Assignment{{Value: config.Value{Elements: []config.Value{{Text: bad}}}}}}},
			{"effective args", "M", "", nil, config.Config{Constants: []config.Assignment{{Args: []config.Value{{Kind: bad}}}}}},
			{"effective substitution", "M", "", nil, config.Config{Substitutions: []config.Substitution{{Target: bad}}}},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%x", tc.name, b), func(t *testing.T) {
				_, err := hashCheckpointInputs(tc.root, tc.sources, tc.cfg, tc.effective, Options{})
				if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
					t.Fatalf("invalid input accepted: %v", err)
				}
			})
		}
	}
}

func TestCheckpointInvalidUTF8PublicAPI(t *testing.T) {
	ctx := context.Background()
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugPauseAfterExpansions: 1}
	if run, err := CheckWithCheckpoint(ctx, source, cfg, opts); err != nil || !run.Paused {
		t.Fatalf("create: %v, %+v", err, run)
	}
	for _, resume := range []bool{false, true} {
		for _, b := range []byte{0xff, 0xfe} {
			bad := string([]byte{b})
			for _, tc := range []struct {
				source, cfg string
				effective   *config.Config
			}{
				{source + "\n\\* " + bad, cfg, nil},
				{source, cfg + "\n\\* " + bad, nil},
				{source, cfg + "\nCONSTANT C = \"" + bad + "\"", nil},
				{source, cfg, &config.Config{Init: "Init", Next: "Next", Constants: []config.Assignment{{Name: "C", Value: config.Value{Kind: "string", Text: bad}}}}},
			} {
				opts.Resume, opts.Config = resume, tc.effective
				_, err := CheckWithCheckpoint(ctx, tc.source, tc.cfg, opts)
				if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
					t.Fatalf("resume=%v byte=%x: %v", resume, b, err)
				}
			}
		}
	}
}

func TestCheckpointInvalidUTF8LoadedSource(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	for _, b := range []byte{0xff, 0xfe} {
		for _, resume := range []bool{false, true} {
			opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, Resume: resume}
			opts.Options.Loader = func(context.Context, string) (string, error) {
				return "---- MODULE A ----\n\\* " + string([]byte{b}) + "\n====", nil
			}
			_, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
			if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
				t.Fatalf("resume=%v byte=%x: %v", resume, b, err)
			}
		}
	}
}

func TestCheckpointLegacyUTF8Identity(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n\\* 日本語\n===="
	cfgText := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	cfg, err := config.Parse(cfgText)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the v1 identity independently using the original JSON layout.
	modelBytes, err := json.Marshal(struct {
		Version, Root string
		Sources       []checkpointSource
	}{checkpointIdentityVersion, "M", []checkpointSource{{"M", source}}})
	if err != nil {
		t.Fatal(err)
	}
	configBytes, err := json.Marshal(struct {
		Version, Source string
		Effective       config.Config
		Options         checkpointIdentityOptions
	}{checkpointIdentityVersion, cfgText, cfg, checkpointIdentityOptions{MaxSetSize: evaluator.DefaultSetBound, Workers: 1}})
	if err != nil {
		t.Fatal(err)
	}
	legacy := checkpointIdentity{Model: sha256.Sum256(modelBytes), Config: sha256.Sum256(configBytes)}
	m, err := prepareCheckpointModel(context.Background(), source, cfgText, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if *m.checkpointIdentity != legacy {
		t.Fatal("valid UTF-8 identity changed")
	}
	s := pauseSearch(t, m, 0)
	snapshot, err := s.checkpoint(legacy)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ObservationLog = true
	path := filepath.Join(t.TempDir(), "legacy")
	if err := checkpointWrite(path, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckWithCheckpoint(context.Background(), source, cfgText, CheckpointOptions{Path: path, Resume: true}); err != nil {
		t.Fatalf("legacy recovery: %v", err)
	}
}

func TestCheckpointIdentityFailsClosed(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A\n===="
	cfgText := "CHECK_DEADLOCK FALSE"
	cfg, err := config.Parse(cfgText)
	if err != nil {
		t.Fatal(err)
	}
	for _, loadErr := range []error{fs.ErrNotExist, errors.New("unreadable")} {
		_, _, _, err := resolveCheckpointIdentity(context.Background(), root, cfgText, cfg, Options{Loader: func(context.Context, string) (string, error) { return "", loadErr }})
		if !errors.Is(err, loadErr) {
			t.Fatalf("load error: %v", err)
		}
	}
	called := false
	_, _, _, err = resolveCheckpointIdentity(context.Background(), root, cfgText, cfg, Options{OnPrint: func(string) { called = true }})
	if err == nil || called {
		t.Fatalf("callback: %v, called=%v", err, called)
	}
	_, _, _, err = resolveCheckpointIdentity(context.Background(), "", cfgText, cfg, Options{})
	if err == nil {
		t.Fatal("missing module source accepted")
	}
	empty := "---- MODULE Empty ----\n===="
	_, _, emptyID, err := resolveCheckpointIdentity(context.Background(), empty, "", config.Config{}, Options{})
	if err != nil {
		t.Fatalf("empty config: %v", err)
	}
	_, _, whitespaceID, err := resolveCheckpointIdentity(context.Background(), empty, "\n", config.Config{}, Options{})
	if err != nil || emptyID == whitespaceID {
		t.Fatalf("config source not bound to identity: %v", err)
	}
}

func TestCheckpointRejectsLivenessGraphCallback(t *testing.T) {
	if err := checkpointIdentityOptionsSupported(Options{}); err != nil {
		t.Fatalf("zero options: %v", err)
	}
	callback := func(GraphEvent) error { return nil }
	if err := checkpointIdentityOptionsSupported(Options{OnLivenessGraph: callback}); err == nil {
		t.Fatal("checkpoint accepted an unidentifiable liveness graph callback")
	}
}

func TestCheckpointIdentityBoundToPreparedSearch(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	cfgText := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	cfg, err := config.Parse(cfgText)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareCheckpointModel(context.Background(), source, cfgText, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := pauseSearch(t, m, 0)
	snap, err := s.checkpoint(*m.checkpointIdentity)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := prepareCheckpointModel(context.Background(), source, cfgText, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.restoreCheckpoint(snap, *fresh.checkpointIdentity); err != nil {
		t.Fatal(err)
	}
	changed, err := prepareCheckpointModel(context.Background(), source+"\n", cfgText, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changed.restoreCheckpoint(snap, *changed.checkpointIdentity); err == nil {
		t.Fatal("changed model restored")
	}
	if _, err := changed.restoreCheckpoint(snap, snap.Identity); err == nil {
		t.Fatal("caller supplied identity bypassed binding")
	}
}

func TestCheckpointIdentityDoesNotEvaluateModel(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS A\nASSUME 1 \\div 0 = 0\n===="
	cfgText := "CHECK_DEADLOCK FALSE"
	cfg, err := config.Parse(cfgText)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	loader := func(_ context.Context, name string) (string, error) {
		calls++
		if name != "A" {
			t.Fatalf("unexpected load %q", name)
		}
		return "---- MODULE A ----\n====", nil
	}
	_, _, _, err = resolveCheckpointIdentity(context.Background(), source, cfgText, cfg, Options{Loader: loader})
	if err != nil {
		t.Fatalf("identity evaluated the assumption: %v", err)
	}
	if calls != 1 {
		t.Fatalf("loader called %d times", calls)
	}
}
