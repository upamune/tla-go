package checker

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/upamune/tla-go/config"
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
