package checker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// The expected counts and traces below were compared with Java TLC 1.7.4.
// Where TLC's random order allows several results, the expectation is the one
// TLC reports when it picks states in generation order.

const dfidCounter = `---- MODULE Counter ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == x < 5 /\ x' = x + 1
TwoInit == x \in {0, 10}
Mono == [][x' > x]_x
Wrap == \/ x < 3 /\ x' = x + 1
        \/ x = 3 /\ x' = 0
Live == <>(x = 3)
Stats == PrintT(<<TLCGet("level"), TLCGet("diameter"), TLCGet("distinct"), TLCGet("queue")>>)
Exit == IF x = 2 THEN TLCSet("exit", TRUE) ELSE TRUE
ActErr == [][IF x' = 2 THEN 1 \div 0 = 0 ELSE TRUE]_x
Choose == x' = IF x < 2 THEN x + 1 ELSE CHOOSE y \in {} : TRUE
Div == x' = IF x < 2 THEN x + 1 ELSE 1 \div 0
ExitNext == x < 5 /\ x' = x + 1 /\ (x = 2 => TLCSet("exit", TRUE))
Post == TLCGet("generated") > 0
====`

const dfidGrid = `---- MODULE Grid ----
EXTENDS Naturals, TLC
VARIABLES x, y
Init == x \in 0..2 /\ y = 0
Next == \/ x' = x + 1 /\ y' = y
        \/ y' = y + 1 /\ x' = x
Constraint == x + y < 4
ActCon == ~(x' = 2 /\ y' = 1)
Inv == ~(x = 1 /\ y = 2)
InitInv == x < 2
Partial == x = 0
====`

const dfidBatch = `---- MODULE Batch ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Set == x' \in {1, 2, 3}
Split == x' = 1 \/ x' = 2 \/ x' = 3
Ex == \E i \in {1, 2, 3} : x' = i
Chain == \/ x < 3 /\ x' = x + 1
         \/ x = 1 /\ x' = 10
Inv == x # 2
InvTen == x # 10
DivInit == IF x = 0 THEN 1 \div 0 = 0 ELSE TRUE
====`

type dfidRun struct {
	result  Result
	levels  []string
	errors  []Result
	printed []string
}

func runDepthFirst(t *testing.T, spec, cfg string, depth int, continueChecking bool) (dfidRun, error) {
	t.Helper()
	var run dfidRun
	options := Options{
		DepthFirst: &DepthFirstOptions{MaxDepth: depth, OnError: func(r Result) { run.errors = append(run.errors, r) }},
		OnSearch: func(event SearchEvent) {
			if event.Kind == DepthFirstLevel {
				run.levels = append(run.levels, fmt.Sprintf("%d:%d/%d", event.Level, event.GeneratedStates, event.DistinctStates))
			}
		},
		OnPrint:  func(text string) { run.printed = append(run.printed, text) },
		Continue: continueChecking,
	}
	if continueChecking {
		options.OnViolation = func(r Result) { run.errors = append(run.errors, r) }
	}
	var err error
	run.result, err = CheckWithOptions(context.Background(), spec, cfg, options)
	return run, err
}

func dfidTrace(trace []TraceStep) string {
	steps := make([]string, len(trace))
	for i, step := range trace {
		var fields []string
		for _, name := range slices.Sorted(func(yield func(string) bool) {
			for name := range step.State {
				if !yield(name) {
					return
				}
			}
		}) {
			fields = append(fields, name+"="+step.State[name].String())
		}
		steps[i] = strings.Join(fields, ",")
	}
	return strings.Join(steps, " ")
}

func dfidOutcome(r Result) string {
	text := fmt.Sprintf("%s %d/%d", r.Status, r.GeneratedStates, r.DistinctStates)
	if r.Violation != nil {
		text += " " + r.Violation.Name + " " + dfidTrace(r.Violation.Trace)
	}
	if r.Failure != nil {
		text += fmt.Sprintf(" %d/%d %s", r.Failure.ToolCode, r.Failure.ExitCode, dfidTrace(r.Failure.Trace))
	}
	return text
}

