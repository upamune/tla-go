package upstreamjunit

import (
	"reflect"
	"strings"
	"testing"
)

func msg(id, body string) string {
	return "@!@!@STARTMSG " + id + ":0 @!@!@\n" + body + "\n@!@!@ENDMSG " + id + " @!@!@\n"
}

func TestMessages(t *testing.T) {
	got, err := Messages("outside\n" + msg("1", "a\nb") + msg("1", "c"))
	if err != nil || !reflect.DeepEqual(got["1"], []string{"a\nb", "c"}) {
		t.Fatalf("%v %v", got, err)
	}
	for _, s := range []string{"", "@!@!@STARTMSG 1:0 @!@!@\n", "@!@!@ENDMSG 1 @!@!@\n", msg("1", "a") + "@!@!@STARTMSG 2:0 @!@!@\n@!@!@ENDMSG 3 @!@!@"} {
		if _, err := Messages(s); err == nil {
			t.Errorf("accepted malformed output %q", s)
		}
	}
}

func TestMessagesNestedError(t *testing.T) {
	output := msg("1000", "The exception was a tlc2.tool.EvalException\n: "+msg("2179", "The second argument of \\div is 0.")+"more")
	got, err := Messages(output)
	if err != nil || !reflect.DeepEqual(got["2179"], []string{"The second argument of \\div is 0."}) || !strings.Contains(got["1000"][0], "STARTMSG 2179") {
		t.Fatalf("%v %v", got, err)
	}
	for _, broken := range []string{
		strings.Replace(output, "ENDMSG 2179", "ENDMSG 1000", 1),
		strings.Replace(output, "@!@!@ENDMSG 2179 @!@!@\n", "", 1),
		strings.Replace(output, "STARTMSG 2179:0", "STARTMSG 2179:", 1),
	} {
		if _, err := Messages(broken); err == nil {
			t.Errorf("accepted %q", broken)
		}
	}
}

func TestCompare(t *testing.T) {
	a := Result{Output: msg("1", "initial") + msg("2", "counts")}
	b := Result{ExitCode: 1, Output: msg("1", "other") + msg("3", "different")}
	got, err := Compare(a, b, []string{"1", "2"})
	if err != nil || !reflect.DeepEqual(got, []string{"exit", "message:1", "message:2"}) {
		t.Fatalf("%v %v", got, err)
	}
	b.TimedOut = true
	if _, err := Compare(a, b, nil); err == nil {
		t.Fatal("timeout accepted")
	}
	ordered := Result{Output: msg("2217", "first") + msg("2217", "second")}
	reversed := Result{Output: msg("2217", "second") + msg("2217", "first")}
	if differences, err := Compare(ordered, reversed, []string{"2217"}); err != nil || !reflect.DeepEqual(differences, []string{"message:2217"}) {
		t.Fatalf("trace order: %v %v", differences, err)
	}
}

func TestMinimumDiameterDepthMismatch(t *testing.T) {
	ids := []string{"2194"}
	java := Result{Output: msg("2194", "The depth of the complete state graph search is 1.")}
	goResult := Result{Output: msg("2194", "The depth of the complete state graph search is 2.")}
	differences, err := Compare(java, goResult, ids)
	if err != nil || !reflect.DeepEqual(differences, []string{"message:2194"}) {
		t.Fatalf("depth regression not detected: %v %v", differences, err)
	}
}

func TestNondeterministicMessageTimes(t *testing.T) {
	for _, tc := range []struct{ id, a, b, changed string }{
		{"2190", "Finished computing initial states: 2 distinct states generated at 2026-11-01 10:00:00.", "Finished computing initial states: 2 distinct states generated at 2026-11-01 10:00:01.", "Finished computing initial states: 3 distinct states generated at 2026-11-01 10:00:01."},
		{"2192", "Checking temporal properties for the complete state space with 2 total distinct states at (2026-11-01 10:00:00)", "Checking temporal properties for the complete state space with 2 total distinct states at (2026-11-01 10:00:01)", "Checking temporal properties for the complete state space with 3 total distinct states at (2026-11-01 10:00:01)"},
	} {
		a := Result{Output: msg(tc.id, tc.a)}
		for _, v := range []struct {
			body      string
			different bool
		}{{tc.b, false}, {tc.changed, true}} {
			diffs, err := Compare(a, Result{Output: msg(tc.id, v.body)}, []string{tc.id})
			if err != nil || (len(diffs) != 0) != v.different {
				t.Fatalf("%s %q: %v %v", tc.id, v.body, diffs, err)
			}
		}
	}
	obs := []AssertedObservation{{ID: "2190", Present: true, Values: []string{"3"}}}
	failures, omissions, err := CheckAssertions(Result{Output: msg("2190", "Finished computing initial states: 2 distinct states generated at 2026-11-01 10:00:00.")}, obs)
	if err != nil || len(omissions) != 0 || !reflect.DeepEqual(failures, []string{"values:2190"}) {
		t.Fatalf("initial count: %v %v %v", failures, omissions, err)
	}
	obs = []AssertedObservation{{ID: "2192", Present: true, Values: []string{"complete", "8"}}}
	result := Result{Output: msg("2192", "Checking temporal properties for the complete state space with 8 total distinct states at (2026-11-01 10:00:00)")}
	failures, omissions, err = CheckAssertions(result, obs)
	if err != nil || len(omissions) != 0 || len(failures) != 0 {
		t.Fatalf("temporal values: %v %v %v", failures, omissions, err)
	}
	obs[0].Values[1] = "9"
	failures, _, _ = CheckAssertions(result, obs)
	if !reflect.DeepEqual(failures, []string{"values:2192"}) {
		t.Fatalf("temporal count: %v", failures)
	}
}

