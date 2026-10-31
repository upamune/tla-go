package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Java TLC prints an empty TupleValue as <<>> but an empty FcnRcdValue, such
// as the result of a function constructor over an empty domain, as << >>.
func TestJavaTLCEmptyFunctionPrinting(t *testing.T) {
	expressions := []string{"[{} -> {1}]", "<<>>", "[i \\in {} |-> i]", "Tail(<<1>>)", "SubSeq(<<1>>, 2, 1)", "<<<<>>, [i \\in 1..0 |-> i]>>", "ToString([i \\in {} |-> i])", "SelectSeq(<<1>>, LAMBDA x : FALSE)", "[[i \\in {} |-> i] EXCEPT ![1] = 2]"}
	want := []string{"{<< >>}", "<<>>", "<< >>", "<<>>", "<<>>", "<<<<>>, << >>>>", `"<< >>"`, "<<>>", "<< >>"}
	source := "---- MODULE M ----\nEXTENDS Naturals, Sequences, TLC\n"
	for _, expression := range expressions {
		source += "ASSUME PrintT(" + expression + ")\n"
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source + "====\n", "M.cfg": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if got := printedLines(out.String()); code != 0 || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("exit %d, printed %q, want %q; stderr:\n%s", code, got, want, errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-jar", jar, "-tool", "M.tla")
		cmd.Dir = dir
		output, _ := cmd.CombinedOutput()
		if got := printedLines(string(output)); cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("Java exit %v, printed %q; output:\n%s", cmd.ProcessState, got, output)
		}
	}
}

// printedLines returns tool-mode output outside messages, without SANY's
// progress lines and the JVM's notices.
func printedLines(output string) []string {
	var lines []string
	depth := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "@!@!@STARTMSG"):
			depth++
		case strings.HasPrefix(line, "@!@!@ENDMSG"):
			depth--
		case depth == 0 && line != "" && !strings.HasPrefix(line, "Parsing file") && !strings.HasPrefix(line, "Semantic processing") && !strings.HasPrefix(line, "Picked up"):
			lines = append(lines, line)
		}
	}
	return lines
}
