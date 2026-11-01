package upstreamjunit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This class leaves a live child holding the direct JVM's stdout and stderr.
const descendantSource = `import java.nio.file.*;
public class Descendant {
  public static void main(String[] args) throws Exception {
    Path marker = Path.of(args[1]);
    if (args[0].equals("child")) {
      Files.writeString(marker, Long.toString(ProcessHandle.current().pid()));
      Thread.sleep(30000);
      return;
    }
    String java = Path.of(System.getProperty("java.home"), "bin", "java").toString();
    new ProcessBuilder(java, "-cp", args[2], "Descendant", "child", args[1]).inheritIO().start();
    while (!Files.exists(marker)) Thread.sleep(10);
    System.out.println("temp=" + System.getProperty("java.io.tmpdir"));
    if (args[0].equals("timeout")) Thread.sleep(30000);
  }
}`

func TestRunJavaCommandBoundsInheritedOutput(t *testing.T) {
	requireJava11(t, "java")
	requireJava11(t, "javac")
	dir := t.TempDir()
	source := filepath.Join(dir, "Descendant.java")
	if err := os.WriteFile(source, []byte(descendantSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("javac", source).CombinedOutput(); err != nil {
		t.Fatalf("compile descendant fixture: %v\n%s", err, output)
	}
	for _, mode := range []string{"timeout", "exit"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "child-pid")
			start := time.Now()
			result, err := runJavaCommand(dir, 5*time.Second, "-cp", dir, "Descendant", mode, marker, dir)
			elapsed := time.Since(start)
			pidData, readErr := os.ReadFile(marker)
			if readErr == nil {
				pid, parseErr := strconv.Atoi(string(pidData))
				if parseErr == nil {
					if child, findErr := os.FindProcess(pid); findErr == nil {
						defer child.Kill()
					}
				}
			}
			if readErr != nil {
				t.Fatalf("child did not start: %v; output: %s", readErr, result.Output)
			}
			if elapsed > 10*time.Second {
				t.Fatalf("inherited output blocked for %s", elapsed)
			}
			if mode == "timeout" && (err == nil || !result.TimedOut) {
				t.Fatalf("expected timeout: %+v, %v", result, err)
			}
			if mode == "exit" && (!errors.Is(err, exec.ErrWaitDelay) || result.TimedOut) {
				t.Fatalf("expected pipe wait error without timeout: %+v, %v", result, err)
			}
			tmp, ok := strings.CutPrefix(strings.TrimSpace(result.Output), "temp=")
			if !ok || tmp == "" {
				t.Fatalf("missing temporary directory in output: %q", result.Output)
			}
			if _, statErr := os.Stat(tmp); !os.IsNotExist(statErr) {
				t.Fatalf("temporary directory still exists: %s (%v)", tmp, statErr)
			}
		})
	}
}
