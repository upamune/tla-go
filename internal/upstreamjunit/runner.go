package upstreamjunit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Case is a provisional invocation. IDs identify observations to compare;
// a match only applies to those observations, not to the whole JUnit test.
type Case struct {
	Name, Model     string
	Path            string
	Deadlock        bool
	Checkpoint      string
	Args, IDs       []string
	TranslationArgs []string
	Translate       bool
	PresenceIDs     []string
	Workers         string
	Coverage, Dump  bool
	Unresolved      string
}

// RunCase gives each implementation its own copy of tracked model fixtures.
// Timeout, process launch and fixture errors are infrastructure errors.
func RunCase(root, jar, binary string, c Case, timeout time.Duration) (Result, Result, []string, error) {
	tmp, err := os.MkdirTemp("", "tla-go-junit-")
	if err != nil {
		return Result{}, Result{}, nil, err
	}
	defer os.RemoveAll(tmp)
	if err := copyModels(root, tmp); err != nil {
		return Result{}, Result{}, nil, err
	}
	if !filepath.IsLocal(c.Path) && c.Path != "" {
		return Result{}, Result{}, nil, fmt.Errorf("unsafe model path %q", c.Path)
	}
	translationDiffs, j, g, err := translateCase(tmp, jar, binary, c, timeout)
	if err != nil {
		return j, g, translationDiffs, err
	}
	j, g, diffs, err := runModelCase(tmp, jar, binary, c, timeout)
	return j, g, append(translationDiffs, diffs...), err
}

func runModelCase(tmp, jar, binary string, c Case, timeout time.Duration) (Result, Result, []string, error) {
	jargs := caseArgs(c, true, filepath.Join(tmp, "java.dot"))
	gargs := caseArgs(c, false, filepath.Join(tmp, "go.dot"))
	j, err := runJavaCase(tmp, jar, c.Path, timeout, jargs)
	if err != nil {
		return j, Result{}, nil, err
	}
	g, err := runCommand(filepath.Join(tmp, "go", c.Path), timeout, binary, gargs...)
	if err != nil {
		return j, g, nil, err
	}
	diffs, err := compareCase(j, g, c)
	if err == nil && c.Dump {
		var dumpDiff []string
		dumpDiff, err = compareDump(tmp)
		diffs = append(diffs, dumpDiff...)
	}
	return j, g, diffs, err
}

func compareDump(tmp string) ([]string, error) {
	j, je := os.ReadFile(filepath.Join(tmp, "java.dot"))
	g, ge := os.ReadFile(filepath.Join(tmp, "go.dot"))
	if je != nil || ge != nil {
		return []string{fmt.Sprintf("dump:unavailable java=%v go=%v", je, ge)}, nil
	}
	if !bytes.Equal(j, g) {
		return []string{"dump:content"}, nil
	}
	return nil, nil
}

func runJavaCase(tmp, jar, path string, timeout time.Duration, args []string) (Result, error) {
	wrapper := filepath.Join(tmp, "JUnitTLC.java")
	if err := os.WriteFile(wrapper, []byte(javaWrapper), 0o600); err != nil {
		return Result{}, err
	}
	return runJavaCommand(filepath.Join(tmp, "java", path), timeout, append([]string{"-cp", jar, wrapper}, args...)...)
}

func compareCase(j, g Result, c Case) ([]string, error) {
	diffs, err := Compare(j, g, c.IDs)
	if err != nil {
		return diffs, err
	}
	presence, err := ComparePresence(j, g, c.PresenceIDs)
	return append(diffs, presence...), err
}

func caseArgs(c Case, java bool, dumpPath string) []string {
	workers, checkpoint := c.Workers, c.Checkpoint
	if workers == "" {
		workers = "1"
	}
	if checkpoint == "" {
		checkpoint = "0"
	}
	args := []string{"-tool"}
	if !c.Deadlock {
		args = append(args, "-deadlock")
	}
	args = append(args, "-fp", "0")
	// Go coverage is under independent integration and is deliberately omitted.
	if c.Coverage && java {
		args = append(args, "-coverage", "1")
	}
	args = append(args, "-workers", workers, "-checkpoint", checkpoint)
	if c.Dump {
		args = append(args, "-dump", "dot", dumpPath)
	}
	args = append(args, c.Args...)
	return append(args, c.Model)
}

