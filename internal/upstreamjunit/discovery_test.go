package upstreamjunit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDiscovery(t *testing.T) {
	got, err := parseDiscovery("noise\nINVENTORY\tcase\tQw==\tbSgxKQ==\n")
	if err != nil || len(got) != 1 || got[0] != (DiscoveryRecord{"case", "C", "m(1)"}) {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := parseDiscovery("INVENTORY\tcase\t!\tbQ==\n"); err == nil {
		t.Fatal("accepted malformed record")
	}
}

func TestJUnitDiscoveryFixture(t *testing.T) {
	junit, hamcrest := os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	if junit == "" || hamcrest == "" {
		t.Skip("set JUnit and Hamcrest jar paths for runtime fixture")
	}
	for _, dep := range []struct{ path, hash string }{{junit, junitSHA}, {hamcrest, hamcrestSHA}} {
		if err := verifyDependency(dep.path, dep.hash); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	files := map[string]string{
		"Base.java":      `import org.junit.*; public class Base { @Test public void inherited() {} @Ignore @Test public void skipped() {} }`,
		"Child.java":     `public class Child extends Base { @org.junit.Test public void own() {} }`,
		"Params.java":    `import org.junit.*; import org.junit.runner.*; import org.junit.runners.*; @RunWith(Parameterized.class) public class Params { @Parameterized.Parameters public static Object[] data(){return new Object[]{1,2};} public Params(int i){} @Test public void x(){} }`,
		"Ignored.java":   `@org.junit.Ignore public class Ignored extends Base {}`,
		"Broken.java":    `public class Broken { @org.junit.Test public void bad(int x) {} }`,
		"Nested.java":    `import org.junit.runner.RunWith; import org.junit.runners.Suite; @RunWith(Suite.class) @Suite.SuiteClasses({Broken.class, Named.class}) public class Nested {}`,
		"Named.java":     `public class Named { @org.junit.Test public void initializationError() {} }`,
		"Ambiguous.java": `import org.junit.runner.*; import org.junit.runner.notification.RunNotifier; @RunWith(Ambiguous.R.class) public class Ambiguous { public static class R extends Runner { public R(Class<?> c) {} public Description getDescription() { return Description.createTestDescription(Ambiguous.class, "initializationError"); } public void run(RunNotifier n) {} } }`,
		"Discovery.java": discoveryJava,
	}
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	got, err := runDiscovery(paths, []string{junit, hamcrest}, dir, []string{"Child", "Params", "Ignored", "Broken", "Nested", "Named", "Ambiguous"})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range got {
		counts[r.Kind]++
	}
	if counts["case"] != 6 || counts["ignored"] != 1 || counts["ignored-class"] != 1 || counts["error"] != 3 {
		t.Fatalf("counts %v records %+v", counts, got)
	}
	for _, want := range []DiscoveryRecord{
		{"error", "Nested", "runner initialization error or ambiguous description: initializationError(Broken)"},
		{"case", "Nested", "initializationError(Named)"},
		{"case", "Named", "initializationError(Named)"},
		{"error", "Ambiguous", "runner initialization error or ambiguous description: initializationError(Ambiguous)"},
	} {
		found := false
		for _, record := range got {
			found = found || record == want
		}
		if !found {
			t.Errorf("missing record %+v in %+v", want, got)
		}
	}
}

func TestUpstreamJUnitDiscovery(t *testing.T) {
	if os.Getenv("TLA_GO_JUNIT_DISCOVERY") != "1" {
		t.Skip("set TLA_GO_JUNIT_DISCOVERY=1")
	}
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	junit, hamcrest := os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	if root == "" || jar == "" || junit == "" || hamcrest == "" {
		t.Fatal("set root, TLC jar, JUnit jar and Hamcrest jar")
	}
	for _, p := range []*string{&root, &jar, &junit, &hamcrest} {
		abs, err := filepath.Abs(*p)
		if err != nil {
			t.Fatal(err)
		}
		*p = abs
	}
	if err := VerifyInputs(root, jar); err != nil {
		t.Fatal(err)
	}
	classes, err := inventoryTracked(filepath.Join(root, "tlatools", "org.lamport.tlatools", "test"), UpstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range classes {
		if !c.Abstract {
			names = append(names, c.Name)
		}
	}
	records, err := discover(root, jar, junit, hamcrest, t.TempDir(), names)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range records {
		counts[r.Kind]++
		t.Logf("%s %s %s", r.Kind, r.Class, r.Display)
	}
	t.Logf("lexical non-abstract classes=%d; JUnit leaves: %s; runner errors and ignored classes are unresolved", len(names), strings.TrimSpace(fmtCounts(counts)))
}

func fmtCounts(c map[string]int) string {
	return fmt.Sprintf("cases=%d ignored=%d ignored-classes=%d errors=%d", c["case"], c["ignored"], c["ignored-class"], c["error"])
}
