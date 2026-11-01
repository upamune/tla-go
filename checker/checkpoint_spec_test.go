package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// The model and configuration are from CheckpointWhenTimeBoundTest at
// tlaplus/tlaplus commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
const upstreamCheckpointSpec = `---- MODULE InfiniteStateSpace ----
EXTENDS Naturals
VARIABLES x,y
Spec == x = 0 /\ y \in 1..1000 /\ [][x' = x + 1 /\ UNCHANGED y]_<<x,y>>
====
`

func TestCheckpointUpstreamInfiniteSpec(t *testing.T) {
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugPauseAfterExpansions: 1}
	cfg := "SPECIFICATION Spec\nCHECK_DEADLOCK FALSE"
	first, err := CheckWithCheckpoint(context.Background(), upstreamCheckpointSpec, cfg, opts)
	if err != nil || !first.Paused {
		t.Fatalf("first checkpoint: paused=%t err=%v", first.Paused, err)
	}
	parsed, err := config.Parse(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareCheckpointModel(context.Background(), upstreamCheckpointSpec, cfg, parsed, Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := checkpointRead(opts.Path, *m.checkpointIdentity)
	if err != nil || snapshot.Counters.Initial != 1000 || snapshot.Head != 1 {
		t.Fatalf("first snapshot: initial=%d head=%d err=%v", snapshot.Counters.Initial, snapshot.Head, err)
	}
	opts.Resume = true
	opts.DebugPauseAfterExpansions = 2
	second, err := CheckWithCheckpoint(context.Background(), upstreamCheckpointSpec, cfg, opts)
	if err != nil || !second.Paused || len(second.Observations) < len(first.Observations) {
		t.Fatalf("recovery: paused=%t err=%v", second.Paused, err)
	}
}

func TestCheckpointSpecProcess(t *testing.T) {
	if os.Getenv("CHECKPOINT_SPEC_CHILD") == "1" {
		source, err := os.ReadFile(os.Getenv("CHECKPOINT_SPEC_SOURCE"))
		if err != nil {
			t.Fatal(err)
		}
		opts := CheckpointOptions{Path: os.Getenv("CHECKPOINT_SPEC_PATH"), Interval: time.Hour, Resume: os.Getenv("CHECKPOINT_SPEC_MODE") == "resume"}
		if os.Getenv("CHECKPOINT_SPEC_MODE") == "pause" {
			opts.DebugPauseAfterExpansions = 2
		}
		cfg, err := os.ReadFile(os.Getenv("CHECKPOINT_SPEC_CONFIG"))
		if err != nil {
			t.Fatal(err)
		}
		run, err := CheckWithCheckpoint(context.Background(), string(source), string(cfg), opts)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("CHECKPOINT_SPEC_RESULT"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	source := `---- MODULE InfiniteStateSpace ----
EXTENDS Naturals
VARIABLES x,y
Zero == 0
Increment(n) == n + 1
Init == x = Zero /\ y \in 1..3
Next == x < 3 /\ x' = Increment(x) /\ UNCHANGED y
Spec == Init /\ [][Next]_<<x,y>>
====
`
	src, path := filepath.Join(dir, "InfiniteStateSpace.tla"), filepath.Join(dir, "checkpoint")
	configSource, configPath := "SPECIFICATION Spec\nCHECK_DEADLOCK FALSE", filepath.Join(dir, "InfiniteStateSpace.cfg")
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(configSource), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(mode string, fail bool) CheckpointRun {
		t.Helper()
		result := filepath.Join(dir, mode+".json")
		cmd := exec.Command(os.Args[0], "-test.run=^TestCheckpointSpecProcess$")
		cmd.Env = append(os.Environ(), "CHECKPOINT_SPEC_CHILD=1", "CHECKPOINT_SPEC_SOURCE="+src, "CHECKPOINT_SPEC_CONFIG="+configPath, "CHECKPOINT_SPEC_PATH="+path, "CHECKPOINT_SPEC_RESULT="+result, "CHECKPOINT_SPEC_MODE="+mode)
		out, err := cmd.CombinedOutput()
		if fail {
			if err == nil {
				t.Fatal("recovery with changed source succeeded")
			}
			return CheckpointRun{}
		}
		if err != nil {
			t.Fatalf("%s: %v %s", mode, err, out)
		}
		data, err := os.ReadFile(result)
		if err != nil {
			t.Fatal(err)
		}
		var got CheckpointRun
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	full, paused := run("full", false), run("pause", false)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(source+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("resume", true)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed recovery changed checkpoint: %v", err)
	}
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(configSource+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("resume", true)
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed recovery with changed configuration modified checkpoint: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(configSource), 0o600); err != nil {
		t.Fatal(err)
	}
	resumed := run("resume", false)
	if !paused.Paused || resumed.Paused || !reflect.DeepEqual(full.Result, resumed.Result) || !reflect.DeepEqual(full.Observations, resumed.Observations) {
		t.Fatalf("full=%+v paused=%+v resumed=%+v", full, paused, resumed)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "InfiniteStateSpace", []byte(source), []byte("SPECIFICATION Spec\nCHECK_DEADLOCK FALSE"), full.Result.GeneratedStates, full.Result.DistinctStates, full.Result.Depth)
	}
}

func TestCheckpointNamedDefinitionPreflight(t *testing.T) {
	tests := []struct {
		name, definitions, spec string
		accept                  bool
	}{
		{"aliases", "A == B\nB == x = 0\nN == x' = Step(x)\nStep(n) == n + 1", "A /\\ [][N]_x", true},
		{"cycle", "A == B\nB == A\nN == x' = x + 1", "A /\\ [][N]_x", false},
		{"recursive", "RECURSIVE F(_)\nF(n) == F(n)\nN == x' = x + 1", "F(x) /\\ [][N]_x", false},
		{"wrong arity", "A(n) == n = 0\nN == x' = x + 1", "A /\\ [][N]_x", false},
		{"too many arguments", "A(n) == n = 0\nN == x' = x + 1", "A(x, x) /\\ [][N]_x", false},
		{"operator parameter", "A(F(_), n) == F(n)\nN == x' = x + 1", "A(N, x) /\\ [][N]_x", false},
		{"effect via alias", "A == B\nB == PrintT(\"effect\")\nN == x' = x + 1", "A /\\ [][N]_x", false},
		{"unused effect", "A == x = 0\nN == x' = x + 1\nUnused == PrintT(\"effect\")", "A /\\ [][N]_x", false},
		{"fairness", "A == x = 0\nN == x' = x + 1\nFair == WF_x(N)", "A /\\ [][N]_x /\\ Fair", false},
		{"action initial", "A == B\nB == x' = x\nN == x' = x + 1", "A /\\ [][N]_x", false},
		{"effect argument", "A(n) == n = 0\nN == x' = x + 1", "A(PrintT(\"effect\")) /\\ [][N]_x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\n" + tt.definitions + "\nSpec == " + tt.spec + "\n====\n"
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Config{Specification: "Spec"}
			err = checkpointPreflight(module, map[string]*parser.Module{"M": module}, cfg)
			if (err == nil) != tt.accept {
				t.Fatalf("accepted=%t error=%v", err == nil, err)
			}
		})
	}
}

func TestCheckpointSpecRejectsEffectsAndLiveness(t *testing.T) {
	for _, spec := range []string{
		`x = 0 /\ [][x' = x + 1]_x /\ WF_x(x' = x + 1)`,
		`x = 0 /\ [][x' = x + 1]_x /\ <> (x = 2)`,
		`x = 0 /\ [][x' = x + PrintT("effect")]_x`,
		`x = 0 /\ [][x' = x + 1]_x /\ [][x' = x + 2]_x`,
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nSpec == " + spec + "\n====\n"
		path := filepath.Join(t.TempDir(), "checkpoint")
		_, err := CheckWithCheckpoint(context.Background(), source, "SPECIFICATION Spec", CheckpointOptions{Path: path, Interval: time.Hour})
		if err == nil {
			t.Fatalf("accepted %s", spec)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("wrote checkpoint for %s: %v", spec, err)
		}
	}
}
