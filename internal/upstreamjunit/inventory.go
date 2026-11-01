package upstreamjunit

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	UpstreamCommit = "5a47802b5c391f59ecdd44117981f4ff8c0656ba"
	JarSHA256      = "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88"
)

// VerifyInputs verifies both revision identity and jar contents. It rejects a
// modified tracked checkout because its fixtures may not match that revision.
func VerifyInputs(root, jar string) error {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("upstream revision: %w", err)
	}
	if strings.TrimSpace(string(out)) != UpstreamCommit {
		return fmt.Errorf("upstream revision %s, want %s", strings.TrimSpace(string(out)), UpstreamCommit)
	}
	cmd = exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=no")
	out, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("upstream status: %w", err)
	}
	if len(out) != 0 {
		return fmt.Errorf("upstream checkout has tracked modifications")
	}
	f, err := os.Open(jar)
	if err != nil {
		return fmt.Errorf("oracle: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("oracle: %w", err)
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != JarSHA256 {
		return fmt.Errorf("oracle SHA-256 does not match %s", JarSHA256)
	}
	return nil
}

// Class is a candidate, not a resolved executable case. Source is relative to
// the test root; Kind is the first known base in its inheritance chain.
type Class struct {
	Name, Parent, Source, Kind string
	Abstract                   bool
}

var (
	classRE   = regexp.MustCompile(`(?m)\b(abstract\s+)?class\s+([A-Za-z_$][\w$]*)\s+extends\s+([A-Za-z_$][\w$.]*)`)
	packageRE = regexp.MustCompile(`(?m)\bpackage\s+([\w.]+)\s*;`)
	commentRE = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`)
)

// Inventory discovers candidate subclasses, including indirect inheritance.
// It intentionally does not claim that candidates are executable: annotations,
// constructor arguments and runtime setup still need semantic resolution.
func Inventory(testRoot string) ([]Class, error) {
	var paths []string
	err := filepath.WalkDir(testRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".java") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inventoryPaths(testRoot, paths)
}

// inventoryTracked lists Java sources in the commit tree, not the working tree
// or index. The caller must verify that the checkout matches the commit first.
func inventoryTracked(testRoot, commit string) ([]Class, error) {
	cmd := exec.Command("git", "-C", testRoot, "ls-tree", "-r", "--name-only", "-z", commit, "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("upstream tree: %w", err)
	}
	var paths []string
	for name := range strings.SplitSeq(string(out), "\x00") {
		if strings.HasSuffix(name, ".java") {
			paths = append(paths, filepath.Join(testRoot, filepath.FromSlash(name)))
		}
	}
	return inventoryPaths(testRoot, paths)
}

func inventoryPaths(testRoot string, paths []string) ([]Class, error) {
	var all []Class
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		clean := commentRE.ReplaceAllStringFunc(string(data), func(s string) string {
			if strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") {
				return " "
			}
			return `""`
		})
		pkg := packageRE.FindStringSubmatch(clean)
		if pkg == nil {
			continue
		}
		rel, err := filepath.Rel(testRoot, path)
		if err != nil {
			return nil, err
		}
		for _, m := range classRE.FindAllStringSubmatch(clean, -1) {
			all = append(all, Class{Name: pkg[1] + "." + m[2], Parent: m[3], Source: filepath.ToSlash(rel), Abstract: m[1] != ""})
		}
	}
	return classify(all), nil
}

func classify(all []Class) []Class {
	byName := make(map[string]Class)
	bySimple := make(map[string][]Class)
	for _, c := range all {
		byName[c.Name] = c
		i := strings.LastIndex(c.Name, ".")
		bySimple[c.Name[i+1:]] = append(bySimple[c.Name[i+1:]], c)
	}
	bases := map[string]string{"ModelCheckerTestCase": "ModelChecker", "PCalModelCheckerTestCase": "PCal", "SuiteTestCase": "Suite", "SuiteETestCase": "SuiteE"}
	var result []Class
	for _, c := range all {
		if kind := classKind(c, bases, byName, bySimple); kind != "" {
			c.Kind = kind
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func classKind(c Class, bases map[string]string, byName map[string]Class, bySimple map[string][]Class) string {
	seen := map[string]bool{c.Name: true}
	for {
		simple := c.Parent
		if i := strings.LastIndex(simple, "."); i >= 0 {
			simple = simple[i+1:]
		}
		if kind, ok := bases[simple]; ok {
			return kind
		}
		next, ok := findParent(c, simple, byName, bySimple)
		if !ok || seen[next.Name] {
			return ""
		}
		seen[next.Name] = true
		c = next
	}
}

func findParent(cur Class, simple string, byName map[string]Class, bySimple map[string][]Class) (Class, bool) {
	next, ok := byName[cur.Parent]
	if !ok {
		pkg := cur.Name[:strings.LastIndex(cur.Name, ".")+1]
		next, ok = byName[pkg+cur.Parent]
	}
	if !ok && len(bySimple[simple]) == 1 {
		return bySimple[simple][0], true
	}
	return next, ok
}
