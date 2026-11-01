package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCLibrarySearch(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		local, first, second string
		want                 int
	}{
		{"first library", "", "0", "1", 0},
		{"second library", "", "", "1", 12},
		{"local first", "0", "1", "1", 0},
		{"missing", "", "", "", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			specDir := filepath.Join(root, "spec with spaces")
			first := filepath.Join(root, "first library")
			second := filepath.Join(root, "second library")
			for _, dir := range []string{specDir, first, second} {
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(specDir, "M.tla"), []byte("---- MODULE M ----\nEXTENDS Lib\nVARIABLE x\nInit == x = K\nNext == UNCHANGED x\nInv == x = 0\n====\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(specDir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for dir, value := range map[string]string{specDir: tc.local, first: tc.first, second: tc.second} {
				if value != "" {
					if err := os.WriteFile(filepath.Join(dir, "Lib.tla"), []byte("---- MODULE Lib ----\nK == "+value+"\n====\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			path := strings.Join([]string{first, second}, string(os.PathListSeparator))
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-DTLA-Library=" + path, filepath.Join(specDir, "M.tla")}, &out, &errout)
			if code != tc.want {
				t.Fatalf("Go exit %d want %d\n%s\n%s", code, tc.want, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-DTLA-Library="+path, "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", filepath.Join(specDir, "M.tla"))
				cmd.Dir = root
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.want {
					t.Fatalf("Java exit: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestMetadataDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "meta")
	now := time.Date(2026, 10, 30, 12, 34, 56, 0, time.Local)
	dir, err := makeMetaDir(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "26-10-30-12-34-56" {
		t.Fatalf("unexpected directory: %s", dir)
	}
	other, err := makeMetaDir(root, now)
	if err != nil || other == dir {
		t.Fatalf("collision: directory %q, error %v", other, err)
	}
	if info, err := os.Stat(other); err != nil || !info.IsDir() {
		t.Fatalf("second directory: %v", err)
	}
}

func TestMetadataDirectoryDump(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	root := filepath.Join(dir, "meta")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-metadir", root, "-dump", "dot,actionlabels", "${metadir}/graph", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 12 {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, entries[0].Name(), "graph.dot")); err != nil {
		t.Fatal(err)
	}
}

func TestJavaTLCLibraryTransitive(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(root, "spec")
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	for _, dir := range []string{spec, first, second} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(spec, "M.tla"):       "---- MODULE M ----\nEXTENDS Lib\nVARIABLE x\nInit == x = K\nNext == UNCHANGED x\n====\n",
		filepath.Join(spec, "M.cfg"):       "INIT Init\nNEXT Next\n",
		filepath.Join(first, "Lib.tla"):    "---- MODULE Lib ----\nEXTENDS Other\nK == J\n====\n",
		filepath.Join(second, "Other.tla"): "---- MODULE Other ----\nJ == 1\n====\n",
	}
	for path, source := range files {
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := strings.Join([]string{first, second}, string(os.PathListSeparator))
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-DTLA-Library=" + paths, filepath.Join(spec, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-DTLA-Library="+paths, "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", filepath.Join(spec, "M.tla"))
		cmd.Dir = root
		if output, err := javatmp.Apply(cmd).CombinedOutput(); err != nil {
			t.Fatalf("Java: %v\n%s", err, output)
		}
	}
}

func TestJavaTLCMetadataDirectory(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare metadata directories")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	meta := filepath.Join(dir, "java metadata")
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-metadir", meta, "-dump", "dot,actionlabels", "${metadir}/graph", filepath.Join(dir, "M.tla"))
	cmd.Dir = dir
	if output, err := javatmp.Apply(cmd).CombinedOutput(); cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 12 {
		t.Fatalf("Java: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(meta)
	if err != nil || len(entries) != 1 {
		t.Fatalf("Java metadata: %v %v", entries, err)
	}
	if _, err := time.ParseInLocation("06-01-02-15-04-05", entries[0].Name(), time.Local); err != nil {
		t.Fatalf("Java metadata directory name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(meta, entries[0].Name(), "graph.dot")); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataDirectoryCleanup(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	root := filepath.Join(dir, "go-meta")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-metadir", root, filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("Go metadata after success: %v %v", entries, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		javaRoot := filepath.Join(dir, "java-meta")
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-metadir", javaRoot, filepath.Join(dir, "M.tla"))
		cmd.Dir = dir
		if output, err := javatmp.Apply(cmd).CombinedOutput(); err != nil {
			t.Fatalf("Java: %v\n%s", err, output)
		}
		if entries, err := os.ReadDir(javaRoot); err != nil || len(entries) != 0 {
			t.Fatalf("Java metadata after success: %v %v", entries, err)
		}
	}
}

func TestJavaTLCDefaultMetadataDirectoryDump(t *testing.T) {
	for _, tc := range []struct {
		name, inv, cfg string
		code           int
	}{
		{"success", "", "", 0},
		{"violation", "Inv == x = 1", "INVARIANT Inv\n", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n"+tc.inv, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := []string{"-tool", "-workers", "1", "-checkpoint", "0", "-dump", "dot", "${metadir}/graph", filepath.Join(dir, "M.tla")}
			var out, errout bytes.Buffer
			code := run(context.Background(), args, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
			root := filepath.Join(dir, "states")
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != boolInt(tc.code != 0) {
				t.Fatalf("Go metadata entries: %v %v", entries, err)
			}
			if tc.code != 0 {
				if _, err := os.Stat(filepath.Join(root, entries[0].Name(), "graph.dot")); err != nil {
					t.Fatal(err)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaDir := t.TempDir()
				writeDOTFixture(t, javaDir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n"+tc.inv, "INIT Init\nNEXT Next\n"+tc.cfg)
				javaArgs := append([]string(nil), args...)
				javaArgs[len(javaArgs)-1] = filepath.Join(javaDir, "M.tla")
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, javaArgs...)...)
				cmd.Dir = javaDir
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java exit %v: %s", err, output)
				}
			}
		})
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestDefaultMetadataRootRelativeAndAbsolute(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ path, want string }{
		{"M.tla", "states"},
		{filepath.Join("nested", "M.tla"), "states"},
		{filepath.Join(dir, "M.tla"), filepath.Join(dir, "states")},
	} {
		if got := defaultMetaRoot(tc.path); got != tc.want {
			t.Errorf("defaultMetaRoot(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestJavaTLCRelativeMetadataDirectoryDump(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "tla-go")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	for _, tc := range []struct {
		name   string
		binary string
		args   []string
	}{
		{"Go", binary, nil},
		{"Java", "java", []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", os.Getenv("TLC_JAR")}},
	} {
		if tc.name == "Java" && os.Getenv("TLC_JAR") == "" {
			continue
		}
		dir := filepath.Join(root, tc.name)
		spec := filepath.Join(dir, "nested")
		if err := os.MkdirAll(spec, 0o700); err != nil {
			t.Fatal(err)
		}
		writeDOTFixture(t, spec, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
		args := append(tc.args, "-tool", "-workers", "1", "-checkpoint", "0", "-dump", "dot", "${metadir}/graph", filepath.Join("nested", "M.tla"))
		cmd := exec.Command(tc.binary, args...)
		cmd.Dir = dir
		output, err := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 12 {
			t.Fatalf("%s exit %v: %s", tc.name, err, output)
		}
		matches, err := filepath.Glob(filepath.Join(dir, "states", "*", "graph.dot"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("%s metadata dump: %v %v", tc.name, matches, err)
		}
		if _, err := os.Stat(filepath.Join(spec, "states")); !os.IsNotExist(err) {
			t.Fatalf("%s created metadata under relative spec directory: %v", tc.name, err)
		}
	}
}