func translateCase(tmp, jar, binary string, c Case, timeout time.Duration) ([]string, Result, Result, error) {
	if !c.Translate {
		return nil, Result{}, Result{}, nil
	}
	options := append([]string{"-unixEOL"}, c.TranslationArgs...)
	options = append(options, c.Model+".tla")
	j, e := runJavaCommand(filepath.Join(tmp, "java", c.Path), timeout, append([]string{"-cp", jar, "pcal.trans"}, options...)...)
	if e != nil {
		return nil, j, Result{}, e
	}
	g, e := runCommand(filepath.Join(tmp, "go", c.Path), timeout, binary, append([]string{"-translate"}, options...)...)
	if e != nil {
		return nil, j, g, e
	}
	var diffs []string
	if j.ExitCode != g.ExitCode {
		diffs = append(diffs, "translation:exit")
	}
	if j.ExitCode != 0 || g.ExitCode != 0 {
		return diffs, j, g, fmt.Errorf("translation failed: Java %d Go %d", j.ExitCode, g.ExitCode)
	}
	files, e := translationFiles(tmp, c)
	return append(diffs, files...), j, g, e
}

func translationFiles(tmp string, c Case) ([]string, error) {
	var diffs []string
	for _, ext := range []string{".tla", ".cfg"} {
		different, err := translationFileDiff(tmp, c, ext)
		if err != nil {
			return nil, err
		}
		if different {
			diffs = append(diffs, "translation:file:"+ext)
		}
	}
	return diffs, nil
}

func translationFileDiff(tmp string, c Case, ext string) (bool, error) {
	a, ea := os.ReadFile(filepath.Join(tmp, "java", c.Path, c.Model+ext))
	b, eb := os.ReadFile(filepath.Join(tmp, "go", c.Path, c.Model+ext))
	if ea != nil && !os.IsNotExist(ea) {
		return false, ea
	}
	if eb != nil && !os.IsNotExist(eb) {
		return false, eb
	}
	return (ea == nil) != (eb == nil) || (ea == nil && !bytes.Equal(a, b)), nil
}

func copyModels(root, tmp string) error {
	modelRoot := filepath.Join(root, "tlatools", "org.lamport.tlatools", "test-model")
	files, err := exec.Command("git", "-C", modelRoot, "ls-files", "-z", "--", ".").Output()
	if err != nil {
		return fmt.Errorf("list fixtures: %w", err)
	}
	for _, side := range []string{"java", "go"} {
		dir := filepath.Join(tmp, side)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := copyModelFiles(modelRoot, dir, string(files)); err != nil {
			return err
		}
	}
	return nil
}

func copyModelFiles(root, dir, files string) error {
	for name := range strings.SplitSeq(files, "\x00") {
		// Unrelated classes can install a global Java override provider.
		if name == "" || strings.HasSuffix(name, ".class") {
			continue
		}
		if err := copyFixture(root, dir, name); err != nil {
			return err
		}
	}
	return nil
}

func copyFixture(root, dest, name string) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("unsafe fixture path %q", name)
	}
	src := filepath.Join(root, name)
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular fixture %s: %s", name, info.Mode())
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	target := filepath.Join(dest, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fs.FileMode(0o600))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func runCommand(dir string, timeout time.Duration, name string, args ...string) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	result := Result{Output: string(out), TimedOut: ctx.Err() != nil}
	if result.TimedOut {
		return result, fmt.Errorf("%s timed out: %w", name, ctx.Err())
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exit.ExitCode()
		} else {
			return result, err
		}
	}
	return result, nil
}

// Each JVM owns its standard-module extraction directory until it exits.
func runJavaCommand(dir string, timeout time.Duration, args ...string) (result Result, err error) {
	tmp, err := os.MkdirTemp("", "tla-go-java-")
	if err != nil {
		return Result{}, fmt.Errorf("create Java temporary directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove Java temporary directory: %w", cleanupErr))
		}
	}()
	return runCommand(dir, timeout, "java", append([]string{"-Djava.io.tmpdir=" + tmp}, args...)...)
}

// The upstream ModelCheckerTestCase defers graph-wide liveness checks.
const javaWrapper = `class JUnitTLC {
  public static void main(String[] args) throws Exception {
    tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE;
    tlc2.TLC.main(args);
  }
}
`
