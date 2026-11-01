package upstreamjunit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestParseDiscovery(t *testing.T) {
	got, err := parseDiscovery("noise\nINVENTORY\tcase\tQw==\tbSgxKQ==\t\n")
	if err != nil || len(got) != 1 || got[0] != (DiscoveryRecord{"case", "C", "m(1)", ""}) {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := parseDiscovery("INVENTORY\tcase\t!\tbQ==\t\n"); err == nil {
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
		"Base.java":        `import org.junit.*; public class Base { @Test public void inherited() {} @Ignore("inherited reason") @Test public void skipped() {} }`,
		"Child.java":       `public class Child extends Base { @org.junit.Test public void own() {} }`,
		"Params.java":      `import org.junit.*; import org.junit.runner.*; import org.junit.runners.*; @RunWith(Parameterized.class) public class Params { @Parameterized.Parameters public static Object[] data(){return new Object[]{1,2};} public Params(int i){} @Test public void x(){} @Ignore("param reason") @Test public void skip(){} }`,
		"Ignored.java":     `@org.junit.Ignore("class reason") public class Ignored extends Base {}`,
		"Broken.java":      `public class Broken { @org.junit.Test public void bad(int x) {} }`,
		"Nested.java":      `import org.junit.runner.RunWith; import org.junit.runners.Suite; @RunWith(Suite.class) @Suite.SuiteClasses({Broken.class, Named.class}) public class Nested {}`,
		"WithIgnored.java": `import org.junit.runner.RunWith; import org.junit.runners.Suite; @RunWith(Suite.class) @Suite.SuiteClasses({Ignored.class, Child.class}) public class WithIgnored {}`,
		"Named.java":       `public class Named { @org.junit.Test public void initializationError() {} }`,
		"Ambiguous.java":   `import org.junit.runner.*; import org.junit.runner.notification.RunNotifier; @RunWith(Ambiguous.R.class) public class Ambiguous { public static class R extends Runner { public R(Class<?> c) {} public Description getDescription() { return Description.createTestDescription(Ambiguous.class, "initializationError"); } public void run(RunNotifier n) {} } }`,
		"Noisy.java":       `import org.junit.runner.*; import org.junit.runner.notification.RunNotifier; @RunWith(Noisy.R.class) public class Noisy { public static class R extends Runner { public R(Class<?> c) { System.out.print("no newline"); System.err.print("stderr diagnostic"); System.out.print("\nINVENTORY\tcase\tTm9pc3k=\tZm9yZ2Vk\t\n"); } public Description getDescription() { return Description.createTestDescription(Noisy.class, "real"); } public void run(RunNotifier n) { Description d = getDescription(); n.fireTestStarted(d); n.fireTestFinished(d); } } public static void main(String[] args) { Result r = new JUnitCore().run(Noisy.class); System.out.println("RESULT=" + r.getRunCount() + "," + r.getFailureCount()); } }`,
		"ExitRunner.java":  `import org.junit.runner.*; import org.junit.runner.notification.RunNotifier; @RunWith(ExitRunner.R.class) public class ExitRunner { public static class R extends Runner { public R(Class<?> c) { System.out.print("stdout diagnostic"); System.err.print("stderr diagnostic"); System.exit(7); } public Description getDescription() { return Description.EMPTY; } public void run(RunNotifier n) {} } }`,
		"SlowRunner.java":  `import org.junit.runner.*; import org.junit.runner.notification.RunNotifier; @RunWith(SlowRunner.R.class) public class SlowRunner { public static class R extends Runner { public R(Class<?> c) throws Exception { System.err.print("waiting diagnostic"); System.err.flush(); java.nio.file.Files.writeString(java.nio.file.Paths.get(System.getenv("DISCOVERY_SLOW_READY")), "ready"); Thread.sleep(120000); } public Description getDescription() { return Description.EMPTY; } public void run(RunNotifier n) {} } }`,
		"Discovery.java":   discoveryJava,
	}
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	got, err := runDiscovery(paths, []string{junit, hamcrest}, dir, []string{"Child", "Params", "Ignored", "Broken", "Nested", "Named", "Ambiguous", "WithIgnored", "Noisy"})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range got {
		counts[r.Kind]++
	}
	if counts["case"] != 9 || counts["ignored"] != 1 || counts["ignored-class"] != 1 || counts["uncertain"] != 4 || counts["error"] != 3 {
		t.Fatalf("counts %v records %+v", counts, got)
	}
	for _, want := range []DiscoveryRecord{
		{"ignored", "Child", "skipped(Child)", "inherited reason"},
		{"uncertain", "WithIgnored", "skipped(Child)", "inherited reason"},
		{"uncertain", "Params", "skip[0](Params)", "param reason"},
		{"uncertain", "Params", "skip[1](Params)", "param reason"},
		{"ignored-class", "Ignored", "Ignored", "class reason"},
		{"uncertain", "WithIgnored", "Ignored", "class reason"},
		{"error", "Nested", "runner initialization error or ambiguous description: initializationError(Broken)", ""},
		{"case", "Nested", "initializationError(Named)", ""},
		{"case", "Named", "initializationError(Named)", ""},
		{"case", "Noisy", "real(Noisy)", ""},
		{"error", "Ambiguous", "runner initialization error or ambiguous description: initializationError(Ambiguous)", ""},
	} {
		found := false
		for _, record := range got {
			found = found || record == want
		}
		if !found {
			t.Errorf("missing record %+v in %+v", want, got)
		}
	}
	cp := strings.Join([]string{filepath.Join(dir, "classes"), junit, hamcrest}, string(os.PathListSeparator))
	out, err := javatmp.Apply(exec.Command("java", "-cp", cp, "Noisy")).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "RESULT=1,0") {
		t.Fatalf("JUnitCore: %v\n%s", err, out)
	}
	_, err = runDiscovery(paths, []string{junit, hamcrest}, dir, []string{"ExitRunner"})
	if err == nil || !strings.Contains(err.Error(), "stdout diagnostic") || !strings.Contains(err.Error(), "stderr diagnostic") {
		t.Fatalf("missing process diagnostics: %v", err)
	}
	marker := filepath.Join(dir, "slow-runner-ready")
	t.Setenv("DISCOVERY_SLOW_READY", marker)
	err = cancelAfterReady(t, marker, func(ctx context.Context) error {
		_, err := runDiscoveryContext(ctx, paths, []string{junit, hamcrest}, dir, []string{"SlowRunner"})
		return err
	})
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "waiting diagnostic") {
		t.Fatalf("cancellation or diagnostics missing: %v", err)
	}
	protocolDirs, err := filepath.Glob(filepath.Join(dir, "discovery-protocol-*"))
	if err != nil || len(protocolDirs) != 0 {
		t.Fatalf("protocol directories left behind: %v, %v", protocolDirs, err)
	}
}

