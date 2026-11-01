package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestJavaTLCViewUnionFingerprintError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, view string }{
		{"initial", "IF x = 0 THEN UNION {1} ELSE {}"},
		{"successor", "IF x = 0 THEN 0 ELSE UNION {1}"},
		{"source-less initial", "IF x = 0 THEN F(UNION) ELSE {}"},
		{"source-less successor", "IF x = 0 THEN 0 ELSE F(UNION)"},
		{"constant initial", "IF x = 0 THEN K ELSE {}"},
		{"constant successor", "IF x = 0 THEN 0 ELSE K"},
		{"initial effect", "IF PrintT(x) THEN IF x = 0 THEN UNION {1} ELSE {} ELSE {}"},
		{"successor effect", "IF PrintT(x) THEN IF x = 0 THEN 0 ELSE UNION {1} ELSE {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nF(op(_)) == op({1})\nK == UNION {1}\nInit == x = 0\nNext == x' = x + 1\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2103:1") || strings.Contains(out, "@!@!@STARTMSG 2147:") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
				if got, want := viewErrorEvents(out), viewErrorEvents(java); !reflect.DeepEqual(got, want) {
					t.Fatalf("Go events %q; Java events %q", got, want)
				}
			}
		})
	}
}

func TestViewUnionErrorRecorderDoesNotChangeOutput(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nV == IF PrintT(x) THEN UNION {1} ELSE {}"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
	args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	plain, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" {
		t.Fatalf("plain exit %d stderr %q output:\n%s", code, stderr, plain)
	}
	path := filepath.Join(dir, "recorder.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	debug, debugCode, debugErr := runGoInDir(t, dir, args)
	if debugCode != code || debugErr != "" || normalizeRecorderToolOutput(plain) != normalizeRecorderToolOutput(debug) {
		t.Fatalf("debug exit %d stderr %q output:\n%s\nplain:\n%s", debugCode, debugErr, debug, plain)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	if err != nil || len(coreRecorderEvents(events)) != 2 {
		t.Fatalf("recorder events %v error %v", events, err)
	}
}

func viewErrorEvents(out string) []string {
	var result []string
	for line := range strings.SplitSeq(out, "\n") {
		if line == "0" || line == "1" || line == "2" || line == "3" || line == "UNION({1})" {
			result = append(result, line)
		}
		for _, code := range []string{"1000", "2102", "2103", "2147"} {
			if strings.HasPrefix(line, "@!@!@STARTMSG "+code+":") {
				result = append(result, code)
			}
		}
	}
	return result
}

func viewPrints(out string) []string {
	var result []string
	for line := range strings.SplitSeq(out, "\n") {
		if line == "0" || line == "1" || line == "2" || line == "3" {
			result = append(result, line)
		}
	}
	return result
}

func TestJavaTLCViewUnionReplayOrder(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, next, view string }{
		{"initial candidates", "x \\in 0..2", "UNCHANGED x", "IF PrintT(x) THEN IF x = 1 THEN UNION {1} ELSE x ELSE x"},
		{"successor candidates", "x = 0", "x' \\in 1..3", "IF PrintT(x) THEN IF x = 2 THEN UNION {1} ELSE x ELSE x"},
		{"initial level", "x = 0", "UNCHANGED x", "IF PrintT(TLCGet(\"level\")) THEN UNION {1} ELSE x"},
		{"successor level", "x = 0", "x' = x + 1", "IF PrintT(TLCGet(\"level\")) THEN IF x = 1 THEN UNION {1} ELSE x ELSE x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next + "\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2103:1") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !viewReplayPrintsMatch(tc.name, viewPrints(out), viewPrints(java)) || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d prints %q messages %q\nJava exit %d prints %q messages %q", code, viewPrints(out), unionStateErrorMessages(out), javaCode, viewPrints(java), unionStateErrorMessages(java))
				}
				if !strings.HasSuffix(tc.name, " level") && !reflect.DeepEqual(viewErrorEvents(out), viewErrorEvents(java)) {
					t.Fatalf("Go events %q; Java events %q", viewErrorEvents(out), viewErrorEvents(java))
				}
			}
		})
	}
}

func viewReplayPrintsMatch(name string, got, want []string) bool {
	if !strings.HasSuffix(name, " level") {
		return reflect.DeepEqual(got, want)
	}
	// Existing initial and successor VIEW evaluation exposes different level
	// values; the replay itself must report Java's level zero.
	return len(got) == len(want) && len(got) > 0 && got[len(got)-1] == "0" && want[len(want)-1] == "0"
}

