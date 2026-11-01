package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUpstreamJUnitSweep(t *testing.T) {
	if os.Getenv("TLA_GO_JUNIT_SWEEP") != "1" {
		t.Skip("set TLA_GO_JUNIT_SWEEP=1 to enable the incomplete upstream sweep")
	}
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Fatal("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInputs(root, jar); err != nil {
		t.Fatal(err)
	}
	classes, err := inventoryTracked(filepath.Join(root, "tlatools", "org.lamport.tlatools", "test"), UpstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "tla-go")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	testRoot := filepath.Join(root, "tlatools", "org.lamport.tlatools", "test")
	ec, err := os.ReadFile(filepath.Join(root, "tlatools", "org.lamport.tlatools", "src", "tlc2", "output", "EC.java"))
	if err != nil {
		t.Fatal(err)
	}
	limit := 0
	if value := os.Getenv("TLA_GO_JUNIT_LIMIT"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 0 {
			t.Fatal("invalid TLA_GO_JUNIT_LIMIT")
		}
	}
	recovered, attempted, unresolved, unsafeCount := 0, 0, 0, 0
	for _, class := range classes {
		if class.Abstract {
			continue
		}
		r := Resolve(testRoot, classes, class)
		if r.Case.Model != "" {
			recovered++
		}
		source, readErr := os.ReadFile(filepath.Join(testRoot, filepath.FromSlash(class.Source)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		ids, presence, omitted := AssertedIDs(string(source), string(ec))
		observations, assertionOmissions := ParseAssertions(string(source), string(ec))
		observations = append(observations, r.Inherited...)
		for _, obs := range r.Inherited {
			if len(obs.Values) > 0 {
				ids = append(ids, obs.ID)
			} else {
				presence = append(presence, obs.ID)
			}
		}
		r.Reasons = append(r.Reasons, omitted...)
		r.Reasons = append(r.Reasons, assertionOmissions...)
		r.Reasons = append(r.Reasons, "Go liveness strategy not verified against JUnit final-only checks")
		r.Case.Path, r.Case.Deadlock, r.Case.Checkpoint = r.Path, r.Deadlock, r.Checkpoint
		r.Case.Coverage, r.Case.Dump = r.Coverage, r.Dump
		r.Case.IDs, r.Case.PresenceIDs = ids, presence
		r.Case.Translate = r.Translate
		unresolved++
		t.Logf("unresolved %s spec=%q path=%q args=%q pcalArgs=%q workers=%s deadlock=%t coverage=%t dump=%t translate=%t reasons=%v", class.Name, r.Case.Model, r.Path, r.Case.Args, r.Case.TranslationArgs, r.Case.Workers, r.Deadlock, r.Coverage, r.Dump, r.Translate, r.Reasons)
		// Only run invocations with fully resolved constructor and setup expressions.
		unsafe := false
		for _, reason := range r.Reasons {
			if strings.HasPrefix(reason, "dynamic ") || strings.HasPrefix(reason, "invalid path:") || strings.HasPrefix(reason, "constructor ") || strings.HasPrefix(reason, "ambiguous constructor") || strings.HasPrefix(reason, "unterminated constructor") || strings.HasPrefix(reason, "unsupported ") || strings.Contains(reason, "parent not resolved") || strings.Contains(reason, "custom harness hook") {
				unsafe = true
			}
		}
		if unsafe || (r.Path != "" && !filepath.IsLocal(r.Path)) {
			unsafeCount++
			continue
		}
		if limit == 0 || attempted >= limit || r.Case.Model == "" || (os.Getenv("TLA_GO_JUNIT_FILTER") != "" && !strings.Contains(class.Name, os.Getenv("TLA_GO_JUNIT_FILTER"))) {
			continue
		}
		attempted++
		j, g, diffs, runErr := RunCase(root, jar, binary, r.Case, 30*time.Second)
		if runErr != nil {
			t.Logf("diagnostic %s comparison unavailable: %v\nJava output:\n%s\nGo output:\n%s", class.Name, runErr, j.Output, g.Output)
			continue
		}
		jf, jo, je := CheckAssertions(j, observations)
		gf, goOmissions, ge := CheckAssertions(g, observations)
		t.Logf("assertions %s Java failures=%v Go failures=%v omissions=%v/%v errors=%v/%v", class.Name, jf, gf, jo, goOmissions, je, ge)
		if class.Name == "tlc2.tool.suite.Test10" {
			zero, coverageErr := ZeroCoverage(j)
			t.Logf("Test10 inherited assertions: expected exit=0 Java=%d Go=%d; Java zero coverage=%v error=%v; Go zero coverage unavailable", j.ExitCode, g.ExitCode, zero, coverageErr)
		}
		t.Logf("diagnostic %s differences=%v selected bodies=%v presence=%v; partial comparison, liveness strategy unresolved, not a match; Java exit=%d Go exit=%d", class.Name, diffs, ids, presence, j.ExitCode, g.ExitCode)
		if len(diffs) > 0 {
			jm, _ := Messages(j.Output)
			gm, _ := Messages(g.Output)
			for _, diff := range diffs {
				if id, ok := strings.CutPrefix(diff, "message:"); ok {
					t.Logf("difference %s %s Java=%q Go=%q", class.Name, id, jm[id], gm[id])
				}
			}
		}
	}
	t.Errorf("incomplete sweep: %d candidates with recovered spec, %d diagnostic runs, %d candidates with unsafe or unresolved setup, %d unresolved non-abstract candidates; inventory is not an exhaustive JUnit count (goal: 336)", recovered, attempted, unsafeCount, unresolved)
}
