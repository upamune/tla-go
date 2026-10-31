package main

import (
	"os"
	"strings"
	"testing"
)

// In depth-first mode, Java TLC names a -maxSetSize failure as a
// util.WrongInvocationException, and after replaying doNext it prints its
// call stack in message 2103 as in breadth-first mode: empty after such a
// failure, and otherwise ending with a blank line. After replaying doInit it
// prints the name of its Tool object, which tla-go leaves out.
func TestJavaTLCDepthFirstErrorMessages(t *testing.T) {
	const bounded = "Attempted to construct a set with too many elements (>10).\n@!@!@ENDMSG 1000 @!@!@\n"
	const emptyStack = "    The error call stack is empty.\n\n@!@!@ENDMSG 2103 @!@!@\n"
	for _, tc := range []struct {
		name, body, cfg   string
		args              []string
		code              int
		exception, nested string
	}{
		{"max set size next", "Init == x = 0\nNext == x' \\in {n \\in 0..20 : n # x}", "", []string{"-maxSetSize", "10", "-dfid", "3"}, 255, "computing the set of next states.\nThe exception was a util.WrongInvocationException\n: " + bounded, emptyStack},
		{"max set size constraint", "Init == x = 0\nNext == x < 3 /\\ x' = x + 1\nC == x = 0 \\/ Cardinality({n \\in 0..20 : n # x}) > 0", "CONSTRAINT C\n", []string{"-deadlock", "-maxSetSize", "10", "-dfid", "5"}, 255, "computing the set of next states.\nThe exception was a util.WrongInvocationException\n: " + bounded, emptyStack},
		{"max set size init", "Init == x \\in {n \\in 0..20 : n # 3}\nNext == x' = x", "", []string{"-maxSetSize", "10", "-dfid", "3"}, 75, "computing initial states.\nThe exception was a util.WrongInvocationException\n: " + bounded, ""},
		{"next-state stack", "Init == x = 0\nNext == x' = IF x = 2 THEN 1 \\div 0 ELSE x + 1", "", []string{"-dfid", "10"}, 255, "computing the set of next states.\nThe exception was a tlc2.tool.EvalException\n: @!@!@STARTMSG 2179:0 @!@!@\nThe second argument of \\div is 0.\n@!@!@ENDMSG 2179 @!@!@\n@!@!@ENDMSG 1000 @!@!@\n", "2. Line 5, column 28 to line 5, column 35 in M\n\n\n@!@!@ENDMSG 2103 @!@!@\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.args...), "M.tla")
			output, code, stderr := runGoInDir(t, dir, args)
			exception, nested := rawToolMessage(output, 1000), rawToolMessage(output, 2103)
			if code != tc.code || stderr != "" || !strings.HasSuffix(exception, tc.exception) || tc.nested != "" && !strings.HasSuffix(nested, tc.nested) {
				t.Fatalf("exit %d, want %d; 1000 %q; 2103 %q; stderr %s", code, tc.code, exception, nested, stderr)
			}
			jar := os.Getenv("TLC_JAR")
			if jar == "" {
				return
			}
			java, javaCode := runJavaTLCArgs(t, jar, dir, args)
			if javaCode != code || rawToolMessage(java, 1000) != exception || tc.nested != "" && rawToolMessage(java, 2103) != nested {
				t.Fatalf("Java exit %d, Go exit %d\nJava:\n%s\nGo:\n%s", javaCode, code, java, output)
			}
		})
	}
}
