package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateExecution(t *testing.T) {
	pass := []executionEvent{{"started", "m(C)"}, {"finished", "m(C)"}, {"summary", "1,0,0"}}
	tests := []struct {
		name   string
		events []executionEvent
		ok     bool
	}{
		{"pass", pass, true},
		{"ignored", []executionEvent{{"ignored", "m(C)"}, {"summary", "0,1,0"}}, false},
		{"initialization error", []executionEvent{{"started", "initializationError(C)"}, {"failure", "bad"}, {"finished", "initializationError(C)"}, {"summary", "1,0,1"}}, false},
		{"assertion failure", []executionEvent{{"started", "m(C)"}, {"failure", "assertion"}, {"finished", "m(C)"}, {"summary", "1,0,1"}}, false},
		{"assumption", []executionEvent{{"started", "m(C)"}, {"assumption", "skipped"}, {"finished", "m(C)"}, {"summary", "1,0,0"}}, false},
		{"missing", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (validateExecution(tt.events, "m(C)") == nil) != tt.ok {
				t.Fatal(tt.events)
			}
		})
	}
	if _, err := parseExecution("EXECUTION\tbogus\tYQ==\n"); err == nil {
		t.Fatal("accepted invalid event")
	}
}

func TestJUnitExecutionFixture(t *testing.T) {
	junit, hamcrest := os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	if junit == "" || hamcrest == "" {
		t.Skip("set JUnit and Hamcrest jar paths")
	}
	deps := []string{junit, hamcrest}
	for i, hash := range []string{junitSHA, hamcrestSHA} {
		if err := verifyDependency(deps[i], hash); err != nil {
			t.Fatal(err)
		}
		deps[i], _ = filepath.Abs(deps[i])
	}
	dir := t.TempDir()
	files := map[string]string{"Execution.java": executionJava, "Fixture.java": `import org.junit.*; public class Fixture { @Before public void setup() { System.setProperty("fixture.setup", "yes"); } @Test public void pass() { Assert.assertEquals("yes", System.getProperty("fixture.setup")); } @Test public void fail() { Assert.fail("expected failure"); } @Ignore @Test public void ignored() {} @Test public void assumption() { Assume.assumeTrue(false); } @Test public void invalid(int x) {} }`}
	// The invalid method makes the whole runner invalid, so compile a separate broken class.
	files["Fixture.java"] = strings.Replace(files["Fixture.java"], " @Test public void invalid(int x) {}", "", 1)
	files["Broken.java"] = `public class Broken { @org.junit.Test public void bad(int x) {} }`
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	args := append([]string{"-cp", strings.Join(deps, string(os.PathListSeparator)), "-d", dir}, paths...)
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v %s", err, out)
	}
	for _, tc := range []struct {
		class, method string
		ok            bool
	}{{"Fixture", "pass", true}, {"Fixture", "fail", false}, {"Fixture", "ignored", false}, {"Fixture", "assumption", false}, {"Broken", "bad", false}, {"Fixture", "missing", false}} {
		t.Run(tc.class+"/"+tc.method, func(t *testing.T) {
			events, err := executeJUnit(dir, deps, dir, dir, tc.class, tc.method)
			if err != nil {
				t.Fatal(err)
			}
			if (validateExecution(events, tc.method+"("+tc.class+")") == nil) != tc.ok {
				t.Fatal(events)
			}
		})
	}
}

func TestUpstreamJUnitExecution(t *testing.T) {
	if os.Getenv("TLA_GO_JUNIT_EXECUTION") != "1" {
		t.Skip("set TLA_GO_JUNIT_EXECUTION=1")
	}
	root, jar, junit, hamcrest := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR"), os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	for _, p := range []*string{&root, &jar, &junit, &hamcrest} {
		if *p == "" {
			t.Fatal("set root and all three jar paths")
		}
		var err error
		*p, err = filepath.Abs(*p)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyInputs(root, jar); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []struct{ path, hash string }{{junit, junitSHA}, {hamcrest, hamcrestSHA}} {
		if err := verifyDependency(dep.path, dep.hash); err != nil {
			t.Fatal(err)
		}
	}
	temp := t.TempDir()
	// Discovery compiles only tracked sources and establishes that this exact leaf exists.
	records, err := discover(root, jar, junit, hamcrest, temp, []string{"tlc2.tool.suite.Test10"})
	if err != nil {
		t.Fatal(err)
	}
	want := DiscoveryRecord{"case", "tlc2.tool.suite.Test10", "testSpec(tlc2.tool.suite.Test10)"}
	if len(records) != 1 || records[0] != want {
		t.Fatalf("unsupported lifecycle or discovery: %+v", records)
	}
	classes := filepath.Join(temp, "classes")
	source := filepath.Join(temp, "Execution.java")
	if err := os.WriteFile(source, []byte(executionJava), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := []string{jar, junit, hamcrest}
	args := []string{"-proc:none", "-cp", strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator)), "-d", classes, source}
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	// The upstream isolated class loader resolves standard modules relative to classes.
	extract := exec.Command("jar", "xf", jar, "tla2sany/StandardModules")
	extract.Dir = classes
	if out, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("extract standard modules: %v\n%s", err, out)
	}
	if err := copyModels(root, temp); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(temp, "run")
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(temp, "java"), filepath.Join(runDir, "test-model")); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, runDir, runDir, want.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(events, want.Display); err != nil {
		t.Fatal(err)
	}
	t.Logf("Java JUnit pass: %s#testSpec (inherited setup, assertion and teardown); events=%v", want.Class, events)
	binary := filepath.Join(temp, "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	c := Case{Name: want.Class, Model: "test10", Path: "suite", Coverage: true, Dump: true, IDs: []string{"2199", "2190"}, PresenceIDs: []string{"2186", "1000"}}
	_, _, differences, err := RunCase(root, jar, binary, c, 30*time.Second)
	if err != nil {
		t.Fatalf("partial Go diagnostic infrastructure error: %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("Go mismatch in partial diagnostic for %s: %v", want.Display, differences)
	}
	t.Logf("partial Go diagnostic matches selected observations for %s; Go JUnit assertions and lifecycle unsupported", want.Display)
}