func TestJavaTLCViewUnionStateAlias(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, init, next, view string
		terse                  bool
		code                   int
	}{
		{"initial", "x = UNION {1}", "UNCHANGED x", "x", false, 255},
		{"initial terse", "x = UNION {1}", "UNCHANGED x", "x", true, 75},
		{"successor", "x = {}", "x = {} /\\ x' = UNION {1}", "x", false, 75},
		{"nested initial", "x = [a |-> UNION {1}]", "UNCHANGED x", "x.a", true, 75},
		{"nested successor", "x = [a |-> {}]", "x = [a |-> {}] /\\ x' = [a |-> UNION {1}]", "x.a", false, 75},
		{"view tuple of state", "x = UNION {1}", "UNCHANGED x", "<<x>>", true, 75},
		{"view record of successor", "x = {}", "x = {} /\\ x' = UNION {1}", "[a |-> x]", false, 75},
		{"unprintable state and subset view", "x = UNION {1}", "UNCHANGED x", "<<SUBSET (1..31), x>>", false, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next + "\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			if tc.terse {
				args = append([]string{"-terse"}, args...)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(out, "Attempted to enumerate UNION(s), but some element of s is nonenumerable.") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCViewUnionCachedConstant(t *testing.T) {
	jar := maxSetOracleJar(t)
	dir := t.TempDir()
	body := "VARIABLE x\nK == UNION {1}\nInit == PrintT(K) /\\ x = 0\nNext == UNCHANGED x\nV == IF x = 0 THEN K ELSE {}"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
	args := append([]string{"-terse", "-fp", "0"}, maxSetOracleArgs()...)
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" {
		t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
	}
	if jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if code != javaCode || !reflect.DeepEqual(viewErrorEvents(out), viewErrorEvents(java)) || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
			t.Fatalf("Go exit %d output:\n%s\nJava exit %d output:\n%s", code, out, javaCode, java)
		}
	}
}

func TestJavaTLCViewUnionLazyFunction(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, definition, view, message string }{
		{"fresh", "x = 0", "L[i \\in 0..0] == UNION {x}", "L", "2103"},
		{"alias", "x = UNION {1}", "L[i \\in 0..0] == x", "L", "2147"},
		{"cached constant", "x = 0", "K == [i \\in 0..0 |-> UNION {1}]", "IF x = 0 THEN K ELSE <<>>", "2103"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\n" + tc.definition + "\nInit == " + tc.init + "\nNext == UNCHANGED x\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-terse", "-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG "+tc.message+":1") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCUnionNestedFingerprintSources(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, next string }{
		{"record", "x = [a |-> {}]", "x = [a |-> {}] /\\ x' = [a |-> UNION {1}]"},
		{"tuple", "x = <<{}>>", "x = <<{}>> /\\ x' = <<UNION {1}>>"},
		{"nested", "x = [a |-> <<{}>>]", "x = [a |-> <<{}>>] /\\ x' = [a |-> <<UNION {1}>>]"},
		{"set", "x = {{}}", "x = {{}} /\\ x' = {UNION {1}}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2147:1") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCHigherOrderUnionFingerprintSource(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, next string }{
		{"initial", "x = F(UNION)", "UNCHANGED x"},
		{"successor", "x = {}", "x = {} /\\ x' = F(UNION)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nF(op(_)) == op({1})\nInit == " + tc.init + "\nNext == " + tc.next
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-terse", "-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2147:1") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCConstantUnionStateFailure(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, init, next string }{
		{"initial", "x = K", "UNCHANGED x"},
		{"successor", "x = {}", "x = {} /\\ x' = K"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nK == UNION {1}\nInit == " + tc.init + "\nNext == " + tc.next
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-terse", "-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2103:1") || strings.Contains(out, "@!@!@STARTMSG 2147:") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCViewUnionOtherModes(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, view string
		args       []string
		code       int
	}{
		{"dfid initial", "IF x = 0 THEN UNION {1} ELSE {}", []string{"-dfid", "3"}, 75},
		{"dfid successor", "IF x = 0 THEN 0 ELSE UNION {1}", []string{"-dfid", "3"}, 255},
		{"dfid terse", "IF x = 0 THEN UNION {1} ELSE {}", []string{"-dfid", "3", "-terse"}, 75},
		{"simulation", "IF x = 0 THEN UNION {1} ELSE {}", []string{"-simulate", "num=1", "-depth", "3"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append(append([]string{}, tc.args...), maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode {
					t.Fatalf("Go exit %d output:\n%s\nJava exit %d output:\n%s", code, out, javaCode, java)
				}
			}
		})
	}
}

func TestJavaTLCViewUnionMultipleWorkers(t *testing.T) {
	jar := maxSetOracleJar(t)
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nV == IF x = 0 THEN 0 ELSE UNION {1}"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
	args := maxSetOracleArgs()
	args[3] = "4"
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2103:1") {
		t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
	}
	if jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != code || !strings.Contains(java, "@!@!@STARTMSG 2103:1") {
			t.Fatalf("Java exit %d output:\n%s", javaCode, java)
		}
	}
}

func TestJavaTLCViewUnionReplayCanSucceed(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, next, view string
		code             int
	}{
		{"initial", "UNCHANGED x", "IF TLCGet(0) = 0 THEN IF TLCSet(0, 1) THEN UNION {1} ELSE {} ELSE {}", 0},
		{"successor", "x' = x + 1", "IF x = 0 THEN 0 ELSE IF TLCGet(0) = 0 THEN IF TLCSet(0, 1) THEN UNION {1} ELSE {} ELSE x", 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "ASSUME TLCSet(0, 0)\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(out, "Attempted to enumerate UNION(s)") || strings.Contains(out, "@!@!@STARTMSG 2103:") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func TestJavaTLCViewSubsetFingerprintError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, view string }{
		{"initial", "IF x = 0 THEN SUBSET (1..31) ELSE {}"},
		{"successor", "IF x = 0 THEN 0 ELSE SUBSET (1..31)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nV == " + tc.view
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" || !strings.Contains(out, "@!@!@STARTMSG 2103:1") || strings.Contains(out, "@!@!@STARTMSG 2147:") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if code != javaCode || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}
