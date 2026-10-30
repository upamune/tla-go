package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type sanySummary struct {
	variables   []string
	definitions map[string]int
}

func summarizeSANY(t *testing.T, data []byte) sanySummary {
	t.Helper()
	var doc sanyMetadataXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	entries := make(map[int]sanyEntry)
	var root *sanyModule
	for i := range doc.Entries {
		e := doc.Entries[i]
		entries[e.UID] = e
		if e.Module != nil && e.Module.Name == doc.Root {
			root = e.Module
		}
	}
	if root == nil {
		t.Fatalf("missing root module %q", doc.Root)
	}
	s := sanySummary{definitions: map[string]int{}}
	refs := append([]sanyRef(nil), root.Declarations...)
	sort.Slice(refs, func(i, j int) bool { return refs[i].UID < refs[j].UID })
	for _, ref := range refs {
		d := entries[ref.UID].Declaration
		if d != nil && d.Kind == 3 {
			s.variables = append(s.variables, d.Name)
		}
	}
	for _, ref := range root.Definitions {
		if d := entries[ref.UID].Definition; d != nil {
			s.definitions[d.Name] = d.Level
		}
	}
	return s
}

func compareSANYMetadata(t *testing.T, jar, path string) {
	t.Helper()
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
		t.Fatalf("Go metadata exit %d: %s", code, errout.String())
	}
	cmd := exec.Command("java", "-cp", jar, "tla2sany.xml.XMLExporter", "-o", "-I", filepath.Dir(path), path)
	var javaOut, javaErr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &javaOut, &javaErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Java SANY: %v: %s", err, javaErr.String())
	}
	got, want := summarizeSANY(t, out.Bytes()), summarizeSANY(t, javaOut.Bytes())
	if !reflect.DeepEqual(got.variables, want.variables) {
		t.Errorf("variables: Go %v Java %v", got.variables, want.variables)
	}
	for name, level := range got.definitions {
		javaLevel, ok := want.definitions[name]
		if !ok || javaLevel != level {
			t.Errorf("definition %s: Go %d, Java %d (present %t)", name, level, javaLevel, ok)
		}
	}
}

func TestJavaSANYMetadataLevels(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nCONSTANT C(_)\nVARIABLES x,y\n" +
		"F(a) == TRUE\nG == F(x)\nI(a) == a\nJ == I(x)\nE == ENABLED (x' = x)\n" +
		"T == LET unused == x' IN TRUE\nQ == \\A v \\in {x}: v = x\nU == UNCHANGED x\n" +
		"P == [] (x = x)\nA == C(x)\nR == \\EE v : TRUE\nS == \\AA v : TRUE\n====\n"
	path := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	compareSANYMetadata(t, jar, path)
}

func TestJavaSANYStatechartMetadata(t *testing.T) {
	root, jar := os.Getenv("STATECHART_DIR"), os.Getenv("TLC_JAR")
	if root == "" || jar == "" {
		t.Skip("set STATECHART_DIR and TLC_JAR")
	}
	corpus := filepath.Join(root, "scripts", "test", "corpus")
	entries, err := os.ReadDir(corpus)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		count++
		t.Run(entry.Name(), func(t *testing.T) {
			compareSANYMetadata(t, jar, filepath.Join(corpus, entry.Name(), fmt.Sprintf("%s.tla", entry.Name())))
		})
	}
	if count == 0 {
		t.Fatal("empty corpus")
	}
}

func TestJavaSANYTLCLevels(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	path := filepath.Join(t.TempDir(), "M.tla")
	source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\n" +
		"A == TLCGet(\"level\")\nB == TLCSet(1, x)\nC == PrintT(x)\n" +
		"D == Print(x, TRUE)\nE == Print(TRUE, x)\nF == Assert(TRUE, x)\nG == RandomElement({x})\n====\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	compareSANYMetadata(t, jar, path)
}

func TestJavaSANYTLCInstanceLevels(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	path := filepath.Join(t.TempDir(), "M.tla")
	source := "---- MODULE M ----\nVARIABLE x\nT == INSTANCE TLC\n" +
		"A == T!PrintT(x)\nB == T!TLCSet(1, x)\nC == T!Print(x, TRUE)\n====\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	compareSANYMetadata(t, jar, path)
}
