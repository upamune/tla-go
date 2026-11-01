package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// This is a partial differential, not an execution or pass of Java JUnit.
func TestPinnedSuiteTest10Partial(t *testing.T) {
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Skip("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
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
	binary := filepath.Join(t.TempDir(), "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	c := Case{
		Name: "tlc2.tool.suite.Test10", Model: "test10", Path: "suite", Coverage: true, Dump: true,
		IDs: []string{"2199", "2190"}, PresenceIDs: []string{"2186", "1000"},
	}
	j, g, differences, err := RunCase(root, jar, binary, c, 30*time.Second)
	if err != nil || len(differences) > 0 {
		t.Fatalf("partial differential: differences=%v error=%v", differences, err)
	}
	observations := []AssertedObservation{
		{ID: "2186", Present: true},
		{ID: "2199", Present: true, Values: []string{"2", "1", "0"}},
		{ID: "2190", Present: true, Values: []string{"1"}},
		{ID: "1000", Present: false},
	}
	for name, result := range map[string]Result{"Java": j, "Go": g} {
		failures, omissions, err := CheckAssertions(result, observations)
		if err != nil || len(failures) > 0 || len(omissions) > 0 || result.ExitCode != 0 {
			t.Errorf("%s: exit=%d failures=%v omissions=%v error=%v", name, result.ExitCode, failures, omissions, err)
		}
	}
	zero, err := ZeroCoverage(j)
	if err != nil || len(zero) != 0 {
		t.Errorf("Java zero coverage: %v, %v", zero, err)
	}
	t.Log("Go coverage assertion and exact JUnit lifecycle remain unverified; this is not a JUnit pass")
}

// CCallReturn1's translation and selected TLC messages are compared, not its
// Java override, coverage assertions, or full JUnit lifecycle.
func TestPinnedCCallReturn1TemporalProgress(t *testing.T) {
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Skip("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
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
	binary := filepath.Join(t.TempDir(), "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	c := Case{Name: "pcal.CCallReturn1Test", Model: "CCallReturn1", Path: "pcal", Workers: "1", Coverage: true, Dump: true, Translate: true, TranslationArgs: []string{"-wf", "-termination"}, IDs: []string{"2190", "2192", "2199", "2194"}, PresenceIDs: []string{"2186", "1000"}}
	j, g, differences, err := RunCase(root, jar, binary, c, 30*time.Second)
	if err != nil || len(differences) != 0 || j.ExitCode != 0 || g.ExitCode != 0 {
		t.Fatalf("partial differential: differences=%v error=%v Java exit=%d Go exit=%d", differences, err, j.ExitCode, g.ExitCode)
	}
	for name, result := range map[string]Result{"Java": j, "Go": g} {
		failures, omissions, err := CheckAssertions(result, []AssertedObservation{{ID: "2192", Present: true, Values: []string{"complete", "8"}}})
		if err != nil || len(failures) != 0 || len(omissions) != 0 {
			t.Errorf("%s: failures=%v omissions=%v error=%v", name, failures, omissions, err)
		}
	}
}

// Effectful PlusCal termination models still require Java's late tableau
// expansion. A missing Go diagnostic is intentional until that is modeled.
func TestPinnedEffectfulPlusCalTableauProgressGuard(t *testing.T) {
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Skip("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
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
	binary := filepath.Join(t.TempDir(), "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, tc := range []struct {
		model, count string
	}{
		{"Factorial2", "12"},
		{"EvenOdd", "13"},
		{"EvenOddBad", "13"},
		{"InnerLabeledIf", "16"},
		{"NoLoop2", "9"},
		{"Euclid3", "94"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			model := tc.model
			c := Case{Name: "pcal." + model + "Test", Model: model, Path: "pcal", Workers: "1", Coverage: true, Dump: true, Translate: true, TranslationArgs: []string{"-wf", "-termination"}, IDs: []string{"2192"}, PresenceIDs: []string{"2267"}}
			j, g, diffs, err := RunCase(root, jar, binary, c, 30*time.Second)
			if err != nil || j.ExitCode != 0 || g.ExitCode != 0 {
				t.Fatalf("Java exit %d Go exit %d: %v", j.ExitCode, g.ExitCode, err)
			}
			if !reflect.DeepEqual(diffs, []string{"message:2192", "presence:2267"}) {
				t.Fatalf("unexpected differences (including translation and dump): %v", diffs)
			}
			failures, omissions, assertionErr := CheckAssertions(j, []AssertedObservation{{ID: "2192", Present: true, Values: []string{"complete", tc.count}}})
			if assertionErr != nil || len(failures) != 0 || len(omissions) != 0 {
				t.Fatalf("Java upstream 2192 assertion: failures=%v omissions=%v error=%v", failures, omissions, assertionErr)
			}
			jm, je := Messages(j.Output)
			gm, ge := Messages(g.Output)
			if je != nil || ge != nil || len(jm["2192"]) == 0 || len(jm["2267"]) == 0 || len(gm["2192"]) != 0 || len(gm["2267"]) != 0 {
				t.Fatalf("Java 2192/2267 %v/%v Go %v/%v errors %v/%v", jm["2192"], jm["2267"], gm["2192"], gm["2267"], je, ge)
			}
		})
	}
}