func TestComparePresence(t *testing.T) {
	a := Result{Output: msg("2186", "finished at one time")}
	b := Result{Output: msg("2186", "finished at another time") + msg("1000", "error")}
	differences, err := ComparePresence(a, b, []string{"2186", "1000"})
	if err != nil || !reflect.DeepEqual(differences, []string{"presence:1000"}) {
		t.Fatalf("presence: %v %v", differences, err)
	}
}

func TestBaseline(t *testing.T) {
	baseline := map[string][]string{"known": {"exit", "message:1"}}
	for _, tc := range []struct {
		id   string
		diff []string
		ok   bool
	}{
		{"new", nil, true},
		{"known", []string{"message:1", "exit"}, true},
		{"new", []string{"exit"}, false},
		{"known", nil, false},
		{"known", []string{"exit"}, false},
		{"known", []string{"exit", "message:2"}, false},
	} {
		if err := CheckBaseline(tc.id, tc.diff, baseline); (err == nil) != tc.ok {
			t.Errorf("%s %v: %v", tc.id, tc.diff, err)
		}
	}
}

func TestUpstreamLiteralAssertions(t *testing.T) {
	source := `assertTrue(recorder.recordedWithStringValue(EC.TLC_SEARCH_DEPTH, "1")); assertFalse(recorder.recorded(EC.GENERAL));`
	ec := `int TLC_SEARCH_DEPTH = 2194; int GENERAL = 1000;`
	observations, omitted := ParseAssertions(source, ec)
	if len(observations) != 2 || len(omitted) != 0 {
		t.Fatalf("%v %v", observations, omitted)
	}
	failures, omissions, err := CheckAssertions(Result{Output: msg("2194", "The depth of the complete state graph search is 2.")}, observations)
	if err != nil || len(omissions) != 0 || !reflect.DeepEqual(failures, []string{"values:2194"}) {
		t.Fatalf("%v %v %v", failures, omissions, err)
	}
}

func TestRecorderChecksFirstRecordAndParameterPrefix(t *testing.T) {
	obs := []AssertedObservation{{ID: "2199", Present: true, Values: []string{"5"}}}
	output := msg("2199", "5 states generated, 4 distinct states found, 0 states left on queue.") + msg("2199", "6 states generated, 4 distinct states found, 0 states left on queue.")
	failures, omissions, err := CheckAssertions(Result{Output: output}, obs)
	if err != nil || len(failures) != 0 || len(omissions) != 0 {
		t.Fatalf("prefix: %v %v %v", failures, omissions, err)
	}
	obs[0].Values[0] = "6"
	failures, _, _ = CheckAssertions(Result{Output: output}, obs)
	if !reflect.DeepEqual(failures, []string{"values:2199"}) {
		t.Fatalf("first record: %v", failures)
	}
}

func TestAssertionsIgnoreJavaCommentsAndLiterals(t *testing.T) {
	ec := `int REAL = 1; int OTHER = 2; // int FAKE = 3;
 /* int HIDDEN = 4; */`
	source := `
 // assertTrue(recorder.recorded(EC.OTHER));
 /* assertTrue(recorder.recorded(EC.HIDDEN)); */
 String ignored = "assertTrue(recorder.recorded(EC.OTHER)) // not a comment";
 char slash = '/'; char quote = '\'';
 String block = """
 assertTrue(recorder.recorded(EC.FAKE)); // literal
 """;
 assertTrue /* split */ (recorder.recordedWithStringValue(EC.REAL, "https://host/*path*/\"ok"));
 assertFalse(recorder.recorded(EC.OTHER)); // recorder.recorded(EC.FAKE)
 `
	obs, omissions := ParseAssertions(source, ec)
	want := []AssertedObservation{{ID: "1", Present: true, Values: []string{`https://host/*path*/"ok`}}, {ID: "2", Present: false}}
	if !reflect.DeepEqual(obs, want) || len(omissions) != 0 {
		t.Fatalf("observations=%#v omissions=%v", obs, omissions)
	}
	bodies, presence, omissions := AssertedIDs(source, ec)
	if !reflect.DeepEqual(bodies, []string{"1"}) || !reflect.DeepEqual(presence, []string{"2"}) || len(omissions) != 0 {
		t.Fatalf("bodies=%v presence=%v omissions=%v", bodies, presence, omissions)
	}
}