func TestDepthFirstSearch(t *testing.T) {
	cases := []struct {
		name, spec, cfg string
		depth           int
		continueChecks  bool
		want, levels    string
	}{
		{
			"deadlock found at the next level", dfidCounter, "INIT Init\nNEXT Next\n", 10, false,
			"deadlock 21/6 deadlock x=0 x=1 x=2 x=3 x=4 x=5", "2:1/1 3:2/2 4:4/3 5:7/4 6:11/5 7:16/6",
		},
		{
			"depth limit before the deadlock", dfidCounter, "INIT Init\nNEXT Next\n", 6, false,
			"depth-limit 16/6", "2:1/1 3:2/2 4:4/3 5:7/4 6:11/5",
		},
		{
			"depth limit when the last level finds no state", dfidCounter, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n", 7, false,
			"depth-limit 21/6", "2:1/1 3:2/2 4:4/3 5:7/4 6:11/5 7:16/6",
		},
		{
			"success after the search ends", dfidCounter, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n", 8, false,
			"success 21/6", "2:1/1 3:2/2 4:4/3 5:7/4 6:11/5 7:16/6",
		},
		{
			"depth below two", dfidCounter, "INIT Init\nNEXT Next\n", 1, false,
			"error 1/1 0/255 ", "",
		},
		{
			"later initial state repeats on the stack", dfidCounter, "INIT TwoInit\nNEXT Next\n", 10, false,
			"deadlock 3/3 deadlock x=10 x=10", "2:2/2",
		},
		{
			"action property", dfidCounter, "INIT Init\nNEXT Wrap\nPROPERTY Mono\n", 10, false,
			"liveness-violation 11/4 Mono x=0 x=1 x=2 x=3 x=0", "2:1/1 3:2/2 4:4/3 5:7/4",
		},
		{
			"constraints", dfidGrid, "INIT Init\nNEXT Next\nCONSTRAINT Constraint\nACTION_CONSTRAINT ActCon\nCHECK_DEADLOCK FALSE\n", 20, false,
			"success 47/9", "2:3/3 3:9/6 4:21/8 5:35/9",
		},
		{
			"initial invariant", dfidGrid, "INIT Init\nNEXT Next\nINVARIANT InitInv\n", 20, false,
			"invariant-violation 3/3 InitInv x=2,y=0", "",
		},
		{
			"continue after a violation", dfidGrid, "INIT Init\nNEXT Next\nCONSTRAINT Constraint\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n", 20, true,
			"success 49/10", "2:3/3 3:9/7 4:23/9 5:37/10",
		},
		{
			"one action with several successors", dfidBatch, "INIT Init\nNEXT Set\nINVARIANT Inv\n", 5, false,
			"invariant-violation 4/3 Inv x=0 x=2", "2:1/1",
		},
		{
			"split disjuncts", dfidBatch, "INIT Init\nNEXT Split\nINVARIANT Inv\n", 5, false,
			"invariant-violation 3/3 Inv x=0 x=2", "2:1/1",
		},
		{
			"split existential", dfidBatch, "INIT Init\nNEXT Ex\nINVARIANT Inv\n", 5, false,
			"invariant-violation 3/3 Inv x=0 x=2", "2:1/1",
		},
		{
			"search continues after a violation", dfidBatch, "INIT Init\nNEXT Chain\nINVARIANT InvTen\n", 8, false,
			"invariant-violation 5/4 InvTen x=0 x=1 x=10", "2:1/1 3:2/2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run, err := runDepthFirst(t, tc.spec, tc.cfg, tc.depth, tc.continueChecks)
			if err != nil {
				t.Fatal(err)
			}
			if got := dfidOutcome(run.result); got != tc.want {
				t.Errorf("result %q, want %q", got, tc.want)
			}
			if got := strings.Join(run.levels, " "); got != tc.levels {
				t.Errorf("levels %q, want %q", got, tc.levels)
			}
		})
	}
}

func TestDepthFirstReportsErrorsWhenFound(t *testing.T) {
	run, err := runDepthFirst(t, dfidGrid, "INIT Init\nNEXT Next\nCONSTRAINT Constraint\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n", 20, true)
	if err != nil || len(run.errors) != 1 || run.result.ContinuedViolations != 1 {
		t.Fatalf("errors %+v, result %+v, %v", run.errors, run.result, err)
	}
	if got := dfidTrace(run.errors[0].Violation.Trace); got != "x=1,y=0 x=1,y=1 x=1,y=2" {
		t.Fatalf("continued violation %q", got)
	}
	run, err = runDepthFirst(t, dfidBatch, "INIT Init\nNEXT Chain\nINVARIANT InvTen\n", 8, false)
	if err != nil || len(run.errors) != 1 || dfidOutcome(run.errors[0]) != "invariant-violation 5/4 InvTen x=0 x=1 x=10" {
		t.Fatalf("reported %+v, %v", run.errors, err)
	}
}

