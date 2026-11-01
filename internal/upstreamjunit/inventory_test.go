package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
