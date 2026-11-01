package upstreamjunit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Invoked only by the adapter while the original upstream @Before is running.
func TestOriginalEmptyGoHelper(t *testing.T) {
	if os.Getenv("TLA_GO_ORIGINAL_EMPTY_HELPER") != "1" {
		return
	}
	if err := originalEmptyGoHelper(); err != nil {
		t.Fatal(err)
	}
}

func originalEmptyGoHelper() error {
	binary, model, mode := os.Getenv("TLA_GO_ORIGINAL_EMPTY_BINARY"), os.Getenv("TLA_GO_ORIGINAL_EMPTY_MODEL"), os.Getenv("TLA_GO_ORIGINAL_EMPTY_MODE")
	if binary == "" || model == "" {
		return fmt.Errorf("missing Go binary or model directory")
	}
	result, observed, err := boundedOriginalEmptyRunWithRecorder(binary, model)
	if err != nil {
		return err
	}
	if mode == "missing-success" {
		observed = slices.DeleteFunc(observed, func(event recorderdebug.Event) bool { return event.Code == 2193 })
	}
	if mode == "wrong-success" {
		for i := range observed {
			if observed[i].Code == 2193 {
				observed[i].Values = []string{"val = 1.0"}
			}
		}
	}
	records, omissions, err := projectEmpty(emptyInvocation(), result)
	if err != nil {
		return err
	}
	records, err = replaceEmptyRecorderValues(records, observed)
	if err != nil {
		return err
	}
	omissions = append(omissions, "recorder diagnostics cover only selected calls; other recorder calls remain unknown")
	omissions = append(omissions, "finish duration is from tool mode, not the original non-tool recorder")
	encoded, err := originalEmptyRecords(records, mode)
	if err != nil {
		return err
	}
	if err := os.WriteFile("bridge-records", []byte(encoded), 0o600); err != nil {
		return err
	}
	data, err := json.Marshal(omissions)
	if err != nil {
		return err
	}
	return os.WriteFile("bridge-omissions.json", data, 0o600)
}

func originalEmptyRecords(records []bridgeRecord, mode string) (string, error) {
	switch mode {
	case "success", "exit":
	case "wrong-stats":
		i := slices.IndexFunc(records, func(record bridgeRecord) bool { return record.code == "2199" })
		if i < 0 || len(records[i].values) == 0 {
			return "", fmt.Errorf("missing Go statistics")
		}
		records[i].values[0] = "999999"
	case "missing-finished":
		records = slices.DeleteFunc(records, func(record bridgeRecord) bool { return record.code == "2186" })
	case "general":
		records = append(records, bridgeRecord{code: "1000"})
	case "malformed":
		return "not-a-record\n", nil
	default:
		return "", fmt.Errorf("unsupported control %q", mode)
	}
	return encodeBridge(records)
}

func boundedOriginalEmptyRun(binary, model string) (Result, error) {
	result, _, err := boundedOriginalEmptyRunWithRecorder(binary, model)
	return result, err
}

func boundedOriginalEmptyRunWithRecorder(binary, model string) (Result, []recorderdebug.Event, error) {
	dir, err := os.MkdirTemp("", "tla-recorder-*")
	if err != nil {
		return Result{}, nil, err
	}
	defer os.RemoveAll(dir)
	recorderPath := filepath.Join(dir, "events.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output := &originalEmptyOutput{limit: 1 << 20, cancel: cancel}
	cmd := exec.CommandContext(ctx, binary, emptyInvocation()...)
	cmd.Dir = model
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=256MiB", "TLA_GO_DEBUG_RECORDER="+recorderPath)
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = time.Second
	err = cmd.Run()
	if ctx.Err() != nil {
		return Result{}, nil, fmt.Errorf("Go invocation timed out or exceeded output limit: %w", ctx.Err())
	}
	if err != nil {
		return Result{}, nil, fmt.Errorf("Go invocation failed: %w", err)
	}
	f, err := os.Open(recorderPath)
	if err != nil {
		return Result{}, nil, fmt.Errorf("open recorder observations: %w", err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	if err != nil {
		return Result{}, nil, err
	}
	return Result{Output: output.String()}, events, nil
}

type originalEmptyOutput struct {
	mu     sync.Mutex
	data   bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (w *originalEmptyOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := w.limit - w.data.Len()
	if len(p) > remaining {
		_, _ = w.data.Write(p[:remaining])
		w.cancel()
		return len(p), nil
	}
	return w.data.Write(p)
}

func (w *originalEmptyOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data.String()
}

func TestOriginalEmptyOutputLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &originalEmptyOutput{limit: 3, cancel: cancel}
	if n, err := out.Write([]byte("ab")); n != 2 || err != nil || ctx.Err() != nil {
		t.Fatalf("first write %d, %v; context %v", n, err, ctx.Err())
	}
	if n, err := out.Write([]byte("cdef")); n != 4 || err != nil || ctx.Err() != context.Canceled || out.String() != "abc" {
		t.Fatalf("second write %d, %v; context %v, output %q", n, err, ctx.Err(), out.String())
	}
}

func TestOriginalEmptyRecordsRejectUnknownMode(t *testing.T) {
	if _, err := originalEmptyRecords(nil, "unexpected"); err == nil {
		t.Fatal("unknown control mode accepted")
	}
	if _, err := originalEmptyRecords(nil, "wrong-stats"); err == nil {
		t.Fatal("missing statistics accepted by mutation control")
	}
}
