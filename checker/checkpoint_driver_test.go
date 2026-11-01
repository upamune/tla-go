package checker

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/config"
)

func TestCheckpointDriverResume(t *testing.T) {
	for _, invariant := range []string{"x <= 4", "x < 4"} {
		t.Run(invariant, func(t *testing.T) {
			source := observerSource[:len(observerSource)-4] + "Inv == " + invariant + "\n===="
			cfgText := observerConfig + " INVARIANT Inv"
			cfg, err := config.Parse(cfgText)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "checkpoint")
			want, observations, paused, err := runCheckpointSearch(context.Background(), source, cfgText, cfg, Options{}, path, 2, 0, false)
			if err != nil || paused {
				t.Fatalf("full: %v %v", paused, err)
			}
			var events []SearchEvent
			ordinary, err := CheckWithOptions(context.Background(), source, cfgText, Options{OnSearch: func(e SearchEvent) { events = append(events, e) }})
			if err != nil || !reflect.DeepEqual(ordinary, want) {
				t.Fatalf("ordinary result: %+v, error %v; checkpoint result: %+v", ordinary, err, want)
			}
			var recorded []SearchEvent
			for _, observation := range observations {
				if observation.searchEvent != nil {
					recorded = append(recorded, *observation.searchEvent)
				}
			}
			if !reflect.DeepEqual(recorded, events) {
				t.Fatalf("ordinary events: %v, checkpoint events: %v", events, recorded)
			}
			_, first, paused, err := runCheckpointSearch(context.Background(), source, cfgText, cfg, Options{}, path, 1, 1, false)
			if err != nil || !paused {
				t.Fatalf("pause: %v %v", paused, err)
			}
			_, middle, paused, err := runCheckpointSearch(context.Background(), source, cfgText, cfg, Options{}, path, 1, 2, true)
			if err != nil || !paused {
				t.Fatalf("second pause: %v %v", paused, err)
			}
			got, last, paused, err := runCheckpointSearch(context.Background(), source, cfgText, cfg, Options{}, path, 1, 0, true)
			if err != nil || paused {
				t.Fatalf("resume: %v %v", paused, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("result: got %+v, want %+v", got, want)
			}
			combined := append(append(first, middle...), last...)
			if !reflect.DeepEqual(combined, observations) {
				t.Fatalf("observations: got %+v, want %+v", combined, observations)
			}
		})
	}
}

func TestCheckpointDriverRejectsMismatchAndErrors(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint")
	source := observerSource
	options := Options{}
	run := func(src, cfgText string, c config.Config, o Options, p string, resume bool) error {
		_, _, _, e := runCheckpointSearch(ctx, src, cfgText, c, o, p, 1, 1, resume)
		return e
	}
	if err := run(source, observerConfig, cfg, options, path, false); err != nil {
		t.Fatal(err)
	}
	changed := cfg
	changed.CheckDeadlock = true
	for name, fn := range map[string]func() error{
		"model":            func() error { return run(source+"\n", observerConfig, cfg, options, path, true) },
		"config source":    func() error { return run(source, observerConfig+"\n", cfg, options, path, true) },
		"effective config": func() error { return run(source, observerConfig, changed, options, path, true) },
		"options":          func() error { o := options; o.Seed++; return run(source, observerConfig, cfg, o, path, true) },
		"coverage":         func() error { o := options; o.Coverage = true; return run(source, observerConfig, cfg, o, path, true) },
		"missing":          func() error { return run(source, observerConfig, cfg, options, path+"-missing", true) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := fn(); err == nil {
				t.Fatal("accepted invalid recovery")
			}
		})
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(source, observerConfig, cfg, options, path, true); err == nil {
		t.Fatal("accepted corruption")
	}
	if err := run(source, observerConfig, cfg, options, filepath.Join(t.TempDir(), "missing", "child"), false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("write error not propagated: %v", err)
	}
}

func TestCheckpointDriverLastExpansion(t *testing.T) {
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint")
	want, _, _, err := runCheckpointSearch(context.Background(), observerSource, observerConfig, cfg, Options{}, path, 2, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, paused, err := runCheckpointSearch(context.Background(), observerSource, observerConfig, cfg, Options{}, path, 5, 5, false)
	if err != nil || !paused {
		t.Fatalf("last expansion: paused=%v error=%v", paused, err)
	}
	got, _, paused, err := runCheckpointSearch(context.Background(), observerSource, observerConfig, cfg, Options{}, path, 5, 0, true)
	if err != nil || paused || !reflect.DeepEqual(got, want) {
		t.Fatalf("completed snapshot: %+v, paused=%v error=%v", got, paused, err)
	}
}

// Dependencies are currently outside the supported search language. Verify a
// changed dependency is refused by the driver and by the identity comparison.
func TestCheckpointDriverDependencyMismatch(t *testing.T) {
	ctx := context.Background()
	source := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	dependency := "---- MODULE A ----\n===="
	o := Options{Loader: func(_ context.Context, name string) (string, error) {
		if name == "A" {
			return dependency, nil
		}
		return "", fs.ErrNotExist
	}}
	_, _, identity, err := resolveCheckpointIdentity(ctx, source, observerConfig, cfg, o)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := checkpointFixture()
	snapshot.Identity = identity
	path := filepath.Join(t.TempDir(), "checkpoint")
	if err := checkpointWrite(path, snapshot); err != nil {
		t.Fatal(err)
	}
	dependency += "\n"
	_, _, changed, err := resolveCheckpointIdentity(ctx, source, observerConfig, cfg, o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkpointRead(path, changed); err == nil {
		t.Fatal("accepted changed dependency identity")
	}
	if _, _, _, err := runCheckpointSearch(ctx, source, observerConfig, cfg, o, path, 1, 0, true); err == nil {
		t.Fatal("accepted unsupported dependency")
	}
}
