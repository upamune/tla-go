package checker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/value"
)

func TestCheckpointStringProcess(t *testing.T) {
	if os.Getenv("CHECKPOINT_STRING_CHILD") == "1" {
		source, err := os.ReadFile(os.Getenv("CHECKPOINT_STRING_SOURCE"))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := os.ReadFile(os.Getenv("CHECKPOINT_STRING_CONFIG"))
		if err != nil {
			t.Fatal(err)
		}
		opt := CheckpointOptions{Path: os.Getenv("CHECKPOINT_STRING_PATH"), Interval: time.Hour, Resume: os.Getenv("CHECKPOINT_STRING_MODE") == "resume"}
		if os.Getenv("CHECKPOINT_STRING_MODE") == "pause" {
			opt.DebugPauseAfterExpansions = 2
		}
		run, err := CheckWithCheckpoint(context.Background(), string(source), string(cfg), opt)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(os.Getenv("CHECKPOINT_STRING_RESULT"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = json.NewEncoder(f).Encode(run); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, invariant := range []string{`x # "never"`, `x # "終"`} {
		t.Run(invariant, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nCONSTANT C\nVARIABLE x\nInit == x = C\nNext == (x = \"\" /\\ x' = \"é\") \\/ (x = \"é\" /\\ x' = \"" + strings.Repeat("a", 4096) + "\") \\/ (x = \"" + strings.Repeat("a", 4096) + "\" /\\ x' = \"終\") \\/ (x = \"終\" /\\ UNCHANGED x)\nInv == " + invariant + "\n====\n"
			cfg := "CONSTANT C = \"\" INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE"
			src, conf, path := filepath.Join(dir, "M.tla"), filepath.Join(dir, "M.cfg"), filepath.Join(dir, "snapshot")
			if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf, []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			run := func(mode string) CheckpointRun {
				t.Helper()
				result := filepath.Join(dir, mode+".json")
				cmd := exec.Command(os.Args[0], "-test.run=^TestCheckpointStringProcess$")
				cmd.Env = append(os.Environ(), "CHECKPOINT_STRING_CHILD=1", "CHECKPOINT_STRING_SOURCE="+src, "CHECKPOINT_STRING_CONFIG="+conf, "CHECKPOINT_STRING_PATH="+path, "CHECKPOINT_STRING_RESULT="+result, "CHECKPOINT_STRING_MODE="+mode)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%s: %v %s", mode, err, out)
				}
				b, err := os.ReadFile(result)
				if err != nil {
					t.Fatal(err)
				}
				var v CheckpointRun
				if err := json.Unmarshal(b, &v); err != nil {
					t.Fatal(err)
				}
				return v
			}
			full, paused, resumed := run("full"), run("pause"), run("resume")
			if !paused.Paused || resumed.Paused || !reflect.DeepEqual(full.Result, resumed.Result) || !reflect.DeepEqual(full.Observations, resumed.Observations) {
				t.Fatalf("full=%+v paused=%+v resumed=%+v", full, paused, resumed)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaSource := strings.NewReplacer("é", "e", "終", "end").Replace(source)
				if invariant == `x # "never"` {
					compareJavaTLCModel(t, jar, "M", []byte(javaSource), []byte(cfg), full.Result.GeneratedStates, full.Result.DistinctStates, full.Result.Depth)
				} else {
					cmd := exec.Command("java", "-Xmx256m", "-jar", jar, "-workers", "1", "M.tla")
					cmd.Dir = dir
					if err := os.WriteFile(src, []byte(javaSource), 0o600); err != nil {
						t.Fatal(err)
					}
					output, err := cmd.CombinedOutput()
					if err == nil || !strings.Contains(string(output), "Invariant Inv is violated") || !strings.Contains(string(output), `x = "end"`) {
						t.Fatalf("Java violation: %v %s", err, output)
					}
				}
			}
		})
	}
}

func TestCheckpointStringValues(t *testing.T) {
	for _, s := range []string{"", "日本語 🐈", strings.Repeat("x", 1<<20), string([]byte{0xff, 0})} {
		state, err := checkpointEncodeState(map[string]value.Value{"x": value.String(s)})
		if err != nil {
			t.Fatal(err)
		}
		got, err := checkpointDecodeState(state)
		if err != nil {
			t.Fatal(err)
		}
		text, ok := got["x"].Text()
		if !ok || text != s {
			t.Fatalf("string did not round trip")
		}
	}
}

func TestCheckpointStringRejectsInvalid(t *testing.T) {
	for _, v := range []checkpointValue{
		{Kind: "string", Number: 1},
		{Kind: "string", Boolean: true},
		{Kind: "int", Bytes: []byte("x")},
		{Kind: "bool", Bytes: []byte("x")},
	} {
		if _, err := checkpointDecodeValue(v); err == nil {
			t.Fatalf("accepted invalid value %+v", v)
		}
	}
	source := "---- MODULE M ----\nCONSTANT C\nVARIABLE x\nInit == x = C\nNext == UNCHANGED x\n====\n"
	for _, cfg := range []string{
		`CONSTANT C = 1 INIT Init NEXT Next`,
		`CONSTANT C = {"a"} INIT Init NEXT Next`,
		`CONSTANT C <- Bad INIT Init NEXT Next`,
	} {
		_, err := CheckWithCheckpoint(context.Background(), source, cfg, CheckpointOptions{Path: filepath.Join(t.TempDir(), "c"), Interval: time.Hour})
		if err == nil {
			t.Fatalf("accepted unsupported constant: %s", cfg)
		}
	}
	source = "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\nUnused == PrintT(\"effect\")\nInit == x = \"\"\nNext == UNCHANGED x\n====\n"
	if _, err := CheckWithCheckpoint(context.Background(), source, "INIT Init NEXT Next", CheckpointOptions{Path: filepath.Join(t.TempDir(), "c"), Interval: time.Hour}); err == nil {
		t.Fatal("accepted effectful definition")
	}
}

func TestCheckpointStringOldAndSizeLimit(t *testing.T) {
	// The original v1 representation has no Bytes field. It remains readable.
	snapshot := checkpointFixture()
	data, err := checkpointEncode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"Bytes"`) {
		t.Fatal("changed v1 scalar encoding")
	}
	decoded, err := checkpointDecode(data, snapshot.Identity)
	if err != nil || !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("old snapshot: %v", err)
	}
	path := filepath.Join(t.TempDir(), "oversized")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 16)
	copy(header, checkpointMagic)
	binary.BigEndian.PutUint64(header[8:], checkpointMaxSize+1)
	if _, err := f.Write(header); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(checkpointMaxSize + 16 + 32 + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := checkpointRead(path, snapshot.Identity); err == nil {
		t.Fatal("accepted oversized checkpoint")
	}
	// Base64 encoding expands this string beyond the 64 MiB payload cap.
	snapshot.Nodes[0].State["x"] = value.String(strings.Repeat("x", 49<<20))
	if _, err := checkpointEncode(snapshot); err == nil {
		t.Fatal("encoded oversized checkpoint")
	}
}

func TestCheckpointStringTamperedStateFailsRecovery(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = \"\"\nNext == x' = \"next\"\n====\n"
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	path := filepath.Join(t.TempDir(), "checkpoint")
	opts := CheckpointOptions{Path: path, Interval: time.Hour, DebugPauseAfterExpansions: 1}
	if run, err := CheckWithCheckpoint(context.Background(), source, cfg, opts); err != nil || !run.Paused {
		t.Fatalf("pause: %+v %v", run, err)
	}
	parsed, err := checkpointEffectiveConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, identity, err := resolveCheckpointIdentity(context.Background(), source, cfg, parsed, Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := checkpointRead(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Nodes[0].State["x"] = value.String("forged")
	if err := checkpointWrite(path, snapshot); err != nil {
		t.Fatal(err)
	}
	opts.Resume = true
	if _, err := CheckWithCheckpoint(context.Background(), source, cfg, opts); err == nil {
		t.Fatal("accepted checksummed forged state")
	}
}
