package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCDotPolynomials(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, index := range []int{0, 1, 63, 130} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x \\in 0..2\nNext == x' = (x + 1) % 3", "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			goPath := filepath.Join(dir, "go.dot")
			code := run(context.Background(), []string{"-workers", "1", "-fp", fmt.Sprint(index), "-dump", "dot,actionlabels", goPath, filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 {
				t.Fatalf("tla-go exit %d: %s %s", code, out.String(), errout.String())
			}
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", fmt.Sprint(index), "-dump", "dot,actionlabels", "java.dot", "M.tla")
			cmd.Dir = dir
			if output, err := javatmp.Apply(cmd).CombinedOutput(); err != nil {
				t.Fatalf("Java TLC: %v\n%s", err, output)
			}
			got, err := os.ReadFile(goPath)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(dir, "java.dot"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("DOT differs from Java TLC\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
