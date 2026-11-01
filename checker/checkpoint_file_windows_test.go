package checker

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestCheckpointWindowsOpenAllowsReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	first := checkpointFixture()
	if err := checkpointWrite(path, first); err != nil {
		t.Fatal(err)
	}
	f, err := checkpointOpen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	second := checkpointFixture()
	second.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := checkpointDecode(data, first.Identity); err != nil || got.Nodes[1].Action != "Other" {
		t.Fatalf("new checkpoint: %v", err)
	}
	old, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := checkpointDecode(old, first.Identity); err != nil || got.Nodes[1].Action != first.Nodes[1].Action {
		t.Fatalf("old handle: %v", err)
	}
}

func TestCheckpointWindowsBlockedReplacementPreservesOld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	s := checkpointFixture()
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	// os.Open intentionally denies delete sharing on Windows.
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	start := time.Now()
	err = checkpointWrite(path, s)
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("replacement error: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("replacement did not stop promptly")
	}
	if _, err := checkpointRead(path, s.Identity); err != nil {
		t.Fatalf("old checkpoint lost: %v", err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file left behind: %v %v", files, err)
	}
}

func TestCheckpointWindowsRetriesBlockedReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	s := checkpointFixture()
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conflict := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- checkpointAtomicWriteWithReplace(ctx, path, func(tmp *os.File) error {
			data, err := checkpointEncode(s)
			if err != nil {
				return err
			}
			_, err = tmp.Write(data)
			return err
		}, func(ctx context.Context, from, to string) error {
			return checkpointRetry(ctx, func() error {
				err := checkpointReplaceWindows(from, to)
				if err != nil {
					select {
					case conflict <- err:
					default:
					}
				}
				return err
			})
		})
	}()
	select {
	case err := <-conflict:
		if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
			cancel()
			<-done
			t.Fatalf("expected sharing conflict: %v", err)
		}
	case err := <-done:
		t.Fatalf("replacement finished while blocked: %v", err)
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("timed out waiting for sharing conflict")
	}
	if err := f.Close(); err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("replacement after release: %v", err)
	}
}

func TestCheckpointWindowsCanceledRetryPreservesOld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint")
	old := checkpointFixture()
	if err := checkpointWrite(path, old); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	conflict := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- checkpointAtomicWriteWithReplace(ctx, path, func(tmp *os.File) error {
			if _, err := tmp.Write([]byte("replacement")); err != nil {
				return err
			}
			return nil
		}, func(ctx context.Context, from, to string) error {
			return checkpointRetry(ctx, func() error {
				err := checkpointReplaceWindows(from, to)
				if err != nil {
					select {
					case conflict <- err:
					default:
					}
				}
				return err
			})
		})
	}()
	select {
	case err := <-conflict:
		if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
			cancel()
			<-done
			t.Fatalf("expected sharing conflict: %v", err)
		}
	case err := <-done:
		cancel()
		t.Fatalf("replacement finished before sharing conflict: %v", err)
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("timed out waiting for sharing conflict")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := checkpointRead(path, old.Identity)
	if err != nil || !reflect.DeepEqual(got, old) {
		t.Fatalf("old checkpoint: %v", err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files: %v %v", files, err)
	}
}

func TestCheckpointWindowsConcurrentFirstWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "チェックポイント")
	const writers = 8
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			<-start
			s := checkpointFixture()
			if i%2 != 0 {
				s.Nodes[1].Action = "Other"
			}
			errs <- checkpointWrite(path, s)
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("write: %v", err)
		}
	}
	s := checkpointFixture()
	got, err := checkpointRead(path, s.Identity)
	if err != nil || (got.Nodes[1].Action != "Next" && got.Nodes[1].Action != "Other") {
		t.Fatalf("final checkpoint: %v", err)
	}
}

func TestCheckpointWindowsPathLengthAlignment(t *testing.T) {
	dir := t.TempDir()
	for n := 1; n <= 8; n++ {
		path := filepath.Join(dir, strings.Repeat("x", n))
		first := checkpointFixture()
		if err := checkpointWrite(path, first); err != nil {
			t.Fatalf("path length %d: %v", n, err)
		}
		second := checkpointFixture()
		second.Nodes[1].Action = "Other"
		if err := checkpointWrite(path, second); err != nil {
			t.Fatalf("replace path length %d: %v", n, err)
		}
		got, err := checkpointRead(path, second.Identity)
		if err != nil || !reflect.DeepEqual(got, second) {
			t.Fatalf("read path length %d: %v", n, err)
		}
	}
}

func TestCheckpointWindowsLongUnicodePath(t *testing.T) {
	dir := t.TempDir()
	for len(dir) < 270 {
		dir = filepath.Join(dir, strings.Repeat("long", 20))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "状態")
	s := checkpointFixture()
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	s.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := checkpointRead(path, s.Identity)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("checkpoint: %v", err)
	}
}

func TestCheckpointWindowsRelativePath(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, err := filepath.Rel(wd, filepath.Join(dir, "checkpoint"))
	if err != nil {
		t.Skipf("temporary directory is on another volume: %v", err)
	}
	s := checkpointFixture()
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	s.Nodes[1].Action = "Other"
	if err := checkpointWrite(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := checkpointRead(path, s.Identity)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("checkpoint: %v", err)
	}
}

func TestCheckpointWindowsRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := checkpointRetry(ctx, func() error { attempts++; cancel(); return syscall.ERROR_ACCESS_DENIED })
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("retry: %v, attempts: %d", err, attempts)
	}
}

func TestCheckpointWindowsRetryResultAfterCancellation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		operationError error
		want           error
	}{
		{"success", nil, nil},
		{"failure", syscall.ERROR_FILE_NOT_FOUND, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			err := checkpointRetry(ctx, func() error { cancel(); return tc.operationError })
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCheckpointWindowsRetryDeadline(t *testing.T) {
	attempts := 0
	want := syscall.ERROR_ACCESS_DENIED
	err := checkpointRetryUntil(context.Background(), time.Now().Add(5*time.Millisecond), func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) || attempts != 1 {
		t.Fatalf("retry: %v, attempts: %d", err, attempts)
	}
}
