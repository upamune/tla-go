package upstreamjunit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInventoryInheritance(t *testing.T) {
	dir := t.TempDir()
	src := `package example;
 // class Fake extends ModelCheckerTestCase {}
 abstract class Middle extends ModelCheckerTestCase {}
 class Child extends Middle { String s = "class Fake2 extends SuiteTestCase"; }
 class SuiteChild extends SuiteETestCase {}
 `
	if err := os.WriteFile(filepath.Join(dir, "Cases.java"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Inventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "example.Child" || got[0].Kind != "ModelChecker" || !got[1].Abstract || got[2].Kind != "SuiteE" {
		t.Fatalf("%+v", got)
	}
}

func TestInventoryTrackedCommit(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "source checkout")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(repo, "init", "-q")
	root := filepath.Join(repo, "test sources")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(root, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(root, "Tracked\nCase.java", "package example; class Tracked extends ModelCheckerTestCase {}")
	write(repo, ".gitignore", "Ignored.java\n")
	git(repo, "add", ".")
	git(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture")
	commit := git(repo, "rev-parse", "HEAD")
	worktree := filepath.Join(base, "linked checkout")
	git(repo, "worktree", "add", "-q", "--detach", worktree, commit)
	root = filepath.Join(worktree, "test sources")
	write(root, "Untracked.java", "package example; class Untracked extends ModelCheckerTestCase {}")
	write(root, "Ignored.java", "package example; class Ignored extends ModelCheckerTestCase {}")
	write(root, "Staged.java", "package example; class Staged extends ModelCheckerTestCase {}")
	git(worktree, "add", "test sources/Staged.java")
	got, err := inventoryTracked(root, commit)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "example.Tracked" || got[0].Source != "Tracked\nCase.java" {
		t.Fatalf("tracked inventory: %+v", got)
	}
}

func TestVerifyInputsRejectsMissing(t *testing.T) {
	if err := VerifyInputs(t.TempDir(), "missing.jar"); err == nil {
		t.Fatal("accepted non-checkout")
	}
}

func TestDiscoveryBoundsInputVerification(t *testing.T) {
	dir := t.TempDir()
	program := `package main
import ("fmt"; "os"; "strings"; "time")
func main() {
 args := strings.Join(os.Args[1:], " ")
 if strings.Contains(args, os.Getenv("BLOCK_GIT_STAGE")) {
  if err := os.WriteFile(os.Getenv("GIT_STARTED"), []byte("started"), 0600); err != nil { panic(err) }
  time.Sleep(120 * time.Second)
 }
 if strings.Contains(args, "rev-parse") { fmt.Println("` + UpstreamCommit + `") }
}`
	source := filepath.Join(dir, "fake_git.go")
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	git := filepath.Join(dir, "git")
	if runtime.GOOS == "windows" {
		git += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", git, source).CombinedOutput(); err != nil {
		t.Fatalf("build git fixture: %v: %s", err, out)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, stage := range []string{"rev-parse", "status"} {
		t.Run(stage, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			t.Setenv("BLOCK_GIT_STAGE", stage)
			t.Setenv("GIT_STARTED", marker)
			err := cancelAfterReady(t, marker, func(ctx context.Context) error {
				_, err := discover(ctx, dir, "unused", "unused", "unused", dir, nil)
				return err
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected cancellation for %s, got %v", stage, err)
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := discover(ctx, dir, "unused", "unused", "unused", dir, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}
