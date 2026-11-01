package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCSymmetryTableauRegeneration(t *testing.T) {
	for _, name := range []string{"ChooseTableauSymmetryMC", "ChooseTableauSymmetryMCa"} {
		t.Run(name, func(t *testing.T) {
			dir := copySymmetryRecurrenceFixtures(t)
			checkSymmetryRecurrence(t, dir, name, 13, "13 states generated, 6 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryTableauToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	for _, name := range []string{"ChooseTableauSymmetryMC", "ChooseTableauSymmetryMCa"} {
		t.Run(name, func(t *testing.T) { compareSymmetryTableauToolTrace(t, jar, name) })
	}
}

func compareSymmetryTableauToolTrace(t *testing.T, jar, name string) {
	t.Helper()
	dir := copySymmetryRecurrenceFixtures(t)
	compareFullSymmetryToolTrace(t, jar, dir, name+".tla")
}

func compareFullSymmetryToolTrace(t *testing.T, jar, dir, model string) {
	t.Helper()
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, model)}, &out, &errout)
	if code != 13 || errout.Len() != 0 {
		t.Fatalf("tla-go exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", model)
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	java := toolConsumerMessages(readToolMessages(string(output)))
	got := toolConsumerMessages(readToolMessages(out.String()))
	if !reflect.DeepEqual(java, got) {
		t.Fatalf("consumer messages differ\nJava: %+v\nGo: %+v", java, got)
	}
	compareRawToolTrace(t, string(output), out.String())
}

func compareRawToolTrace(t *testing.T, java, got string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)@!@!@STARTMSG (?:2217|2218|2122):4 @!@!@\n.*?@!@!@ENDMSG (?:2217|2218|2122) @!@!@`)
	want := pattern.FindAllString(java, -1)
	actual := pattern.FindAllString(got, -1)
	if !reflect.DeepEqual(want, actual) {
		t.Fatalf("raw trace messages differ\nJava: %q\nGo: %q", want, actual)
	}
}

func TestJavaTLCSymmetryLeadsToFairness(t *testing.T) {
	for _, tc := range []struct {
		name, variables, fairness, property string
		constraints                         bool
		code                                int
	}{
		{"no fairness", "x,bit", "", "(x=a) ~> (bit=1)", false, 13},
		{"weak fairness", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"strong fairness", "x,bit", " /\\ SF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"partial weak fairness", "x,bit", " /\\ WF_x(Next) /\\ WF_bit(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"partial mixed fairness", "x,bit", " /\\ SF_x(Next) /\\ WF_bit(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"other variable order", "bit,x", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"state and action constraints", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", true, 0},
		{"other trigger without fairness", "x,bit", "", "(bit=1) ~> (x=a)", false, 13},
		{"other trigger with fairness", "x,bit", " /\\ WF_vars(Next)", "(bit=1) ~> (x=a)", false, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES " + tc.variables + "\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ bit' \\in {0,1}\n" +
				"Spec == Init /\\ [][Next]_vars" + tc.fairness + "\nSym == Permutations(P)\nProp == " + tc.property
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			if tc.constraints {
				body += "\nCon == bit=0 \\/ x=a\nAct == bit'=bit \\/ x'=x"
				cfg += " CONSTRAINT Con ACTION_CONSTRAINT Act"
			}
			writeDOTFixture(t, dir, body, cfg)
			checkSymmetryRecurrence(t, dir, "M", tc.code, "10 states generated, 2 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryLeadsToToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	for _, tc := range []struct{ name, fairness, property string }{
		{"no fairness", "", "(x=a) ~> (x=b)"},
		{"weak fairness", " /\\ WF_vars(Next)", "(x=a) ~> (x=b)"},
		{"strong fairness", " /\\ SF_vars(Next)", "(x=a) ~> (x=b)"},
		{"partial weak fairness", " /\\ WF_x(Next) /\\ WF_bit(Next)", "<>(x=b /\\ []<>(x=a))"},
		{"partial mixed fairness", " /\\ SF_x(Next) /\\ WF_bit(Next)", "<>(x=b /\\ []<>(x=a))"},
		{"partial strong fairness", " /\\ WF_x(Next) /\\ SF_bit(Next)", "<>(x=b /\\ []<>(x=a))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES x,bit\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ bit' \\in {0,1}\n" +
				"Spec == Init /\\ [][Next]_vars" + tc.fairness + "\nSym == Permutations(P)\nProp == " + tc.property
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			writeDOTFixture(t, dir, body, cfg)
			compareFullSymmetryToolTrace(t, jar, dir, "M.tla")
		})
	}
}

func TestJavaTLCPartialFairness(t *testing.T) {
	for _, tc := range []struct {
		name, fairness string
		code           int
	}{
		{"weak x", "WF_x(Next)", 0},
		{"strong x", "SF_x(Next)", 0},
		{"weak y", "WF_y(Next)", 13},
		{"strong y", "SF_y(Next)", 13},
		{"both", "WF_x(Next) /\\ WF_y(Next)", 0},
		{"expression", "WF_sum(Next)", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLES x,y\nvars == <<x,y>>\nsum == y+0\n" +
				"Init == x=0 /\\ y=0\nNext == x' \\in {x,1} /\\ y' \\in {y,1}\n" +
				"Spec == Init /\\ [][Next]_vars /\\ " + tc.fairness + "\nP == <>(x=1)"
			writeDOTFixture(t, dir, body, "SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE")
			checkSymmetryRecurrence(t, dir, "M", tc.code, "10 states generated, 4 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryFairRecurrenceToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	for _, tc := range []struct{ name, fairness string }{
		{"two weak", "WF_x(Next) /\\ WF_bit(Next)"},
		{"strong x", "SF_x(Next) /\\ WF_bit(Next)"},
		{"strong bit", "WF_x(Next) /\\ SF_bit(Next)"},
		{"weak tuple", "WF_vars(Next)"},
		{"strong tuple", "SF_vars(Next)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES x,bit\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ UNCHANGED bit\n" +
				"Spec == Init /\\ [][Next]_vars /\\ " + tc.fairness + "\nSym == Permutations(P)\nProp == <>[](x=a) => []<>(bit=1)"
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			writeDOTFixture(t, dir, body, cfg)
			compareFullSymmetryToolTrace(t, jar, dir, "M.tla")
		})
	}
}

func TestJavaTLCSymmetryUnfairGeneralToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	for _, tc := range []struct{ name, next, property string }{
		{"guard nondeterministic", "x' \\in P /\\ bit' \\in {0,1}", "[](bit=0 => <>(x=b))"},
		{"nested alternating", "x' = x /\\ bit' = 1-bit", "<>(x=b /\\ []<>(x=a))"},
		{"nested symmetric alternating", "x' \\in P /\\ bit' = 1-bit", "<>(x=b /\\ []<>(x=a))"},
		{"guard alternating", "x' = x /\\ bit' = 1-bit", "[](bit=0 => <>(x=b))"},
		{"guard symmetric alternating", "x' \\in P /\\ bit' = 1-bit", "[](bit=0 => <>(x=b))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES x,bit\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == " + tc.next + "\nSpec == Init /\\ [][Next]_vars\n" +
				"Sym == Permutations(P)\nProp == " + tc.property
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			writeDOTFixture(t, dir, body, cfg)
			compareFullSymmetryToolTrace(t, jar, dir, "M.tla")
		})
	}
}
