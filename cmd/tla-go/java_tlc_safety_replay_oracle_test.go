package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Java-only oracle: these cases document an unresolved safety-trace difference,
// not a claim of parity. Set TLA_GO_TEST_SAFETY_REPLAY=1 as well as TLC_JAR to
// compare tla-go and expose the current incompatibility.
func TestJavaTLCSafetyReplayOracle(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned Java TLC 1.7.4 jar")
	}
	const init = `VARIABLE x
ASSUME TLCSet(0, 0)
Init == /\ PrintT("I") /\ x = 0
Next == /\ TLCSet(0, TLCGet(0) + 1)
        /\ PrintT(<<"N", x, TLCGet(0)>>)
        /\ x < 3
        /\ x' = IF `
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	for _, tc := range []struct {
		name, body string
		config     string
		args       []string
		exit       int
		events     []string
	}{
		{
			name: "continuation mutates subsequent exploration",
			body: init + `x = 2 /\ TLCGet(0) > 4 THEN 4 ELSE x + 1
Inv == x < 2`,
			args: []string{"-continue"}, exit: 0,
			events: []string{`"I"`, `<<"N", 0, 1>>`, `<<"N", 1, 2>>`, "2110", `"I"`, "2121", "2217", "x = 0", `<<"N", 0, 3>>`, "2217", "x = 1", `<<"N", 1, 4>>`, "2217", "x = 2", `<<"N", 2, 5>>`, "2110", `"I"`, `<<"N", 0, 6>>`, "2121", "2217", "x = 0", "2217", "x = 1", `<<"N", 1, 7>>`, "2217", "x = 2", `<<"N", 2, 8>>`, "2217", "x = 4", `<<"N", 4, 9>>`},
		},
		{
			name: "failed successor recovery",
			body: init + `x = 1 /\ TLCGet(0) > 2 THEN 4 ELSE x + 1
Inv == x < 2`,
			args: []string{"-continue"}, exit: 1,
			events: []string{`"I"`, `<<"N", 0, 1>>`, `<<"N", 1, 2>>`, "2110", `"I"`, "2121", "2217", "x = 0", `<<"N", 0, 3>>`, "2217", "x = 1", `<<"N", 1, 4>>`, "2123", "2128", "2216", "x = 2"},
		},
		{
			name: "deadlock replays initial and next",
			body: `VARIABLE x
Init == /\ PrintT("I") /\ x = 0
Next == /\ PrintT(<<"N", x>>) /\ x = 0 /\ x' = 1`,
			config: "INIT Init\nNEXT Next\n", exit: 11,
			events: []string{`"I"`, `<<"N", 0>>`, `<<"N", 1>>`, "2114", `"I"`, "2121", "2217", "x = 0", `<<"N", 0>>`, "2217", "x = 1"},
		},
		{
			name: "recovery evaluates preceding actions",
			body: `VARIABLE x
Init == /\ PrintT("I") /\ x = 1
Next == \/ /\ PrintT(<<"A", x>>) /\ x = 0 /\ x' = 2
        \/ /\ PrintT(<<"B", x>>) /\ x = 1 /\ x' = 2
Inv == x < 2`,
			exit:   12,
			events: []string{`"I"`, `<<"A", 1>>`, `<<"B", 1>>`, "2110", "2121", "2217", "x = 1", `<<"A", 1>>`, `<<"B", 1>>`, "2217", "x = 2"},
		},
		{
			name: "multiple initial states and actions",
			body: `VARIABLE x
Init == /\ PrintT("I") /\ x \in 0..1
Next == \/ /\ PrintT(<<"A", x>>) /\ x = 0 /\ x' = 2
        \/ /\ PrintT(<<"B", x>>) /\ x = 1 /\ x' = 2
Inv == x < 2`,
			exit:   12,
			events: []string{`"I"`, `<<"A", 0>>`, "2110", "2121", "2217", "x = 0", `<<"A", 0>>`, "2217", "x = 2"},
		},
	} {
		for _, tool := range []bool{false, true} {
			name := tc.name + "/normal"
			if tool {
				name = tc.name + "/tool"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				modelCfg := tc.config
				if modelCfg == "" {
					modelCfg = cfg
				}
				writeDOTFixture(t, dir, tc.body, modelCfg)
				args := append([]string{"-workers", "1", "-fp", "0", "-checkpoint", "0"}, tc.args...)
				if tool {
					args = append(args, "-tool")
				}
				args = append(args, "M.tla")
				out, code := runJavaTLCArgs(t, jar, dir, args)
				if got := safetyReplayEvents(out); code != tc.exit || !reflect.DeepEqual(got, tc.events) {
					t.Fatalf("Java exit %d, events %q, output:\n%s", code, got, out)
				}
				if os.Getenv("TLA_GO_TEST_SAFETY_REPLAY") != "1" {
					return
				}
				goOut, goCode, stderr := runGoInDir(t, dir, args)
				if got := safetyReplayEvents(goOut); goCode != code || stderr != "" || !reflect.DeepEqual(got, tc.events) {
					t.Errorf("Go exit %d, stderr %q, events %q; Java exit %d, events %q", goCode, stderr, got, code, tc.events)
				}
			})
		}
	}
}

func safetyReplayEvents(output string) []string {
	var events []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case line == `"I"`, strings.HasPrefix(line, `<<"N",`), strings.HasPrefix(line, `<<"A",`), strings.HasPrefix(line, `<<"B",`), strings.HasPrefix(line, "x = "):
			events = append(events, line)
		case strings.HasPrefix(line, "@!@!@STARTMSG 2217:"), strings.HasPrefix(line, "State 1:"), strings.HasPrefix(line, "State 2:"), strings.HasPrefix(line, "State 3:"), strings.HasPrefix(line, "State 4:"):
			events = append(events, "2217")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2216:"), line == "State :":
			events = append(events, "2216")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2114:"), line == "Error: Deadlock reached.":
			events = append(events, "2114")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2110:"), strings.HasPrefix(line, "Error: Invariant Inv is violated."):
			events = append(events, "2110")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2121:"), strings.HasPrefix(line, "Error: The behavior up to this point is:"):
			events = append(events, "2121")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2123:"), strings.HasPrefix(line, "Error: Failed to recover the initial state"):
			events = append(events, "2123")
		case strings.HasPrefix(line, "@!@!@STARTMSG 2128:"), strings.HasPrefix(line, "Error: This is probably a TLC bug"):
			events = append(events, "2128")
		}
	}
	return events
}
