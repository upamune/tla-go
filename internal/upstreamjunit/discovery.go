package upstreamjunit

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed Discovery.java
var discoveryJava string

const (
	junitSHA    = "59721f0805e223d84b90677887d9ff567dc534d7c502ca903c0c2b17f05c116a"
	hamcrestSHA = "66fdef91e9739348df7a096aa384a5685f4e875584cce89386a7a47251c4d8e9"
)

type DiscoveryRecord struct{ Kind, Class, Display string }

func verifyDependency(path, hash string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(b)) != hash {
		return fmt.Errorf("unexpected SHA-256 for %s", path)
	}
	return nil
}

func parseDiscovery(output string) ([]DiscoveryRecord, error) {
	var records []DiscoveryRecord
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "INVENTORY\t") {
			continue
		}
		record, err := parseDiscoveryLine(line)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func parseDiscoveryLine(line string) (DiscoveryRecord, error) {
	parts := strings.Split(line, "\t")
	if len(parts) != 4 {
		return DiscoveryRecord{}, fmt.Errorf("malformed discovery record: %q", line)
	}
	switch parts[1] {
	case "case", "ignored", "ignored-class", "error":
	default:
		return DiscoveryRecord{}, fmt.Errorf("unknown discovery kind: %q", parts[1])
	}
	c, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return DiscoveryRecord{}, err
	}
	d, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return DiscoveryRecord{}, err
	}
	return DiscoveryRecord{parts[1], string(c), string(d)}, nil
}

// discover runs the actual JUnit runner on explicitly selected classes. It
// compiles the pinned tracked test tree, never untracked Java sources.
func discover(root, jar, junit, hamcrest, temp string, names []string) ([]DiscoveryRecord, error) {
	if err := VerifyInputs(root, jar); err != nil {
		return nil, err
	}
	for _, dep := range []struct{ path, hash string }{{junit, junitSHA}, {hamcrest, hamcrestSHA}} {
		if err := verifyDependency(dep.path, dep.hash); err != nil {
			return nil, err
		}
	}
	testRoot := filepath.Join(root, "tlatools", "org.lamport.tlatools", "test")
	cmd := exec.Command("git", "-C", testRoot, "ls-tree", "-r", "--name-only", "-z", UpstreamCommit, "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sources []string
	for name := range strings.SplitSeq(string(out), "\x00") {
		if strings.HasSuffix(name, ".java") {
			sources = append(sources, filepath.Join(testRoot, filepath.FromSlash(name)))
		}
	}
	sort.Strings(sources)
	helper := filepath.Join(temp, "Discovery.java")
	if err := os.WriteFile(helper, []byte(discoveryJava), 0o600); err != nil {
		return nil, err
	}
	sources = append(sources, helper)
	return runDiscovery(sources, []string{jar, junit, hamcrest}, temp, names)
}

func runDiscovery(sources, deps []string, temp string, names []string) ([]DiscoveryRecord, error) {
	classes := filepath.Join(temp, "classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		return nil, err
	}
	args := append([]string{"-proc:none", "-encoding", "UTF-8", "-cp", strings.Join(deps, string(os.PathListSeparator)), "-d", classes}, sources...)
	cmd := exec.Command("javac", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("javac: %w\n%s", err, out)
	}
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	cmd = exec.Command("java", "-cp", cp, "Discovery")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("discovery: %w\n%s", err, out)
	}
	records, err := parseDiscovery(string(out))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, record := range records {
		seen[record.Class] = true
	}
	for _, name := range names {
		if !seen[name] {
			return nil, fmt.Errorf("no discovery result for %s", name)
		}
	}
	return records, nil
}