// TLC finishes the search below the current initial state after an error, so
// later output follows the error report.
func TestDepthFirstOutputAfterError(t *testing.T) {
	spec := `---- MODULE After ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == \/ x = 0 /\ x' = 1
        \/ x = 0 /\ x' = 2
        \/ x = 1 /\ x' = 5
        \/ x = 2 /\ PrintT("after") /\ x' = 6
Inv == x # 5
====`
	var events []string
	options := Options{
		DepthFirst: &DepthFirstOptions{MaxDepth: 5, OnError: func(r Result) { events = append(events, "error "+dfidTrace(r.Violation.Trace)) }},
		OnPrint:    func(text string) { events = append(events, "print "+text) },
	}
	result, err := CheckWithOptions(context.Background(), spec, "INIT Init\nNEXT Next\nINVARIANT Inv\n", options)
	if err != nil || dfidOutcome(result) != "invariant-violation 7/5 Inv x=0 x=1 x=5" {
		t.Fatalf("result %q, %v", dfidOutcome(result), err)
	}
	if got := strings.Join(events, "; "); got != `error x=0 x=1 x=5; print "after"` {
		t.Fatalf("events %q", got)
	}
}

func TestDepthFirstEvaluationFailures(t *testing.T) {
	cases := []struct {
		name, spec, cfg, want string
		detail                bool
	}{
		{"TLCSet exit fails", dfidCounter, "INIT Init\nNEXT Next\nINVARIANT Exit\n", "error 5/3 2111/76 x=0 x=1 x=2", false},
		{"action property replay", dfidCounter, "INIT Init\nNEXT Next\nPROPERTY ActErr\n", "error 5/3 2113/13 x=0 x=1 x=2", true},
		{"next-state relation", dfidCounter, "INIT Init\nNEXT Choose\n", "error 6/3 1000/255 x=0 x=1 x=2", true},
		{"initial invariant", dfidBatch, "INIT Init\nNEXT Split\nINVARIANT DivInit\n", "error 1/1 2102/0 ", false},
		{"incomplete initial state", dfidGrid, "INIT Partial\nNEXT Next\n", "error 0/0 2106/255 ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run, err := runDepthFirst(t, tc.spec, tc.cfg, 10, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := dfidOutcome(run.result); got != tc.want {
				t.Errorf("result %q, want %q", got, tc.want)
			}
			if got := run.result.Failure.EvaluationDetail != ""; got != tc.detail {
				t.Errorf("nested expressions reported %v: %q", got, run.result.Failure.EvaluationDetail)
			}
			if len(run.errors) != 1 {
				t.Errorf("reported %d errors", len(run.errors))
			}
		})
	}
}

// TLC names the Java exception behind an error in the next-state relation and
// nests a coded message in tool output.
func TestDepthFirstExceptionMessages(t *testing.T) {
	for _, tc := range []struct{ next, class, nested string }{
		{"Choose", "java.lang.RuntimeException\n: Attempted to compute the value of an expression of form\nCHOOSE x \\in S: P, but no element of S satisfied P.", ""},
		{"Div", "tlc2.tool.EvalException\n: The second argument of \\div is 0.", "2179 The second argument of \\div is 0."},
		{"ExitNext", "java.lang.RuntimeException\n: Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.Value tlc2.module.TLC.TLCSet(", "2154 Attempted to apply the operator overridden by the Java method"},
	} {
		run, err := runDepthFirst(t, dfidCounter, "INIT Init\nNEXT "+tc.next+"\n", 10, false)
		if err != nil || run.result.Failure == nil {
			t.Fatalf("%s: %+v, %v", tc.next, run.result, err)
		}
		failure := run.result.Failure
		nested := fmt.Sprintf("%d %s", failure.NestedToolCode, failure.NestedMessage)
		if !strings.Contains(failure.Message, "The error occurred when TLC was computing the set of next states.\nThe exception was a "+tc.class) || !strings.HasPrefix(nested, tc.nested) || (tc.nested == "") != (failure.NestedToolCode == 0) {
			t.Errorf("%s: message %q, nested %q", tc.next, failure.Message, nested)
		}
	}
}

func TestDepthFirstRuntimeValues(t *testing.T) {
	run, err := runDepthFirst(t, dfidCounter, "INIT Init\nNEXT Next\nINVARIANT Stats\nCHECK_DEADLOCK FALSE\n", 3, false)
	if err != nil || run.result.Status != DepthLimit {
		t.Fatalf("result %+v, %v", run.result, err)
	}
	for _, line := range run.printed {
		if line != "<<0, -1, -1, -1>>" {
			t.Fatalf("TLCGet values %q, want TLC's depth-first values", run.printed)
		}
	}
	_, err = runDepthFirst(t, dfidCounter, "INIT Init\nNEXT Wrap\nPROPERTY Live\n", 10, false)
	var validation *ModelValidationError
	if !errors.As(err, &validation) || validation.Code != 1000 || validation.ExitCode != 255 || !strings.Contains(validation.Message, "does not support checking liveness") {
		t.Fatalf("liveness error %v", err)
	}
	// TLC rejects several workers before liveness properties.
	_, err = CheckWithOptions(context.Background(), dfidCounter, "INIT Init\nNEXT Wrap\nPROPERTY Live\n", Options{DepthFirst: &DepthFirstOptions{MaxDepth: 10, Workers: 2}})
	if !errors.As(err, &validation) || validation.ExitCode != 255 || !strings.Contains(validation.Message, "does not support multiple workers") {
		t.Fatalf("workers error %v", err)
	}
	_, err = runDepthFirst(t, dfidCounter, "INIT Init\nNEXT Next\nPOSTCONDITION Post\n", 10, false)
	if err == nil || !strings.Contains(err.Error(), "not supported in depth-first mode") {
		t.Fatalf("postcondition error %v", err)
	}
}

