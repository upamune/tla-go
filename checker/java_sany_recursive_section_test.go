package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// Generator.checkIfInRecursiveSection at upstream commit 5a47802b, Java
// TLA+ tools 1.7.4. These cases test source order, not evaluation of a model.
func TestJavaSANYRecursiveDefinitionSection(t *testing.T) {
	cases := []struct {
		name, body string
		invalid    bool
	}{
		{"constant", "RECURSIVE F(_)\nCONSTANT C\nF(x) == x", true},
		{"variable", "RECURSIVE F(_)\nVARIABLE v\nF(x) == x", true},
		{"assume", "RECURSIVE F(_)\nASSUME TRUE\nF(x) == x", true},
		{"named assume", "RECURSIVE F\nASSUME F == TRUE", true},
		{"named axiom", "RECURSIVE F\nAXIOM F == TRUE", true},
		{"named theorem", "RECURSIVE F\nTHEOREM F == TRUE", true},
		{"theorem", "RECURSIVE F(_)\nTHEOREM TRUE\nF(x) == x", true},
		{"use", "G == TRUE\nRECURSIVE F(_)\nUSE DEF G\nF(x) == x", true},
		{"hide", "G == TRUE\nRECURSIVE F(_)\nHIDE DEF G\nF(x) == x", true},
		{"inner module", "RECURSIVE F(_)\n---- MODULE Inner ----\n====", true},
		{"one unresolved", "RECURSIVE F(_), G(_)\nF(x) == x\nVARIABLE v\nG(x) == x", true},
		{"multiple recursive statements", "RECURSIVE F(_)\nRECURSIVE G(_)\nF(x) == x\nASSUME TRUE\nG(x) == x", true},
		{"before", "CONSTANT C\nVARIABLE v\nASSUME TRUE\nRECURSIVE F(_)\nF(x) == x", false},
		{"after", "RECURSIVE F(_)\nF(x) == x\nCONSTANT C\nVARIABLE v\nASSUME TRUE\nTHEOREM TRUE", false},
		{"after all definitions", "RECURSIVE F(_), G(_)\nG(x) == x\nF(x) == x\nVARIABLE v", false},
		{"two separate sections", "RECURSIVE F(_)\nF(x) == x\nCONSTANT C\nRECURSIVE G(_)\nG(x) == x\nVARIABLE v", false},
		{"intermediate definition", "RECURSIVE F(_)\nG == TRUE\nF(x) == x\nUSE DEF G", false},
		{"separate LET", "RECURSIVE F(_)\nG == LET RECURSIVE H(_) H(x) == x IN H(1)\nF(x) == x\nVARIABLE v", false},
		{"instance", "RECURSIVE F(_)\nINSTANCE A\nI == INSTANCE A\nF(x) == x\nVARIABLE v", false},
		{"inner after resolution", "RECURSIVE F(_)\nF(x) == x\n---- MODULE Inner ----\n====\nVARIABLE v", false},
	}
	jar := recursiveSectionJar(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.body + "\n====\n"
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			err = validateRecursiveModule(module)
			if (err != nil) != tc.invalid || tc.invalid && !strings.Contains(err.Error(), "recursive definition section") {
				t.Fatalf("Go error %v; expected section error %t", err, tc.invalid)
			}
			if jar != "" {
				checkRecursiveSectionSANY(t, jar, source, tc.invalid)
			}
		})
	}
}

// SANY 1.7.4 clears the unresolved count while leaving an inner module, then
// crashes when the enclosing module defines the declared operator. Go rejects
// the section as a semantic error; it does not reproduce the Java exception.
func TestJavaSANYRecursiveSectionInnerModuleCrash(t *testing.T) {
	const source = "---- MODULE M ----\nRECURSIVE F(_)\n---- MODULE Inner ----\n====\nF(x) == x\n====\n"
	m, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRecursiveModule(m); err == nil || !strings.Contains(err.Error(), "recursive definition section") {
		t.Fatalf("Go error: %v", err)
	}
	jar := recursiveSectionJar(t)
	if jar == "" {
		return
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
	if code != 255 || !strings.Contains(out, "WrongInvocationException: Defined more recursive operators than were declared") {
		t.Fatalf("Java exit %d:\n%s", code, out)
	}
}

func TestRecursiveSectionPublicCheckAndUnknownPositions(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE v\nRECURSIVE F(_)\nCONSTANT C\nF(x) == x\nInit == v = 0\nNext == UNCHANGED v\n====\n"
	_, err := Check(source, "CONSTANT C = 1\nINIT Init\nNEXT Next\n")
	if err == nil || !strings.Contains(err.Error(), "recursive definition section") {
		t.Fatalf("Check error: %v", err)
	}
	parsed, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("CONSTANT C = 1\nINIT Init\nNEXT Next\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = CheckModuleWithLoader(context.Background(), parsed, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "recursive definition section") {
		t.Fatalf("CheckModuleWithLoader error: %v", err)
	}
	// Location-free constructed modules cannot establish source order.
	module := &parser.Module{
		RecursiveDeclarations: []parser.Declaration{{Name: "F", Arity: 1}},
		Constants:             []parser.Declaration{{Name: "C"}},
		Definitions:           []parser.Definition{{Name: "F", Params: []parser.Declaration{{Name: "x"}}}},
	}
	if err := validateRecursiveSection(module); err != nil {
		t.Fatalf("unknown source order: %v", err)
	}
}

func TestRecursiveSectionInLoadedModule(t *testing.T) {
	const root = "---- MODULE M ----\nEXTENDS A\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
	const imported = "---- MODULE A ----\nRECURSIVE F(_)\nCONSTANT C\nF(x) == x\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name == "A" {
			return imported, nil
		}
		return "", os.ErrNotExist
	}
	_, err := CheckWithLoader(context.Background(), root, "CONSTANT C = 1\nINIT Init\nNEXT Next\n", loader)
	if err == nil || !strings.Contains(err.Error(), "recursive definition section") {
		t.Fatalf("loaded module error: %v", err)
	}
}

func recursiveSectionJar(t *testing.T) string {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return ""
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}

func checkRecursiveSectionSANY(t *testing.T, jar, source string, invalid bool) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source, "A.tla": "---- MODULE A ----\n====\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
	if code != 0 || strings.Contains(out, "Parse Error") || strings.Contains(out, "Semantic errors:") != invalid || invalid && !strings.Contains(out, "recursive definition section") {
		t.Fatalf("Java exit %d, expected section error %t:\n%s", code, invalid, out)
	}
}
