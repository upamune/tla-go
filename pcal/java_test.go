package pcal

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// corpusCase is one run of the translator on files from testdata.
type corpusCase struct {
	Name  string   `json:"name"`
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
	// Dirs lists empty directories to create, for names of files that
	// cannot be read.
	Dirs []string `json:"dirs,omitempty"`
	Args []string `json:"args"`
	// Strip removes an existing translation, so that the translator
	// inserts a new one with checksums.
	Strip bool `json:"strip,omitempty"`
	// Source names the upstream file a mutated input was derived from.
	Source string `json:"source,omitempty"`
}

// outcome is what a run prints and leaves in its directory. Files holds
// the contents of the files that differ from the input file of the same
// name; Same lists the files that equal an input file, and Dirs the
// directories.
type outcome struct {
	Exit   int               `json:"exit"`
	Stdout string            `json:"stdout"`
	Files  map[string]string `json:"files,omitempty"`
	Same   map[string]string `json:"same,omitempty"`
	Dirs   []string          `json:"dirs,omitempty"`
}

const goldenPath = "testdata/golden.json.gz"

func loadCases(t *testing.T) []corpusCase {
	t.Helper()
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []corpusCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func loadGolden(t *testing.T) map[string]outcome {
	t.Helper()
	f, err := os.Open(goldenPath)
	if os.IsNotExist(err) {
		return map[string]outcome{}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	golden := map[string]outcome{}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

// Java resolves Upper.tla to Upper.TLA on case-insensitive filesystems. The
// default golden was generated on Linux, where those are different paths.
func useCaseInsensitiveGolden(t *testing.T, golden map[string]outcome) bool {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CaseProbe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/golden-case-insensitive.json")
	if err != nil {
		t.Fatal(err)
	}
	var alt outcome
	if err := json.Unmarshal(data, &alt); err != nil {
		t.Fatal(err)
	}
	golden["cli-upper"] = alt
	return true
}

func writeCaseInsensitiveGolden(t *testing.T, value outcome) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("testdata/golden-case-insensitive.json", append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeGolden(t *testing.T, golden map[string]outcome) {
	t.Helper()
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	enc := json.NewEncoder(zw)
	enc.SetIndent("", " ")
	if err := enc.Encode(golden); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stripTranslation removes the lines from BEGIN TRANSLATION through END
// TRANSLATION.
func stripTranslation(text string) string {
	var out []string
	skipping := false
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case !skipping && strings.Contains(line, "BEGIN TRANSLATION"):
			skipping = true
		case skipping:
			skipping = !strings.Contains(line, "END TRANSLATION")
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// prepare copies the input files of c into a new directory and returns the
// directory and the inputs.
func prepare(t *testing.T, c corpusCase) (string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	inputs := map[string]string{}
	for _, name := range c.Files {
		data, err := os.ReadFile(filepath.Join("testdata", c.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if c.Strip && strings.HasSuffix(name, ".tla") {
			text = stripTranslation(text)
		}
		inputs[name] = text
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range c.Dirs {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, inputs
}

// collect records the files a run left in dir.
func collect(t *testing.T, dir string, inputs map[string]string, exit int, stdout string) outcome {
	t.Helper()
	o := outcome{Exit: exit, Stdout: stdout, Files: map[string]string{}, Same: map[string]string{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			o.Dirs = append(o.Dirs, e.Name())
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if same := sameInput(string(data), inputs); same != "" {
			o.Same[e.Name()] = same
		} else {
			o.Files[e.Name()] = string(data)
		}
	}
	return o
}

func sameInput(text string, inputs map[string]string) string {
	for _, name := range slices.Sorted(maps.Keys(inputs)) {
		if inputs[name] == text {
			return name
		}
	}
	return ""
}

func runGo(t *testing.T, c corpusCase) outcome {
	t.Helper()
	dir, inputs := prepare(t, c)
	var stdout, stderr bytes.Buffer
	code := RunDir(dir, c.Args, &stdout, &stderr)
	if stderr.Len() != 0 {
		t.Errorf("stderr: %s", stderr.String())
	}
	return collect(t, dir, inputs, code, stdout.String())
}

func runJava(t *testing.T, jar string, c corpusCase) outcome {
	t.Helper()
	dir, inputs := prepare(t, c)
	cmd := exec.Command("java", append([]string{"-cp", jar, "pcal.trans"}, c.Args...)...)
	cmd.Dir = dir
	// Java encodes its messages in the locale's charset, and tla-go always
	// writes UTF-8.
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	code := 0
	if err := cmd.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return collect(t, dir, inputs, code, stdout.String())
}

func compareOutcome(t *testing.T, what string, got, want outcome) {
	t.Helper()
	if got.Exit != want.Exit {
		t.Errorf("%s: exit %d, want %d", what, got.Exit, want.Exit)
	}
	if got.Stdout != want.Stdout {
		t.Errorf("%s: stdout\n%s\nwant\n%s", what, got.Stdout, want.Stdout)
	}
	if !maps.Equal(got.Same, want.Same) {
		t.Errorf("%s: unchanged files %v, want %v", what, got.Same, want.Same)
	}
	if !slices.Equal(got.Dirs, want.Dirs) {
		t.Errorf("%s: directories %v, want %v", what, got.Dirs, want.Dirs)
	}
	for _, name := range slices.Sorted(maps.Keys(want.Files)) {
		if got.Files[name] != want.Files[name] {
			t.Errorf("%s: file %s differs:\n%s", what, name, firstDifference(got.Files[name], want.Files[name]))
		}
	}
	for name := range got.Files {
		if _, ok := want.Files[name]; !ok {
			t.Errorf("%s: unexpected file %s", what, name)
		}
	}
}

func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n got: %q\nwant: %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("%d lines, want %d", len(g), len(w))
}

// TestJavaPlusCalCorpus translates the upstream PlusCal corpus, the inline
// sources of the upstream pcal JUnit tests, and inputs written for tla-go,
// and compares every output byte with Java's pcal.trans. The expected
// outputs were produced by Java and are checked in; with TLC_JAR set, the
// test also runs Java. TLC_JAR with PCAL_UPDATE_GOLDEN=1 rewrites them.
func TestJavaPlusCalCorpus(t *testing.T) {
	cases := loadCases(t)
	golden := loadGolden(t)
	jar := os.Getenv("TLC_JAR")
	update := jar != "" && os.Getenv("PCAL_UPDATE_GOLDEN") != ""
	insensitive := useCaseInsensitiveGolden(t, golden)
	if update && insensitive {
		t.Fatal("regenerate the default golden on a case-sensitive filesystem")
	}
	updateInsensitive := os.Getenv("PCAL_UPDATE_CASE_INSENSITIVE") != ""
	if updateInsensitive && (jar == "" || !insensitive) {
		t.Fatal("updating the case-insensitive golden requires TLC_JAR and a case-insensitive filesystem")
	}
	var mu sync.Mutex
	updated := map[string]outcome{}
	t.Run("cases", func(t *testing.T) {
		for _, c := range cases {
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				got := runGo(t, c)
				if jar != "" {
					java := runJava(t, jar, c)
					compareOutcome(t, "Java", got, java)
					if updateInsensitive && c.Name == "cli-upper" {
						writeCaseInsensitiveGolden(t, java)
					}
					mu.Lock()
					updated[c.Name] = java
					mu.Unlock()
				}
				if !update && !(updateInsensitive && c.Name == "cli-upper") {
					want, ok := golden[c.Name]
					if !ok {
						t.Fatalf("no golden output; run with TLC_JAR and PCAL_UPDATE_GOLDEN=1")
					}
					compareOutcome(t, "golden", got, want)
				}
			})
		}
	})
	if update {
		writeGolden(t, updated)
	}
}