func TestDepthFirstSymmetryAndView(t *testing.T) {
	spec := `---- MODULE Sym ----
EXTENDS Naturals, TLC
CONSTANT P
VARIABLES owner, count
Init == owner = {} /\ count = 0
Next == \/ \E p \in P : p \notin owner /\ owner' = owner \cup {p} /\ count' = count + 1
        \/ owner # {} /\ owner' = {} /\ UNCHANGED count
Inv == count < 3
Perms == Permutations(P)
View == owner
====`
	for _, tc := range []struct{ cfg, want string }{
		{"SYMMETRY Perms\n", "invariant-violation 20/6 Inv count=0,owner={} count=1,owner={p1} count=2,owner={p1, p2} count=3,owner={p1, p2, p3}"},
		{"VIEW View\n", "invariant-violation 33/8 Inv count=0,owner={} count=1,owner={p1} count=2,owner={p1, p2} count=3,owner={p1, p2, p3}"},
	} {
		run, err := runDepthFirst(t, spec, "CONSTANT P = {p1, p2, p3}\nINIT Init\nNEXT Next\nINVARIANT Inv\n"+tc.cfg, 20, false)
		if err != nil || dfidOutcome(run.result) != tc.want {
			t.Errorf("%s: %q, %v", tc.cfg, dfidOutcome(run.result), err)
		}
	}
}

func TestDepthFirstCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CheckWithOptions(ctx, dfidCounter, "INIT Init\nNEXT Next\n", Options{DepthFirst: &DepthFirstOptions{MaxDepth: 10}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want cancellation", err)
	}
}

func TestDepthFirstWithoutVariables(t *testing.T) {
	run, err := runDepthFirst(t, "---- MODULE C ----\nASSUME TRUE\n====", "", 10, false)
	if err != nil || dfidOutcome(run.result) != "success 0/0" || !run.result.HasFingerprintDistance || run.result.FingerprintDistance != math.MaxInt64 {
		t.Fatalf("result %+v, %v", run.result, err)
	}
}

// javaCheckFPs is a direct port of MemFPIntSet.checkFPs for small sets.
func javaCheckFPs(fingerprints []uint64) int64 {
	mask := dfidTableMask(len(fingerprints))
	buckets := map[uint64][]uint64{}
	var order []uint64
	for _, fp := range fingerprints {
		if _, ok := buckets[fp&mask]; !ok {
			order = append(order, fp&mask)
		}
		buckets[fp&mask] = append(buckets[fp&mask], fp)
	}
	slices.Sort(order)
	value := func(fp uint64) int64 { return int64(int32(fp>>32))<<32 | int64(int32(fp)) }
	distance := int64(math.MaxInt64)
	for i, index := range order {
		bucket := buckets[index]
		for j := range bucket {
			for k := j + 1; k < len(bucket); k++ {
				d := value(bucket[j]) - value(bucket[k])
				if d < 0 {
					d = -d
				}
				distance = min(distance, d)
			}
			for _, other := range order[i+1:] {
				for _, fp := range buckets[other] {
					x, y := value(bucket[j]), value(fp)
					d := y - x
					if x > y {
						d = x - y
					}
					if d >= 0 {
						distance = min(distance, d)
					}
				}
			}
		}
	}
	return distance
}

func TestDFIDFingerprintDistanceMatchesTLC(t *testing.T) {
	cases := [][]uint64{
		nil,
		{42},
		{0x0000000100000005, 0x0000000200000009},
		{0x7fffffff00010000, 0x8000000000010000},
		{0x1234567880000001, 0x7654321080000001},
		{0x00000001ffff0003, 0x0000000500000003, 0x7fffffff7fff0003},
	}
	random := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		set := map[uint64]bool{}
		for range random.IntN(40) + 2 {
			fp := random.Uint64()
			if random.IntN(3) == 0 {
				fp = fp&^0xffff | 7
			}
			set[fp] = true
		}
		cases = append(cases, slices.Collect(func(yield func(uint64) bool) {
			for fp := range set {
				if !yield(fp) {
					return
				}
			}
		}))
	}
	for _, fingerprints := range cases {
		if got, want := dfidFingerprintDistance(fingerprints), javaCheckFPs(fingerprints); got != want {
			t.Fatalf("distance for %x = %d, want %d", fingerprints, got, want)
		}
	}
}