// Cancel only after the fixture reaches the operation under test. The watchdog
// bounds setup independently from compilation and process startup timing.
func cancelAfterReady(t *testing.T, marker string, run func(context.Context) error) error {
	t.Helper()
	watchdog, stop := context.WithTimeout(context.Background(), 45*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(watchdog)
	done := make(chan error, 1)
	received := false
	go func() { done <- run(ctx) }()
	defer func() {
		cancel()
		if !received {
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				t.Error("fixture did not stop after cancellation")
			}
		}
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			received = true
			t.Fatalf("fixture stopped before readiness: %v", err)
		case <-watchdog.Done():
			t.Fatal("fixture did not become ready before the watchdog deadline")
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case err := <-done:
		received = true
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("fixture did not stop after cancellation")
		return nil
	}
}

func TestDiscoveryCancellationAndCleanup(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runDiscoveryContext(ctx, nil, nil, dir, []string{"Example"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "discovery-protocol-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("protocol directories after cancellation: %v, %v", paths, err)
	}
}

func TestDiscoveryCommandBoundsOutputWait(t *testing.T) {
	for _, name := range []string{"java", "javac"} {
		cmd := discoveryCommand(context.Background(), name)
		if cmd.WaitDelay != time.Second {
			t.Fatalf("%s output wait is unbounded: %s", name, cmd.WaitDelay)
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
	discoveryCtx, discoveryCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer discoveryCancel()
	if err := verifyInputs(discoveryCtx, root, jar); err != nil {
		t.Fatal(err)
	}
	classes, err := inventoryTrackedContext(discoveryCtx, filepath.Join(root, "tlatools", "org.lamport.tlatools", "test"), UpstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range classes {
		if !c.Abstract {
			names = append(names, c.Name)
		}
	}
	records, err := discover(discoveryCtx, root, jar, junit, hamcrest, t.TempDir(), names)
	if err != nil {
		t.Fatal(err)
	}
	byClass := inventoryByClass(records)
	example := byClass["tlc2.tool.TLCGetNonDeterminismTest"]
	if len(example.Cases) != 0 || len(example.Ignored) != 0 || len(example.Uncertain) != 1 || example.Uncertain[0].Display != "testSpec(tlc2.tool.TLCGetNonDeterminismTest)" || example.Uncertain[0].Reason != "No known fix/By design" || len(example.Errors) != 0 || len(example.IgnoredClasses) != 0 {
		t.Fatalf("pinned ignored method: %+v", example)
	}
	counts := map[string]int{}
	for _, r := range records {
		counts[r.Kind]++
		t.Logf("%s %s %s reason=%q", r.Kind, r.Class, r.Display, r.Reason)
	}
	t.Logf("lexical non-abstract classes=%d; JUnit leaves: %s; runner errors and ignored classes are unresolved", len(names), strings.TrimSpace(fmtCounts(counts)))
}

func fmtCounts(c map[string]int) string {
	return fmt.Sprintf("cases=%d ignored=%d ignored-classes=%d empty=%d uncertain=%d errors=%d", c["case"], c["ignored"], c["ignored-class"], c["empty"], c["uncertain"], c["error"])
}

func TestInventoryByClass(t *testing.T) {
	records := []DiscoveryRecord{
		{"case", "Mixed", "a(Mixed)", ""},
		{"ignored", "Mixed", "b(Mixed)", "reason"},
		{"ignored-class", "Suite", "Skipped", "class reason"},
		{"error", "Suite", "failure", ""},
	}
	got := inventoryByClass(records)
	if len(got["Mixed"].Cases) != 1 || len(got["Mixed"].Ignored) != 1 || len(got["Mixed"].IgnoredClasses) != 0 || got["Mixed"].Ignored[0].Reason != "reason" {
		t.Fatalf("mixed class: %+v", got["Mixed"])
	}
	if len(got["Suite"].IgnoredClasses) != 1 || len(got["Suite"].Errors) != 1 {
		t.Fatalf("suite: %+v", got["Suite"])
	}
}

func TestDiscoveryLifecycle(t *testing.T) {
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
		"Discovery.java": discoveryJava,
		"Lifecycle.java": `import org.junit.*; import org.junit.runner.*; import org.junit.runner.notification.*; import org.junit.runners.Suite;
public class Lifecycle {
 @RunWith(Custom.class) public static class CustomTest { @Ignore("leaf") @Test public void executes() {} }
 public static class Custom extends Runner {
  Description root, leaf;
  public Custom(Class<?> c) throws Exception { root = Description.createSuiteDescription(c); leaf = Description.createTestDescription(c, "executes", c.getMethod("executes").getAnnotations()); root.addChild(leaf); }
  public Description getDescription() { return root; }
  public void run(RunNotifier n) { n.fireTestStarted(leaf); n.fireTestFinished(leaf); }
 }
 @RunWith(Suite.class) @Suite.SuiteClasses({}) public static class Empty {}
 @RunWith(UnknownEmptyRunner.class) public static class UnknownEmpty {}
 public static class UnknownEmptyRunner extends Runner { public UnknownEmptyRunner(Class<?> c) {} public Description getDescription() { return Description.createSuiteDescription(UnknownEmpty.class); } public void run(RunNotifier n) {} }
 public static class Mixed { @Test public void runs() {} @Ignore("method") @Test public void skips() {} }
 @RunWith(Suite.class) @Suite.SuiteClasses({CustomTest.class, Mixed.class, Empty.class}) public static class Nested {}
 @RunWith(ParentAnnotationRunner.class) public static class ParentAnnotation {}
 @Ignore("parent") public static class AnnotationSource {}
 public static class ParentAnnotationRunner extends Runner {
  Description root, leaf;
  public ParentAnnotationRunner(Class<?> c) { root = Description.createSuiteDescription("parent", AnnotationSource.class.getAnnotations()); leaf = Description.createTestDescription(c, "runs"); root.addChild(leaf); }
  public Description getDescription() { return root; } public void run(RunNotifier n) { n.fireTestStarted(leaf); n.fireTestFinished(leaf); }
 }
 public static void main(String[] args) throws Exception { for (String name : args) { Result r = new JUnitCore().run(Class.forName(name)); System.out.println(name + "=" + r.getRunCount() + "," + r.getIgnoreCount() + "," + r.getFailureCount()); } }
}`,
	}
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	names := []string{"Lifecycle$CustomTest", "Lifecycle$Empty", "Lifecycle$UnknownEmpty", "Lifecycle$Mixed", "Lifecycle$Nested", "Lifecycle$ParentAnnotation"}
	records, err := runDiscovery(paths, []string{junit, hamcrest}, dir, names)
	if err != nil {
		t.Fatal(err)
	}
	inv := inventoryByClass(records)
	if len(inv[names[0]].Uncertain) != 1 || inv[names[0]].Uncertain[0].Reason != "leaf" || len(inv[names[1]].Empty) != 1 || len(inv[names[2]].Uncertain) != 1 || len(inv[names[3]].Cases) != 1 || len(inv[names[3]].Ignored) != 1 || len(inv[names[4]].Uncertain) != 3 || len(inv[names[4]].Cases) != 1 || len(inv[names[5]].Uncertain) != 1 || inv[names[5]].Uncertain[0].Reason != "parent" {
		t.Fatalf("inventory: %+v", inv)
	}
	cp := strings.Join([]string{filepath.Join(dir, "classes"), junit, hamcrest}, string(os.PathListSeparator))
	args := append([]string{"-cp", cp, "Lifecycle"}, names...)
	out, err := javatmp.Apply(exec.Command("java", args...)).CombinedOutput()
	if err != nil {
		t.Fatalf("JUnitCore: %v\n%s", err, out)
	}
	for _, want := range []string{"Lifecycle$CustomTest=1,0,0", "Lifecycle$Empty=0,0,0", "Lifecycle$UnknownEmpty=0,0,0", "Lifecycle$Mixed=1,1,0", "Lifecycle$Nested=2,1,0", "Lifecycle$ParentAnnotation=1,0,0"} {
		if !strings.Contains(strings.ReplaceAll(string(out), "\r\n", "\n"), want+"\n") {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}
