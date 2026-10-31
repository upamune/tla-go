package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const compatOptionModel = "VARIABLE x\nInit == x = 0\nNext == x < 2 /\\ x' = x + 1\n" +
	"ASSUME PrintT([i \\in 1..30 |-> i * 1000])\nASSUME Print([i \\in 1..30 |-> i * 1000], TRUE)\nASSUME PrintT(\"short\")"

// Java TLC accepts these options without changing the reported result; they
// configure its fingerprint set, value streams, and debugging output.
func TestJavaTLCAcceptedOptions(t *testing.T) {
	for _, extra := range [][]string{
		{"-gzip"}, {"-modelcheck"}, {"-debug"}, {"-fpbits", "3"}, {"-fpmem", "0.5"}, {"-fpmem", "1"},
	} {
		t.Run(strings.Join(extra, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, compatOptionModel, "INIT Init\nNEXT Next\n")
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0"}, extra...), "M.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if code != 11 || stderr != "" || !hasToolMessage(messages, 2114) {
				t.Fatalf("exit %d; messages %+v; stderr %s", code, messages, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !reflect.DeepEqual(optionMessages(readToolMessages(java)), optionMessages(messages)) {
					t.Fatalf("Java exit %d, Go exit %d\nJava: %s\nGo: %+v", javaCode, code, java, optionMessages(messages))
				}
			}
		})
	}
}

// An absolute -fpmem size is deprecated; Java TLC prints a notice before its
// version banner.
func TestJavaTLCDeprecatedFingerprintMemory(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, compatOptionModel, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-fpmem", "2", "M.tla"}
	output, code, stderr := runGoInDir(t, dir, args)
	if code != 11 || stderr != "" || !strings.HasPrefix(output, fpmemDeprecation+"\n@!@!@STARTMSG 2262:0 @!@!@\n") {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != code || !strings.Contains(java, "\n"+fpmemDeprecation+"\n@!@!@STARTMSG 2262:0 @!@!@\n") {
			t.Fatalf("Java exit %d:\n%s", javaCode, java)
		}
	}
}

// Java TLC rejects invalid option values with exit code 1.
func TestJavaTLCCompatOptionErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-fpbits", "31", "M.tla"}, "Error: Value in interval [0, 30] for fpbits required. But encountered 31"},
		{[]string{"-fpbits", "x", "M.tla"}, "Error: An integer for fpbits required. But encountered x"},
		{[]string{"M.tla", "-fpbits"}, "Error: fpbits required."},
		{[]string{"-fpmem", "-1", "M.tla"}, "Error: An positive integer or a fraction for fpset memory size/percentage required. But encountered -1"},
		{[]string{"-fpmem", "NaN", "M.tla"}, "Error: An positive integer or a fraction for fpset memory size/percentage required. But encountered NaN"},
		{[]string{"M.tla", "-fpmem"}, "Error: fpset memory size required."},
		{[]string{"M.tla", "-userFile"}, "Error: need to specify the full qualified file."},
		{[]string{"-userFile", filepath.Join("missing", "out.txt"), "M.tla"}, "Error: Failed to create user output log file."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, compatOptionModel, "INIT Init\nNEXT Next\n")
			output, code, stderr := runGoInDir(t, dir, tc.args)
			if code != 1 || output != "" || strings.TrimSpace(stderr) != tc.want {
				t.Fatalf("exit %d; stdout %q; stderr %q", code, output, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, tc.args)
				if javaCode != code || !strings.Contains(java, "Error: "+tc.want+"\n") {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
			}
		})
	}
}

// -userFile redirects Print and PrintT to a file. Java TLC pretty-prints
// Print values there but writes PrintT values on a single line.
func TestJavaTLCUserFile(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, compatOptionModel, "INIT Init\nNEXT Next\n")
	output, code, stderr := runGoInDir(t, dir, []string{"-workers", "1", "-userFile", "go.txt", "M.tla"})
	got, err := os.ReadFile(filepath.Join(dir, "go.txt"))
	if code != 11 || stderr != "" || strings.Contains(output, "short") || err != nil {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s\nfile error %v", code, output, stderr, err)
	}
	if !strings.HasPrefix(string(got), "<<1000, 2000, 3000,") || !strings.Contains(string(got), "<< 1000,\n   2000,\n") || !strings.HasSuffix(string(got), "30000 >>  TRUE\n\"short\"\n") {
		t.Fatalf("user file:\n%s", got)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-workers", "1", "-userFile", "java.txt", "M.tla"})
		want, err := os.ReadFile(filepath.Join(dir, "java.txt"))
		if javaCode != code || err != nil || string(want) != string(got) {
			t.Fatalf("Java exit %d, error %v\nJava file:\n%s\nGo file:\n%s\nJava output:\n%s", javaCode, err, want, got, java)
		}
	}
}

// -cleanup removes the states directory in the working directory before
// checking, as Java TLC does when it is not recovering from a checkpoint.
func TestJavaTLCCleanup(t *testing.T) {
	for _, name := range []string{"go", "java"} {
		t.Run(name, func(t *testing.T) {
			jar := os.Getenv("TLC_JAR")
			if name == "java" && jar == "" {
				t.Skip("TLC_JAR is not set")
			}
			dir := t.TempDir()
			writeDOTFixture(t, dir, compatOptionModel, "INIT Init\nNEXT Next\n")
			stale := filepath.Join(dir, "states", "old", "MC.st")
			if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(stale, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-cleanup", "M.tla"}
			var code int
			if name == "go" {
				_, code, _ = runGoInDir(t, dir, args)
			} else {
				_, code = runJavaTLCArgs(t, jar, dir, args)
			}
			if _, err := os.Stat(filepath.Dir(stale)); code != 11 || !os.IsNotExist(err) {
				t.Fatalf("exit %d, stale states: %v", code, err)
			}
		})
	}
}
