package upstreamjunit

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

//go:embed Discovery.java
var discoveryJava string

const (
	junitSHA    = "59721f0805e223d84b90677887d9ff567dc534d7c502ca903c0c2b17f05c116a"
	hamcrestSHA = "66fdef91e9739348df7a096aa384a5685f4e875584cce89386a7a47251c4d8e9"
)

// DiscoveryRecord describes a JUnit runner leaf for the selected candidate Class.
// Reason is the @Ignore value when the runner exposes one.
type DiscoveryRecord struct{ Kind, Class, Display, Reason string }

// JUnitInventory groups runner descriptions by selected candidate, not by model.
// An ignored class is a single description, whose method count is unknown.
type JUnitInventory struct {
	Cases, Ignored, IgnoredClasses, Empty, Uncertain, Errors []DiscoveryRecord
}

func inventoryByClass(records []DiscoveryRecord) map[string]JUnitInventory {
	result := make(map[string]JUnitInventory)
	for _, r := range records {
		v := result[r.Class]
		switch r.Kind {
		case "case":
			v.Cases = append(v.Cases, r)
		case "ignored":
			v.Ignored = append(v.Ignored, r)
		case "ignored-class":
			v.IgnoredClasses = append(v.IgnoredClasses, r)
		case "empty":
			v.Empty = append(v.Empty, r)
		case "uncertain":
			v.Uncertain = append(v.Uncertain, r)
		case "error":
			v.Errors = append(v.Errors, r)
		}
		result[r.Class] = v
	}
	return result
}

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
	if len(parts) != 5 {
		return DiscoveryRecord{}, fmt.Errorf("malformed discovery record: %q", line)
	}
	switch parts[1] {
	case "case", "ignored", "ignored-class", "empty", "uncertain", "error":
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
	reason, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		return DiscoveryRecord{}, err
	}
	return DiscoveryRecord{parts[1], string(c), string(d), string(reason)}, nil
}

// discover runs the actual JUnit runner on explicitly selected classes. It
// compiles the pinned tracked test tree, never untracked Java sources.
func discover(ctx context.Context, root, jar, junit, hamcrest, temp string, names []string) ([]DiscoveryRecord, error) {
	if err := verifyInputs(ctx, root, jar); err != nil {
		return nil, err
	}
	for _, dep := range []struct{ path, hash string }{{junit, junitSHA}, {hamcrest, hamcrestSHA}} {
		if err := verifyDependency(dep.path, dep.hash); err != nil {
			return nil, err
		}
	}
	testRoot := filepath.Join(root, "tlatools", "org.lamport.tlatools", "test")
	out, err := gitOutput(ctx, "-C", testRoot, "ls-tree", "-r", "--name-only", "-z", UpstreamCommit, "--", ".")
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
	return runDiscoveryContext(ctx, sources, []string{jar, junit, hamcrest}, temp, names)
}

func runDiscovery(sources, deps []string, temp string, names []string) ([]DiscoveryRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runDiscoveryContext(ctx, sources, deps, temp, names)
}

func runDiscoveryContext(ctx context.Context, sources, deps []string, temp string, names []string) ([]DiscoveryRecord, error) {
	protocolDir, err := os.MkdirTemp(temp, "discovery-protocol-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(protocolDir)
	protocolPath := filepath.Join(protocolDir, "inventory")
	classes := filepath.Join(temp, "classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		return nil, err
	}
	args := append([]string{"-proc:none", "-encoding", "UTF-8", "-cp", strings.Join(deps, string(os.PathListSeparator)), "-d", classes}, sources...)
	cmd := discoveryCommand(ctx, "javac", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, discoveryCommandError(ctx, "javac", err, out)
	}
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	cmd = discoveryCommand(ctx, "java", "-cp", cp, "Discovery", protocolPath)
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, discoveryCommandError(ctx, "discovery", err, out)
	}
	protocol, err := os.ReadFile(protocolPath)
	if err != nil {
		return nil, fmt.Errorf("read discovery protocol: %w\n%s", err, out)
	}
	records, err := parseDiscovery(string(protocol))
	if err != nil {
		return nil, fmt.Errorf("parse discovery protocol: %w\n%s", err, out)
	}
	if err := validateDiscoveryNames(records, names); err != nil {
		return nil, fmt.Errorf("%w\n%s", err, out)
	}
	return records, nil
}

func discoveryCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	if name == "java" {
		javatmp.Apply(cmd)
	}
	// A runner's descendants may keep the output pipes open after the JVM exits.
	cmd.WaitDelay = time.Second
	return cmd
}

func discoveryCommandError(ctx context.Context, stage string, err error, out []byte) error {
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return fmt.Errorf("%s: %w\n%s", stage, err, out)
}

func validateDiscoveryNames(records []DiscoveryRecord, names []string) error {
	seen := map[string]bool{}
	for _, record := range records {
		seen[record.Class] = true
	}
	for _, name := range names {
		if !seen[name] {
			return fmt.Errorf("no discovery result for %s", name)
		}
	}
	return nil
}
