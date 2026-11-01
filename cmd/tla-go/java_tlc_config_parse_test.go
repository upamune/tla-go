package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCConfigParsingFailures(t *testing.T) {
	for _, tc := range []struct {
		name, config, message string
		code, nested          int
		missing               bool
	}{
		{"missing config", "", "TLC encountered the following error when trying to read the configuration file M.cfg:\nFile not found.", 255, 5001, true},
		{"unexpected token in set", "CONSTANT C = {%1%}\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 1\nIt was expecting }, but did not find it.", 255, 5006, false},
		{"missing set terminator", "CONSTANT C = {1, 2\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 2\nIt was expecting }, but did not find it.", 255, 5006, false},
		{"trailing set comma", "CONSTANT C = {1,}\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 2\nIt was expecting }, but did not find it.", 255, 5006, false},
		{"duplicate init", "INIT Init\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 2\nThe keyword INIT appeared twice.", 255, 5004, false},
		{"duplicate init without value", "INIT Init\nINIT", "TLC found an error in the configuration file at line 2\nThe keyword INIT was not followed by an identifier.", 255, 5003, false},
		{"duplicate deadlock with invalid value", "CHECK_DEADLOCK TRUE\nCHECK_DEADLOCK MAYBE", "TLC found an error in the configuration file at line 2\nIt was expecting TRUE or FALSE, but did not find it.", 255, 5006, false},
		{"duplicate deadlock without value", "CHECK_DEADLOCK TRUE\nCHECK_DEADLOCK", "TLC found an error in the configuration file at line 2\nThe keyword CHECK_DEADLOCK was not followed by an identifier.", 255, 5003, false},
		{"unknown directive", "INVALID Foo\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 1\nIt was expecting a keyword, but did not find it.", 255, 5006, false},
		{"missing identifier", "INIT", "TLC found an error in the configuration file at line 1\nThe keyword INIT was not followed by an identifier.", 255, 5003, false},
		{"invalid deadlock flag", "CHECK_DEADLOCK FOO\nINIT Init\nNEXT Next\n", "TLC found an error in the configuration file at line 1\nIt was expecting TRUE or FALSE, but did not find it.", 255, 5006, false},
		{"missing deadlock flag", "CHECK_DEADLOCK\n", "TLC found an error in the configuration file at line 1\nThe keyword CHECK_DEADLOCK was not followed by an identifier.", 255, 5003, false},
		{"missing replacement", "CONSTANT X <-", "TLC found an error in the configuration file at line 1\nExpected an identifier after <-.", 255, 5005, false},
		{"missing replacement after newline", "CONSTANT X <-\n", "TLC found an error in the configuration file at line 1\nExpected an identifier after <-.", 255, 5005, false},
		{"missing scoped module", "CONSTANT X <- [", "TLC found an error in the configuration file at line 1\nExpected an identifier after <-[.", 255, 5005, false},
		{"missing scoped replacement", "CONSTANT X <- [Foo]", "TLC found an error in the configuration file at line 1\nExpected an identifier after <-[mod].", 255, 5005, false},
		{"missing scope terminator", "CONSTANT X <- [Foo", "TLC found an error in the configuration file at line 1\nIt was expecting ], but did not find it.", 255, 5006, false},
		{"missing value", "CONSTANT X =", "TLC found an error in the configuration file at line 1\nIt was expecting a value, but did not find it.", 255, 5006, false},
		{"missing value after newline", "CONSTANT X =\n", "TLC found an error in the configuration file at line 1\nIt was expecting a value, but did not find it.", 255, 5006, false},
		{"missing set terminator at eof", "CONSTANT X = {1\n", "TLC found an error in the configuration file at line 1\nIt was expecting }, but did not find it.", 255, 5006, false},
		{"empty arguments", "CONSTANT F() = 1\nINIT Init\nNEXT Next", "TLC found an error in the configuration file at line 1", 255, 5002, false},
		{"missing argument comma", "CONSTANT F(1 2) = 3\nINIT Init\nNEXT Next", "TLC found an error in the configuration file at line 1", 255, 5002, false},
		{"digit-prefixed invariant", "INVARIANT 12Inv\nINIT Init\nNEXT Next\n", "The invariant 12Inv specified in the configuration file\nis not defined in the specification.", 151, 0, false},
		{"numeric initial name", "INIT 12\nNEXT Next\n", "The initial predicate 12 specified in the configuration file\nis not defined in the specification.", 151, 0, false},
		{"mixed spec and init", "INIT Init\nNEXT Next\nSPECIFICATION Spec\n", "The configuration file cannot specify both INIT/NEXT and SPECIFICATION fields.", 151, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			if !tc.missing {
				if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if tc.nested != 0 && !hasNestedToolCode(out.String(), tc.nested) {
				t.Fatalf("missing nested code %d: %s", tc.nested, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func hasNestedToolCode(output string, code int) bool {
	return strings.Contains(output, "@!@!@STARTMSG "+strconv.Itoa(code)+":0 @!@!@")
}

func TestJavaTLCConfigBeforeModuleParsing(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		code         int
	}{
		{"invalid config", "INVALID", 255},
		{"valid config", "INIT Init\nNEXT Next\n", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte("not a module"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.config), 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

func TestJavaTLCSemanticErrorBeforeConfigModelError(t *testing.T) {
	for _, tc := range []struct{ name, configuration string }{
		{"mixed configuration", "INIT Init\nNEXT Next\nSPECIFICATION Spec\n"},
		{"missing initial predicate", "NEXT Next\n"},
		{"missing specification", "SPECIFICATION MissingSpec\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = Missing\nNext == x' = x\nSpec == Init /\\ [][Next]_x"
			writeDOTFixture(t, dir, body, tc.configuration)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 150 || !strings.Contains(out.String(), "Missing") {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", 150)
			}
		})
	}
}

func TestJavaTLCConfigLexicalFailures(t *testing.T) {
	for _, tc := range []struct {
		name, configuration, message string
		code                         int
	}{
		{"unfinished comment after directives", "INIT Init\nNEXT Next\n(* unfinished", "2 states generated, 1 distinct states found", 0},
		{"unfinished comment before directives", "(* unfinished", "The configuration file did not specify the initial state predicate", 151},
		{"unfinished string after directives", "INIT Init\nNEXT Next\n\"unfinished", "2 states generated, 1 distinct states found", 0},
		{"invalid character after directives", "INIT Init\nNEXT Next\n☃", "2 states generated, 1 distinct states found", 0},
		{"unfinished string as value", "CONSTANT X = \"unfinished", `Cannot invoke "String.equals(Object)" because "tt.image" is null`, 255},
		{"invalid character as value", "CONSTANT X = ☃", `Cannot invoke "String.equals(Object)" because "tt.image" is null`, 255},
		{"nested comment", "(* outer\n(* inner *)\nouter *)\nINIT Init\nNEXT Next\n", "2 states generated, 1 distinct states found", 0},
		{"unfinished nested comment", "INIT Init\nNEXT Next\n(* outer (* inner *)", "2 states generated, 1 distinct states found", 0},
		{"symbol model value", "CONSTANT X = @\nINIT Init\nNEXT Next\n", "2 states generated, 1 distinct states found", 0},
		{"digit-prefixed model value", "CONSTANT X = 12Inv\nINIT Init\nNEXT Next\n", "2 states generated, 1 distinct states found", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x", tc.configuration)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCMissingStatePredicates(t *testing.T) {
	for _, tc := range []struct {
		name, init, cfg, message string
		code                     int
	}{
		{"missing init and next", "x = 0", "", "The configuration file did not specify the initial state predicate.", 151},
		{"missing init", "x = 0", "NEXT Next\n", "The configuration file did not specify the initial state predicate.", 151},
		{"missing next with states", "x = 0", "INIT Init\n", "No next state actions defined to generate successor states from.", 75},
		{"missing next with no states", "FALSE /\\ x = 0", "INIT Init\n", "0 states generated, 0 distinct states found", 0},
		{"missing next with invariant", "x = 0", "INIT Init\nINVARIANT Inv\n", "The configuration file did not specify the next state predicate.", 151},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == "+tc.init+"\nNext == x' = x\nInv == x = 1", tc.cfg)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if tc.code == 75 && hasToolMessage(readToolMessages(out.String()), 2199) {
				t.Fatalf("unexpected statistics for missing NEXT: %s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

// TLC's Tool.getActions yields no action for an existential with an empty
// constant bound.
func TestJavaTLCNextWithoutActions(t *testing.T) {
	for _, tc := range []struct {
		name, next, message string
		code                int
	}{
		{"empty existential", "\\E i \\in {} : x' = i", "No next state actions defined to generate successor states from.", 75},
		{"empty disjuncts", "(\\E i \\in {} : x' = i) \\/ (\\E j \\in 1..0 : x' = j)", "No next state actions defined to generate successor states from.", 75},
		{"empty second bound", "\\E i \\in 1..2, j \\in {} : x' = i + j", "No next state actions defined to generate successor states from.", 75},
		{"other action", "(\\E i \\in {} : x' = i) \\/ x' = x", "2 states generated, 1 distinct states found, 0 states left on queue", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == "+tc.next, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCStateOnlySpecification(t *testing.T) {
	for _, tc := range []struct {
		name, spec, extra, message string
		code                       int
	}{
		{"initial states", "x = 0", "", "No next state actions defined to generate successor states from.", 75},
		{"no initial states", "x \\in {}", "", "0 states generated, 0 distinct states found", 0},
		{"multiple initial states", "x \\in {0,1}", "", "No next state actions defined to generate successor states from.", 75},
		{"invariant requires next", "x = 0", "INVARIANT Inv\n", "The configuration file did not specify the next state predicate.", 151},
		{"temporal property requires next", "x = 0", "PROPERTY Future\n", "The configuration file did not specify the next state predicate.", 151},
		{"state property", "x = 0", "PROPERTY Inv\n", "Property Inv is violated by the initial state", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nSpec == "+tc.spec+"\nInv == x = 1\nFuture == <>(x = 1)", "SPECIFICATION Spec\n"+tc.extra)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCConfigDigitPrefixedName(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\n12Inv == TRUE\nInit == x = 0\nNext == x' = x"
	writeDOTFixture(t, dir, body, "INVARIANT 12Inv\nINIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "M", 0)
	}
}

func TestConfigParseErrorHumanReadable(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x", "INVALID\n")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != 255 || !strings.Contains(out.String(), "It was expecting a keyword") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
}

func TestJavaTLCConfigOperatorErrors(t *testing.T) {
	for _, tc := range []struct{ name, config, message string }{
		{"missing init", "INIT Missing\nNEXT Next\n", "The initial predicate Missing specified in the configuration file\nis not defined in the specification."},
		{"missing next", "INIT Init\nNEXT Missing\n", "The next state action Missing specified in the configuration file\nis not defined in the specification."},
		{"init with arguments", "INIT F\nNEXT Next\n", "TLC requires initial predicate not to take any argument, but one was given: F"},
		{"next with arguments", "INIT Init\nNEXT F\n", "TLC requires next state action not to take any argument, but one was given: F"},
		{"missing spec", "SPECIFICATION Missing\n", "The name Missing specified in the configuration file\nis not defined in the specification."},
		{"spec with arguments", "SPECIFICATION F\n", "TLC requires F not to take any argument."},
		{"missing property", "INIT Init\nNEXT Next\nPROPERTY Missing\n", "The property Missing specified in the configuration file\nis not defined in the specification."},
		{"property with arguments", "INIT Init\nNEXT Next\nPROPERTY F\n", "TLC requires F not to take any argument."},
		{"missing constraint", "INIT Init\nNEXT Next\nCONSTRAINT Missing\n", "The constraint Missing specified in the configuration file\nis not defined in the specification."},
		{"constraint with arguments", "INIT Init\nNEXT Next\nCONSTRAINT F\n", "TLC requires constraint not to take any argument, but one was given: F"},
		{"missing action constraint", "INIT Init\nNEXT Next\nACTION_CONSTRAINT Missing\n", "The action constraint Missing specified in the configuration file\nis not defined in the specification."},
		{"action constraint with arguments", "INIT Init\nNEXT Next\nACTION_CONSTRAINT F\n", "TLC requires action constraint not to take any argument, but one was given: F"},
		{"missing view", "INIT Init\nNEXT Next\nVIEW Missing\n", "The view function Missing specified in the configuration file\nis not defined in the specification."},
		{"view with arguments", "INIT Init\nNEXT Next\nVIEW F\n", "TLC requires view function not to take any argument, but one was given: F"},
		{"invariant with arguments", "INIT Init\nNEXT Next\nINVARIANT F\n", "TLC requires invariant not to take any argument, but one was given: F"},
		{"invariant before init", "INIT MissingInit\nNEXT Next\nINVARIANT MissingInv\n", "The invariant MissingInv specified in the configuration file\nis not defined in the specification."},
		{"init before property", "INIT MissingInit\nNEXT Next\nPROPERTY MissingProp\n", "The initial predicate MissingInit specified in the configuration file\nis not defined in the specification."},
		{"property before constraint", "INIT Init\nNEXT Next\nPROPERTY MissingProp\nCONSTRAINT MissingConstraint\n", "The property MissingProp specified in the configuration file\nis not defined in the specification."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nF(a) == a\nInit == x = 0\nNext == x' = x", tc.config)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 151 || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 151)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCConfigErrorPrecedence(t *testing.T) {
	dir := t.TempDir()
	body := "CONSTANT C\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nSPECIFICATION Spec\n")
	var out, errout bytes.Buffer
	message := "The constant parameter C is not assigned a value by the configuration file."
	if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 151 || !strings.Contains(out.String(), message) || !hasToolMessage(readToolMessages(out.String()), 2222) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 151)
		if !strings.Contains(java, message) {
			t.Fatalf("Java output: %s", java)
		}
	}
}

func TestJavaTLCUpstreamMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Github432.tla", "Github432.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	message := "TLC found an error in the configuration file at line 1\nIt was expecting }, but did not find it."
	if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "Github432.tla")}, &out, &errout); code != 255 || !strings.Contains(out.String(), message) || !hasNestedToolCode(out.String(), 5006) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "Github432", 255)
		if !strings.Contains(java, message) {
			t.Fatalf("Java output: %s", java)
		}
	}
}

// Java TLC 1.8.0 reports configuration syntax errors differently from 1.7.4.
func TestJavaTLCModernConfigSyntax(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("set TLC_MODERN_JAR to compare modern TLC configuration errors")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x", "CONSTANT C = {%1%}\nINIT Init\nNEXT Next\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 151 || !hasToolMessage(readToolMessages(string(output)), 5006) {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
}
