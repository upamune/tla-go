# Upstream JUnit differential sweep (incomplete)

## Final liveness strategy investigation (2026-11-01)

No strategy change was made. In pinned Java 1.7.4, `-lncheck final`
sets `TLCGlobals.lnCheck`; `doLiveness()` prevents ordinary periodic
`LiveCheck.check` calls from checking an incomplete graph. The JUnit base
class instead sets `livenessThreshold` to `Double.MAX_VALUE`: ordinary
periodic checks use a graph-growth ratio and are normally suppressed. A
management-requested `forceCheck` bypasses both settings, so it does not
establish a difference between them. The source initializes the previous graph
size to one, but `recordSize()` can update it. If it were zero at an ordinary
check, positive growth would yield infinity and zero growth NaN; infinity exceeds
even `Double.MAX_VALUE`, while NaN does not. Reachability of a zero previous
size at such a check has not been established. This is a source-level potential
distinction, not a demonstrated behavioral difference. Both paths construct
the liveness graph during exploration, including stuttering edges and
state/action evaluation;
`ModelChecker` invokes `finalCheck` after a successful search without an
error state. Simulation instead invokes `LiveCheck1.checkTrace` on traces.

Go currently consumes and ignores the strategy argument. Its
`checkIncrementalLiveness` evaluates and caches temporal leaves during each
expansion; the final graph algorithm runs in `finishLiveness`. This is not the
same operation as Java's periodic SCC check. Disabling it for `final` would
therefore also change evaluation timing, errors and effects, rather than just
defer Java's periodic graph check. Java's tableau can regenerate successors
and re-evaluate constraints and effects for late nodes. Go deliberately omits
2192 and 2267 when its progress count cannot safely reproduce this behavior.
Changing the option or printing a count derived from explored states would not
fix the observed effectful cases safely. Default, simulation, `-continue`,
and forced-check behavior would need separate compatibility work.

Reproduced with `/tmp/tlaplus-java-v1.7.4` at commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` and the pinned
`/tmp/tla2tools-v1.7.4.jar` (SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`).
The existing pinned partial differential reproduced Java 2192 counts 12, 13,
13, 16, 9 and 94 for Factorial2, EvenOdd, EvenOddBad, InnerLabeledIf,
NoLoop2 and Euclid3, while Go omitted 2192 and 2267 in those six runs. The
pure CCallReturn1 control reported 8 on both sides. These tests compare
selected observations and translation/dump output, not JUnit execution.

An additional three-state model with `WF_x(Next)` and the false eventuality
`<>(x = 4)` produced exit 13, 2192 count 3, a loop back to state 1, and
statistics 4 generated / 3 distinct / 0 queued under Java's max-threshold
launcher, Java `-lncheck final`, and Go `-lncheck final`. Java simulation
with either max threshold or `-lncheck final`, seed 1 and depth 5 also exited
13 with a loop back to state 1; this is one example, not proof of equivalence.
The model defines `Init == x = 0`, `Next == x' = (x + 1) % 3`,
`Spec == Init /\ [][Next]_x /\ WF_x(Next)` and `P == <>(x = 4)`;
the config selects `SPECIFICATION Spec`, `PROPERTY P` and
`CHECK_DEADLOCK FALSE`. Both model-checking commands used `-tool -workers 1
-fp 0`; the simulation commands additionally used `-simulate num=1 -seed 1
-depth 5`. The max-threshold launcher uses the same assignment as the sweep's
`javaWrapper` before calling `TLC.main`.

To reproduce this standalone experiment, run the following from the repository
root with a JDK, Go, `mise`, `curl`, and `shasum` installed. It downloads the
pinned release jar and checks its SHA-256 before executing it. It creates a new
temporary directory, leaves its outputs there for inspection, and does not
overwrite existing models. Each run is finite; exit 13 is the expected liveness
violation, not a shell failure. Output timestamps and durations vary.

```sh
repo_dir=$(pwd)
experiment_dir=$(mktemp -d "${TMPDIR:-/tmp}/tla-liveness-XXXXXX") || exit 1
curl -fL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$experiment_dir/tla2tools.jar" || exit 1
printf '%s  %s\n' 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 "$experiment_dir/tla2tools.jar" | shasum -a 256 -c || exit 1
mise exec -- go build -o "$experiment_dir/tla-go" ./cmd/tla-go || exit 1
cd "$experiment_dir" || exit 1
cat > M.tla <<'EOF'
---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 3
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == <> (x = 4)
Inv == x < 2
====
EOF
cat > M.cfg <<'EOF'
SPECIFICATION Spec
PROPERTY P
CHECK_DEADLOCK FALSE
EOF
cat > Max.java <<'EOF'
public class Max {
  public static void main(String[] args) throws Exception {
    tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE;
    tlc2.TLC.main(args);
  }
}
EOF
javac -cp tla2tools.jar Max.java || exit 1

run_case() {
  name=$1
  shift
  "$@" > "$name.out" 2>&1
  status=$?
  printf '%s exit: %s\n' "$name" "$status"
  [ "$status" -eq 13 ] || return 1
}
run_case max java -cp .:tla2tools.jar Max -tool -workers 1 -fp 0 -metadir meta-max M || exit 1
run_case final java -cp tla2tools.jar tlc2.TLC -tool -workers 1 -fp 0 -lncheck final -metadir meta-final M || exit 1
run_case go-final ./tla-go -tool -workers 1 -fp 0 -lncheck final -metadir meta-go M || exit 1
run_case sim-max java -cp .:tla2tools.jar Max -tool -workers 1 -fp 0 -simulate num=1 -seed 1 -depth 5 -metadir meta-sim-max M || exit 1
run_case sim-final java -cp tla2tools.jar tlc2.TLC -tool -workers 1 -fp 0 -lncheck final -simulate num=1 -seed 1 -depth 5 -metadir meta-sim-final M || exit 1
printf 'Outputs: %s\n' "$experiment_dir"
cd "$repo_dir" || exit 1
```

Inspect `max.out`, `final.out`, and `go-final.out` for message 2192 (3 total
distinct states), the loop back to state 1, and the 4 / 3 / 0 statistics. Inspect
`sim-max.out` and `sim-final.out` for their loop back to state 1. These are
selected observations, not a comparison of the entire output.

The pinned Java comparison tests for temporal progress include success,
fairness, stuttering/tableau violations, constraints, invariant and evaluation
errors; model and simulation `-continue` and simulation liveness tests also
passed. These tests compare selected messages, and most `-continue` and
simulation cases use their default strategy rather than `-lncheck final`.
Tests for newer Java postconditions were skipped without `TLC_MODERN_JAR`.

Reproduction commands for the checked-in partial comparisons:

```sh
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar \
  mise exec -- go test ./internal/upstreamjunit -run 'TestPinned(CCallReturn1TemporalProgress|EffectfulPlusCalTableauProgressGuard)$' -v -count=1
TLC_JAR=/tmp/tla2tools-v1.7.4.jar mise exec -- go test ./cmd/tla-go \
  -run 'Test(JavaTLCTemporalProgress|PinnedJavaFairnessProgressEffects|JavaTLCSimulationLiveness|JavaTLCSimulationContinue|JavaTLCModelContinue|JavaTLCTraceStyle|JavaTLCLivenessBranches)$' -count=1
```

These observations do not classify all 14 snapshot differences, prove general
strategy equivalence, or establish JUnit parity. A scoped implementation needs
Java-compatible graph insertion and late tableau replay with ordered effects,
plus comparisons covering forced checks, safety errors, `-continue`, simulation,
stuttering and statistics before connecting a strategy flag to it.

This is not a 336-test sweep and no match count is claimed. Inventory is a
lexical candidate inventory, not JUnit discovery. It can include non-tests and
miss tests. Resolution follows literal constructor delegation and inherited
literal setup overrides, including `this(...)` delegation and literal paths
joined with `File.separator`; unsupported expressions, hooks, and harnesses produce
explicit reasons. The checked-in Java sources in `testdata` are unchanged
regression fixtures from the pinned upstream revision; EC.java is the upstream
message-code registry. They retain their upstream licenses.

The sweep inventories Java paths from Git commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` after verifying the checkout and
jar. Untracked and ignored Java files are excluded. `Inventory` also supports
ordinary source directories for the checked-in regression fixtures.

Run unit tests with `go test ./internal/upstreamjunit`. To inspect the pinned
inventory and run up to 25 diagnostic invocations:

```sh
TLA_GO_JUNIT_SWEEP=1 \
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 \
TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar \
TLA_GO_JUNIT_LIMIT=25 \
go test -v ./internal/upstreamjunit -run '^TestUpstreamJUnitSweep$' -count=1
```

Set `TLA_GO_JUNIT_FILTER` to a substring of a class name to select diagnostics.
The enabled test deliberately fails while coverage is incomplete. With no limit
it inventories only. It checks the pinned checkout and jar, copies tracked
model files separately for Java and Go, preserves relative model paths, applies
process timeouts and translates PlusCal in each copy before running TLC. A Java
launcher sets `TLCGlobals.livenessThreshold` to `Double.MAX_VALUE`, as the
upstream JUnit harness does. Go performs exploration-time liveness processing;
equivalence to the JUnit final-only graph check is unverified, so all diagnostics
remain unresolved with respect to liveness setup. It
compares translated TLA and CFG files when applicable. Test-specific Java
classes are not compiled or loaded; C and other Java overrides therefore remain
unresolved. No output from these diagnostic invocations is a JUnit match.

For `pcal.Euclid3Test`, the pinned Java diagnostic reports 94 fingerprints in
2192 while Go omits the message. The fairness action `Next` reaches `lp` and
its `PrintT`, so `safeTableauProgress` rejects the effectful definition. WF/SF
diagnostic normalization itself only constructs leaves for the fairness action
and subscript; it does not evaluate them. This allows the initial 2212 solution
count for such WF/SF formulas. Final progress remains guarded even without a
tableau because effect counts and recovery ordering can differ. One narrow
exception is a completed single-solution search where the only temporal property
is a simple `[]<>` state comparison true in every explored state; see
[`model-fairness-graph-insertion.md`](../../docs/investigations/model-fairness-graph-insertion.md).
Recurrence formulas still require the diagnostic
safety check. Java's late tableau
expansion regenerates successors and rechecks both constraints; effects and
dynamic expressions can change the graph, so Go cannot derive this count from
its completed exploration without reproducing that replay. This is a deliberate
omission, not evidence that Java's count equals Go's distinct-state count in
general. The direct `TestPinnedCCallReturn1TemporalProgress` comparison checks
the independent pure case's 2192 count of 8, but remains a partial diagnostic.

The diagnostic ignores Java comments and literals when finding recorder message
IDs and ignores comments when extracting literal assertion values. It compares
selected tool messages, including 2194. For 2190, 2191, and 2192 it removes only
the timestamp appended by upstream `MP.java`; counts and all other text remain.
It evaluates literal presence/absence assertions and initial-state, temporal,
numeric depth and statistics values separately on each side.
For direct subclasses of the pinned `SuiteTestCase` and `SuiteETestCase`, the
resolver derives the lowercased model name and `suite` path, handles literal
constructor forms and inherited literal flag overrides, and records the regular
suite's inherited `testSpec` observations unless overridden. Suite error output
interception and substring assertions remain omissions. Constructor side effects,
custom hooks, indirect suite inheritance and dynamic setup prevent execution.
Other recorder parameter forms, numeric predicates, coverage assertions,
translation mapping/message assertions, arbitrary JUnit assertions, other inherited
assertions, test-specific Java overrides, and nonliteral setup are omissions.
Java runs with inherited coverage enabled; Go does not, pending its separate
coverage integration. DOT dumps are enabled on both sides and compared byte for
byte, using isolated explicit output paths rather than the harness's metadir path.
The dump path and cleanup lifecycle are therefore not equivalent. Unsupported
or ambiguous constructor expressions and custom setup hooks are not run.
The harness does not yet prove that its extracted observations are exhaustive,
execute JUnit lifecycle methods, or establish the requested 336-test denominator.

## Diagnostic snapshot (2026-11-01)

Against the verified Java TLC 1.7.4 checkout and jar, the lexical inventory
reported 340 non-abstract candidates, 236 with a recovered spec, and 340
unresolved candidates. This is not JUnit discovery and cannot establish the
336-case denominator. The enabled test intentionally fails to make that status
visible. The earlier 25-invocation snapshot used Java's default liveness
threshold and included commented-out assertions; its 2192 counts should not be
used as compatibility results. The rerun under the corrected setup is described
below. All results remain partial comparisons, not JUnit passes.

With the corrected Java threshold and comment handling, a limit-25 rerun
completed 24 comparisons: 15 reported Java 2192 messages absent from Go, and
nine reported no selected differences. `pcal.DetlefsTest` timed out on Java.
For example, `pcal.CCallReturn1Test` reported Java's complete state space of
8 states and no Go 2192 message. These counts happen to equal the earlier
snapshot, but the Go liveness strategy remains unresolved; they do not establish
15 JUnit failures or nine passes.

After tableau progress reporting and the subsequent diagnostic changes, a new
limit-25 run completed 24 comparisons: 23 had no selected differences and
`pcal.Euclid3Test` lacked Java's 2192 message in Go. Java timed out on
`pcal.DetlefsTest`. The newer diagnostic also requests Java coverage and compares
DOT output, so these numbers are not directly comparable to the earlier run.
Both snapshots remain partial comparisons, not JUnit pass counts.

After the progress-count change, a filtered `CCallReturn1Test` diagnostic
reported no selected differences and both sides satisfied the extracted
assertions, including 2192's count of 8. The sweep still intentionally failed:
coverage, lifecycle, translation messages, test-specific overrides, and the
equivalence of Go's exploration-time liveness strategy remain unresolved.

In Java 1.7.4 `LiveCheck.check0` sums each checker's `diskGraph.size()` before
checking and prints 2267 after the result (including the violation trace).
`DiskGraph` counts fingerprint entries; despite the product graph,
`TableauDiskGraph.size()` delegates to `TableauNodePtrTable.size()`, which
counts fingerprint keys, not the tableau indices stored under each key.
`setDone` inserts even a fingerprint with no consistent tableau node. Completed,
unreduced pure exploration therefore provides the count per branch: every
explored fingerprint is marked done, and replay of a done state cannot discover
an outside fingerprint. This is not a general equivalence between reachable
product nodes and states. Go continues to omit tableau progress when reduction
or unstable/effectful definitions make replay equivalence unproven, and omits
oversized or unsafe normalization. It does not reproduce intermediate Java
liveness checks.

After suite resolution was added, the verified inventory reported 340 recovered
specs, 31 candidates with unsafe or unresolved invocation setup, and 340 unresolved
non-abstract candidates. Each of three filtered runs attempted one case. Test10
and Test22 had no selected differences and both sides satisfied the extracted
inherited recorder observations; both exited 0. ETest1 exited 150 on both sides,
with no selected recorder observations: its intercepted error substring was not
checked. The enabled sweep failed intentionally in all three runs. Coverage,
dump, lifecycle, exit expectations, and liveness equivalence remain unresolved;
none of these results is a JUnit pass or proof of the 336-test denominator.

## Test10 focused differential

`TestPinnedSuiteTest10Partial` verifies the pinned checkout and jar, runs the
real `suite/test10` config and model, and checks both exits against the inherited
success expectation, the four inherited recorder assertions, and DOT contents.
Java receives `-coverage 1` and its complete coverage report is projected using
`TestMPRecorder.getZeroCoverage` (2221 records only). On the 1.7.4 oracle this
set is empty. The Go coverage flag and zero-coverage assertion remain missing;
an absent report is never accepted as evidence of zero coverage. Set
`TLA_GO_JUNIT_ROOT` and `TLA_GO_JUNIT_JAR` to run this partial regression.

The upstream lifecycle calls `beforeSetUp`, sets vetoCleanup and the user dir,
installs a recorder, sets the final-only liveness threshold, invokes TLC with
`-deadlock -fp 0 -coverage 1 -workers 1 -checkpoint 0 -dump dot` and the
class-named metadir output, then asserts the exit in `tearDown`. Test10 has no
overrides. The diagnostic uses a launcher and tool messages instead of the
actual recorder/lifecycle, a fixed dump path, and Go's exploration-time
liveness strategy. Test10's config has INIT, NEXT and INVARIANT but no temporal
property. Recorder equivalence, lifecycle effects, metadir cleanup, and Go
coverage remain unproven; this is not a JUnit pass.

## JUnit runner discovery (opt-in)

The discovery test compiles **all tracked files** in upstream `test/` against the
verified TLC jar, JUnit 4.12 and Hamcrest Core 1.3, then asks JUnit
`Request.aClass(...).getRunner().getDescription()` for each non-abstract lexical
candidate class. It prints one record per described leaf, including the owning
candidate class and JUnit display name; this is the mapping needed to distinguish
inherited methods and parameterized instances. It reports ignored leaves,
class-level ignores and runner initialization errors separately.
The latter are not executable test cases. A class-level `@Ignore` is represented
by JUnit as a single ignored class, without enumerating its methods.
Nested unannotated `initializationError` descriptions are reported as errors:
they may be initialization failures or ambiguous custom-runner descriptions.
An `initializationError` leaf carrying `@Test` is retained as a test method.
Runner construction can execute code, including parameter factories; discovery
does not execute test methods or their assertions. JUnit descriptions do not reliably distinguish an empty suite from a leaf. Custom runners can choose
what they expose, and the results depend on the Java runtime and environment.
Discovery records use an isolated temporary protocol file; runner stdout and
stderr are retained for diagnostics if discovery fails.
The sweep and pinned discovery callers give Git input verification, compilation,
and runner construction a shared five-minute context, independently of the remaining
test work. The context bounds subprocesses; filesystem operations themselves can
still block. A one-second
`WaitDelay` starts when that context ends or the direct command exits, whichever
occurs first; it bounds waiting for inherited output pipes, not a running command
whose context has no deadline. If a command exits successfully but pipes remain open,
discovery returns `exec.ErrWaitDelay` and does not consume the protocol. A nonzero
exit retains its exit error; cancellation reports the context error. Cancellation
kills the direct command only, and closing pipes does not kill or reap descendants.
Descendants may outlive discovery on any OS, so use trusted inputs and external
process isolation and cleanup. Temporary protocol removal is best effort, not a
guarantee. Runner code is not sandboxed and can access the protocol file and other
process resources.

The inputs are checked by SHA-256: TLC jar `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`,
JUnit 4.12 `59721f0805e223d84b90677887d9ff567dc534d7c502ca903c0c2b17f05c116a`,
and Hamcrest Core 1.3 `66fdef91e9739348df7a096aa384a5685f4e875584cce89386a7a47251c4d8e9`.
The latter two are Maven Central artifacts `junit:junit:4.12` and
`org.hamcrest:hamcrest-core:1.3`; versions match upstream `customBuild.xml`.
The checkout must have the pinned HEAD and no tracked modifications. The test
source list comes from the pinned Git tree, so untracked sources are excluded.

```sh
TLA_GO_JUNIT_DISCOVERY=1 \
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 \
TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar \
TLA_GO_JUNIT_JUNIT=/tmp/junit-4.12.jar \
TLA_GO_JUNIT_HAMCREST=/tmp/hamcrest-core-1.3.jar \
go test -v ./internal/upstreamjunit -run 'Test(JUnitDiscoveryFixture|UpstreamJUnitDiscovery)$' -count=1
```

On 2026-11-01, OpenJDK 21.0.11 reported 322 case descriptions without an
`@Ignore` annotation,
20 uncertain descriptions and one class-level ignore among 340 lexical
non-abstract candidate classes. No runner errors were reported. The uncertain
descriptions carry ignore annotations under custom runners; annotations alone do
not establish whether such a runner executes them. A case description also does
not guarantee execution by a custom runner. `tlc2.tool.Github432Test` accounts
for four case descriptions in one class. The command prints individual names
and reasons. These counts cover only the explicitly selected candidate classes,
not all upstream tests or the upstream Ant suite: Ant selects source filenames
and uses multiple batches and exclusions, and the lexical inheritance scan can
miss classes. The full set has not been mapped individually to Go runs or proven
equivalent. Existing class-based diagnostic runs and extracted
observations are not JUnit executions.

## Executing upstream JUnit cases (opt-in experiment)

`TestUpstreamJUnitExecution` discovers and runs
`tlc2.tool.suite.Test10#testSpec`, `tlc2.tool.EmptyTest#testSpec`, and
`tlc2.tool.coverage.BCoverageTest#testSpec` separately with JUnit 4.12's
`Request.method`, each in a fresh JVM. For Test10, the
upstream isolated runner executes the inherited
`SuiteTestCase.setUp`, `ModelCheckerTestCase.setUp` (including TLC and its
recorder), the original `SuiteTestCase.testSpec` assertions including zero
uncovered expressions, and `ModelCheckerTestCase.tearDown` exit assertion.
JUnit's started/finished events and result must identify exactly the requested
case in each run with no failure, ignored test, or failed assumption. A Java JUnit pass means
those original Java assertions and lifecycle completed; it is not a Go pass.
JUnit reports test events, not individual lifecycle callbacks; the lifecycle
mapping above is specific to the verified pinned source and this class.

The experiment checks the same commit and three jar hashes as discovery. It
compiles the tracked upstream test sources, copies tracked model files to a
private temporary `test-model`, and extracts standard modules from the verified
jar into temporary classes for the upstream isolated class loader. The JVM's
working directory, `basedir`, and temporary directory are private; TLC output
is removed with the temporary directory. The checkout is not written. The JVM
is terminated after JUnit finishes because TLC can leave RMI threads alive.

The linked Go diagnostic is a separate invocation of the existing `RunCase`
for the same class/model: it compares selected tool messages, exit status and
DOT dump. It does not execute the Java assertions on Go, nor reproduce Java's
recorder, coverage assertion, final-only liveness strategy, or lifecycle. A
mismatch fails as a **Go mismatch**; a match is only a **partial Go diagnostic**.
Unexpected discovery or lifecycle selection is unsupported, and ignored,
initialization-error, assertion-failure, assumption, missing-event and process
errors cannot become passes. The fixture exercises these cases with real JUnit.
Only the explicitly analyzed cases are supported; arbitrary methods, custom fixtures, parameter
factories, overrides, and alternate lifecycle hooks need explicit analysis and
isolation before expanding the allowlist. An eventual Go JUnit pass would need
an adapter that executes the original assertions against Go observations and
reproduces every relevant lifecycle effect, with negative tests proving failures
are detected.

For example, prepare the inputs with Git, a JDK (including `javac` and `jar`),
Go, and `curl` (the hashes above are verified by the test):

```sh
git clone https://github.com/tlaplus/tlaplus.git /tmp/tlaplus-java-v1.7.4
git -C /tmp/tlaplus-java-v1.7.4 checkout --detach 5a47802b5c391f59ecdd44117981f4ff8c0656ba
curl -fL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o /tmp/tla2tools-v1.7.4.jar
curl -fL https://repo.maven.apache.org/maven2/junit/junit/4.12/junit-4.12.jar -o /tmp/junit-4.12.jar
curl -fL https://repo.maven.apache.org/maven2/org/hamcrest/hamcrest-core/1.3/hamcrest-core-1.3.jar -o /tmp/hamcrest-core-1.3.jar
```

```sh
TLA_GO_JUNIT_EXECUTION=1 \
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 \
TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar \
TLA_GO_JUNIT_JUNIT=/tmp/junit-4.12.jar \
TLA_GO_JUNIT_HAMCREST=/tmp/hamcrest-core-1.3.jar \
go test -v ./internal/upstreamjunit -run 'Test(ValidateExecution|JUnitExecutionFixture|UpstreamJUnitExecution)$' -count=1
```

`TestUpstreamJUnitExecution` also executes the pinned upstream
`BCoverageTest#testSpec` with its actual Java setup, coverage assertions, and
teardown. The Go CLI's matching `B` coverage report, state counts, search depth,
and tool coverage messages are checked separately by `TestJavaTLCUpstreamBooleanCoverage`;
this is not a Go JUnit lifecycle pass.

## Test10 assertion-method bridge (experimental, unsupported result)

The opt-in `TestUpstreamJUnitExecution` also runs Go with coverage enabled on a
separate copied model directory. A retry without coverage is permitted only for
exit 255 with either the exact legacy periodic-coverage CLI rejection or the
complete observed Test10 tool stream reporting the unsupported coverage model
shape. If coverage succeeds, its observations are retained. Timeouts, other exits, and malformed or
truncated output fail the experiment. The driver records the omission and feeds
a conservative projection of that output to a fresh upstream `Test10` instance. A JUnit wrapper invokes the
original inherited `testSpec` method itself. It does not translate the method's
assertions. The synthetic controls run that same method with changed statistics,
a missing finish record, a general error and zero coverage; each must produce an
`AssertionError` from the original method. Process errors, malformed event
sequences and other exceptions fail the experiment rather than counting as rejection.
The existing direct JUnit invocation runs the original Java setup, method and
teardown separately for comparison.

This is **not a Go JUnit pass**, even if the method returns successfully. The
adapter supports well-formed 2186, 2190, 2194, 2199, 2221 and 2772–2775 text messages; it
records every other code and malformed supported message as an omission. For
`GENERAL` (1000) and coverage mismatch (2776), it records presence using an
empty placeholder array and reports that the original arguments are unavailable.
The placeholder supports presence assertions only. Its
encoder rejects unknown record codes. Every text projection includes an
omission for unproven recorder types and unobserved calls, even if every
printed message was recognized. Tool output is rendered text, not the
complete typed MP recorder stream: projected events follow tool start-marker
order and preserve duplicate records, but that does not establish Java recorder
call order. Nested messages explicitly add an omission. Other argument types,
hidden parameters and recorder calls without tool output are not established.
In particular, absence of a complete Go coverage stream is
not proof of zero uncovered expressions, even though upstream's recorder treats
no 2221 records as an empty set. The wrapper does not run upstream setup or
teardown on the bridged instance. Initial process globals, final-only liveness,
metadir and cleanup behavior, the exact output lifecycle and exit-status
conversion are not reproduced or verified. The Go process exit and omissions
are reported; they cannot turn the result into a pass. A broader bridge requires
a typed recorder interface from Go, complete lifecycle equivalence, and an
explicit completeness contract before it can claim one.

The experiment additionally runs three test subclasses, `GoSuiteTest10`,
`GoEmptyTest` and `GoBCoverageTest`, through JUnit itself. Their inherited upstream `testSpec` and
`ModelCheckerTestCase.tearDown` execute on Go's projected records and exit
status. The subclass replaces the upstream setup with an adapter and consumes
the already completed Go invocation; it does not run Java TLC. Negative
controls verify that a wrong state count fails in the original test method and
a nonzero exit fails in the inherited exit-status assertion. The Test10
invocation currently retries without coverage if the exact documented
unsupported-coverage response occurs, so its zero-uncovered assertion on this
projection is not evidence that Go collected complete coverage. For both
subclasses, missing recorder events, upstream globals, setup, the JUnit-selected
class identity and cleanup behavior remain unresolved. A successful adapted
run is not an upstream JUnit pass for tla-go.

The recorder transfer preserves `String[]` versus `Object[]` and supports
strings, 32-bit integers, 64-bit integers, booleans and null in object arrays;
unknown types, malformed values and unsupported codes fail closed. A Java
fixture verifies the retained runtime array and element types, repetitions and
per-code order in the pinned `TestMPRecorder`, including rejection of malformed
input. This transport supports more than the text projection can infer: it does
not manufacture TLCStateInfo, exceptions or other Java objects from printed
text. Each text-projected record is explicitly a `String[]` hypothesis.

For the upstream B coverage model, the experiment also compares the projected
2190, 2194, 2199, 2221 and 2772–2775 records, including order, duplicates,
array type and arguments, against a fresh run of the verified Java TLC JAR with
an actual instrumented `MPRecorder`. This is a comparison of those selected
records for that model, not a completeness guarantee for other cases. The Go
invocation enables coverage but omits the upstream harness's DOT dump because
coverage and dump cannot currently be combined. The real `BCoverageTest`
method and inherited teardown execute against the projected records; a changed
action count must fail the original coverage assertion. Other recorder events,
setup and lifecycle effects remain unverified, so this is not an upstream JUnit
pass for Go.

## EmptyTest assertion-method bridge

The same opt-in experiment discovers and executes the pinned
`tlc2.tool.EmptyTest#testSpec` through its actual JUnit runner. Its inherited
setup invokes Java TLC, and its teardown checks the converted exit status. This
Java JUnit pass is separate from the Go observation experiment.

The Go invocation checks the harness's deadlock, fingerprint, worker,
checkpoint and duplicate `-coverage 1` arguments. Tool mode is added and its
DOT destination differs from the harness's metadir path. The experiment requires
exit zero and an empty-model output without a coverage report, projects the
FINISHED and state-count messages into string recorder arguments, preserves
GENERAL presence with a synthetic placeholder in tool-message order, and calls
the original `EmptyTest.testSpec` on a fresh
instance. Unknown and otherwise unrepresented messages are reported. Synthetic
altered counts, missing FINISHED and GENERAL must fail the original assertion;
disabled coverage, malformed output, unexpected coverage, errors and timeouts
are rejected by the driver.

A successful assertion is a partial match, not a Go JUnit pass. Rendered tool
messages do not establish the complete Java recorder calls or argument types;
the bridged instance does not run the original setup or teardown, and Java
process globals, side effects, metadir and dump behavior are not reproduced.
Absence of a coverage report for this empty model does not establish general
coverage equivalence. The experiment does not claim lifecycle completeness.

## Full diagnostic snapshot

Targeted follow-up for `ETest7` and `Github179a/b/c` reproduces the original
models with the pinned 1.7.4 jar and the resolver's worker, fingerprint,
deadlock and checkpoint flags. Tool-mode nested codes 2280 and 2154 were
missing from Go output; direct differential tests now cover the messages,
exit codes, selected evaluation stacks and trace states. The original snapshot
is retained as historical evidence. A rerun still reports a DOT content
difference for Github179a, and the resolver's coverage, dump lifecycle and
JUnit limitations described above remain. These diagnostics do not establish
JUnit pass counts or full compatibility.

For the later `56336d5` snapshot, direct one-worker runs with the pinned jar,
`-tool -deadlock -fp 0 -coverage 1 -workers 1 -checkpoint 0 -dump dot`, and
isolated original model files show that Github179a's failed assumption leaves
only the DOT header. Github798I and Github798N use `-config Github798I.tla`
and `-config Github798N.tla`; those arguments select absent `.tla.cfg` files,
so their header-only dumps are configuration errors, not explored graphs.
ETest12 through ETest15 likewise leave the header on assumption errors, while
ETest6 completes a graph containing its initial state after successor evaluation
fails. These are raw byte comparisons: the diagnostic runner does not normalize
DOT. Its Java-only coverage, liveness threshold and incomplete JUnit lifecycle
still limit the conclusions. ETest8 specifies simulation; parallel or random
exploration order is not made deterministic by these comparisons.

[`testdata/snapshots/sweep-2399b153.json`](testdata/snapshots/sweep-2399b153.json)
records the completed opt-in full run at tla-go commit
`2399b1535d1ef68aa78d81bba2b9559cf8e586f7`, against the pinned upstream
commit and Java 1.7.4 jar above. The source log SHA-256 is recorded in the JSON.
Recreate the snapshot from that exact log with:

```sh
go run ./internal/upstreamjunit/cmd/snapshot /path/to/tla-integration-sweep-full.log > snapshot.json
```

The later [`testdata/snapshots/sweep-56336d54.json`](testdata/snapshots/sweep-56336d54.json)
comes from `/tmp/tla-profiling-sweep-latest.log`, using the actual tla-go
binary revision `56336d54f659dbb744aff66ffa8e653a6117c06a` and the same
pinned upstream and jar documented above. Its exact raw source SHA-256 is
`fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec5`.
While the ephemeral source log is available, reproduce it with:

```sh
go run ./internal/upstreamjunit/cmd/snapshot /tmp/tla-profiling-sweep-latest.log > snapshot.json
```

Of 340 unresolved candidates, 309 were attempted and 31 had unsafe or
unresolved setup. Of the attempts, 293 comparisons completed: 236 had no
selected differences and 57 had selected differences; 16 comparisons were
unavailable. These are diagnostic observations, not JUnit passes.
Nondeterminism, timeouts and environmental differences mean that changes in
these counts alone are not evidence of improvements or regressions.

The third [`testdata/snapshots/sweep-e81f221f.json`](testdata/snapshots/sweep-e81f221f.json)
records the completed run from `/tmp/tla-e81-diagnostic-sweep.log`, using the
fixed worktree and binary revision `e81f221f7ba59b833244e7613bd149f836532c86`
with the same pinned upstream commit and Java TLC 1.7.4 jar. The exact raw log
SHA-256 is `65e957a50c1a58779f5982458ca42adb3d4ed16e0884229dd7bbe752aa298765`.
The absolute log path is ephemeral; the hash remains in the snapshot. While
the exact log is available, regenerate and compare it with:

```sh
go run ./internal/upstreamjunit/cmd/snapshot /tmp/tla-e81-diagnostic-sweep.log > snapshot.json
cmp snapshot.json internal/upstreamjunit/testdata/snapshots/sweep-e81f221f.json
```

The generator test also compares the regenerated bytes when the log is at
that path, or at the path supplied by `TLA_GO_E81_SWEEP_LOG`; it skips that
comparison when the default ephemeral log is absent.

All 340 candidates remain unresolved: 309 attempts produced 294 completed
comparisons (254 with no selected differences and 40 with selected
differences) and 15 unavailable comparisons; 31 candidates had unsafe or
unresolved setup. These are not JUnit pass counts. Timing, nondeterminism and
the unfinished harness mean the count changes do not establish an improvement
or regression.

The [`testdata/snapshots/sweep-9f581626.json`](testdata/snapshots/sweep-9f581626.json)
records the completed opt-in run from `/tmp/tla-9f58162-diagnostic-sweep.log`,
using detached source and binary revision `9f5816269083eb7c9d44e5477695e95e6bf68059`,
upstream `5a47802b` and Java TLC 1.7.4. The exact source log SHA-256 is
`e548e0fba387f59c1278069d2bb0bcd7196475e36a6049003336d2616bb13000`.
While the ephemeral log is available, verify the deterministic bytes with:

```sh
go run ./internal/upstreamjunit/cmd/snapshot /tmp/tla-9f58162-diagnostic-sweep.log > snapshot.json
cmp snapshot.json internal/upstreamjunit/testdata/snapshots/sweep-9f581626.json
```

The regeneration test uses that path by default, or `TLA_GO_9F58162_SWEEP_LOG`
when set; it skips if the default log is absent. Of 340 unresolved candidates,
309 were attempted and 31 had unsafe or unresolved setup. The attempts yielded
295 comparisons: 256 with no selected differences and 39 with selected
differences; 14 were unavailable. This is an incomplete diagnostic harness,
not JUnit pass counts. Timing and nondeterminism prevent these counts from
establishing an improvement or regression, including against the e81 run.

Only two structured class records differ from e81. `pcal.SimpleMultiProcTest`
timed out on Go in e81 but completed with no selected differences here; its
PlusCal translation and liveness assertions remain unresolved. The
[targeted investigation](../../docs/investigations/simple-multiproc.md) repeats
that model with pinned binaries and records timings and limitations.
`tlc2.tool.ContinueTest` reported `dump:content` in e81 but no selected
difference here; both runs use `-continue` and three workers. The sweep compares
DOT bytes, and does not preserve the differing dump in the snapshot. Repeat
the model with pinned binaries and capture both dumps across runs to determine
whether ordering or graph content differs before attributing this change to code.

The later [`testdata/snapshots/sweep-51c3a77c.json`](testdata/snapshots/sweep-51c3a77c.json)
records the full diagnostic log `/tmp/tla-51c3a77-sweep.log`, whose SHA-256 is
`606f46f5a51d30125f9b980a8eafccc03c40b219faff0194933eca28ac135598`.
The recorded run used a freshly created detached, unmodified worktree at
`51c3a77c6c4cbc02c6f388b0c90047cc895f9869`. The harness builds its temporary
`tla-go` binary from that worktree; the log itself does not prove the binary
revision or the command. The upstream checkout and 1.7.4 jar were verified
against the pinned commit and hash above. The recorded command, run from that
worktree, was:

```sh
MISE_AUTO_INSTALL=false TLA_GO_JUNIT_SWEEP=1 \
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 \
TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar TLA_GO_JUNIT_LIMIT=340 \
mise exec -- go test -v ./internal/upstreamjunit -run '^TestUpstreamJUnitSweep$' \
  -count=1 -timeout=120m > /tmp/tla-51c3a77-sweep.log 2>&1
```

The opt-in test intentionally fails because the harness is incomplete. This
run took 803.76 seconds and recorded 340 unresolved candidates, 309 attempted
runs and 31 unsafe or unresolved setups. Of the attempts, 293 were comparable:
254 had no selected differences and 39 had selected differences; 16 were
unavailable. These are partial diagnostic counts, not JUnit pass counts.
While the exact ephemeral log is available, regenerate the snapshot with:

```sh
mise exec -- go run ./internal/upstreamjunit/cmd/snapshot /tmp/tla-51c3a77-sweep.log > snapshot.json
cmp snapshot.json internal/upstreamjunit/testdata/snapshots/sweep-51c3a77c.json
```

The regeneration test uses that path, or the path in `TLA_GO_51C3A77_SWEEP_LOG`,
and skips when the default ephemeral log is absent. Compared with `9f581626`,
four structured records differ: `pcal.SimpleMultiProcTest` and
`tlc2.tool.liveness.CodePlexBug08EWD840FL2Test` became unavailable due to Go
and Java timeouts respectively; `tlc2.tool.ContinueTest` now reports
`dump:content`; and `tlc2.tool.coverage.BCoverageTest` has no selected
differences. The [SimpleMultiProc](../../docs/investigations/simple-multiproc.md)
and [ContinueTest](../../docs/investigations/continue-dot-sweep.md)
investigations provide additional observations. The coverage case also has a separate
targeted Java comparison, but the sweep still does not reproduce its complete
JUnit lifecycle. Timeouts, scheduling and other nondeterminism make snapshot
count changes insufficient to establish a general improvement or regression.
The later [timeout follow-up](../../docs/investigations/sweep-51c3a77-timeouts.md)
records bounded reruns of SimpleMultiProc and Java EWD840MC2.
The [DiameterTest investigation](../../docs/investigations/diameter-worker-order.md)
compares repeated worker-dependent depth and DOT output with Java 1.7.4.

The extractor recognizes only the five documented source hashes and associates
each with its recorded binary revision; the logs themselves do not establish
the binary revision. Unknown or modified logs are rejected.

The extractor checks the source hash, the completion marker, and the source
summary against the individual records. It strips raw output, temporary paths,
times and generated values. The JSON records selected message-body and presence
IDs, difference categories, and TLC exits where a comparison completed. The
translation failure category records its two translator exits; TLC exits are
unknown. Timeouts and missing tool messages are unavailable, not matches. The
31 `unsafe_setup` classifications are inferred from candidate records without a
diagnostic in this full, unfiltered run, corroborated by the summary count.

The 340 lexical candidates include 309 attempted and 31 unsafe; of the 309,
276 comparisons completed (224 with no selected differences, 52 with selected
differences) and 33 were unavailable. Every candidate remains unresolved for
full compatibility. These are neither JUnit executions nor JUnit pass counts.
The sweep still deliberately fails even when enabled: its lifecycle, coverage,
assertion and liveness omissions have not been resolved.

These observations do not constitute a permissive baseline rule or a sound automatic mismatch allowlist: scheduling,
model behavior, dump order, timeouts and infrastructure can vary. A new or
changed difference needs investigation, not automatic acceptance; an unavailable
or nondeterministic rerun remains unresolved. Comparing category names alone
also cannot detect changes in the underlying differing values. Use the snapshot
as an index for targeted reproduction, not a complete parity guard.

## Follow-up: symmetry fingerprints and record order

At tla-go `3840db5c62ad76e902af7cf0072737702c82d7ab`, the DOT path
fingerprinted the concrete state even though Java `DotStateWriter` calls
`TLCStateMut.fingerPrint`, which fingerprints the lexicographically smallest
symmetry image. For the OneBitMutex state shown in the preceding investigation,
the concrete state's Go FP64 was `-4001293080066318631`; its symmetry image
produces Java's `-3946579319044950398`. The label remains the concrete state;
the fingerprint is for its representative. The search already uses a separate
reduced state key. The DOT path now calls the existing Java fingerprint routine,
including its symmetry and VIEW handling, without changing that key or collision
semantics.

A new run with the same pinned checkout and jar, `-fp 0`, one worker, separate
model copies and explicit per-copy `-metadir metadata` produced byte-identical
NQ, OneBitMutex and TwoPhaseCommit dumps. For SymmetryTableau both dumps have
1,123 lines; masking only state label contents leaves identical lines in this
run. For instance both use ID `579606404721062638`, while the Java label has
`[op |-> "Rd", adr |-> e]` and Go has `[adr |-> e, op |-> "Rd"]`.
These are observed runs, not a guarantee of graph order across schedules. The
new Java oracle regression asserts selected IDs on both sides rather than full
DOT equality. The pinned no-symmetry OneBitMutex test checks 244 generated and
127 distinct states. Existing Unicode collision fixtures and a graph callback
control check that a Java FP64 collision still merges states where the checker
uses FP64.

A separate direct Java 1.7.4 probe used `FP64.Init(0)`, interned `op` before
`adr`, and constructed both `RecordValue` and `FcnRcdValue` for the mapping
`op -> "Rd", adr -> 1`. Both returned `0x56131a998eb0afd4`; the record printed
`[adr |-> 1, op |-> "Rd"]` before fingerprinting and
`[op |-> "Rd", adr |-> 1]` afterward. The corresponding Go direct test uses
the same intern order and expected fingerprint. Thus equal mathematical
records/functions and matching FP64 need not have equal display order. Java's
`RecordValue.fingerPrint` and `permute` normalize the record in place, and
`toString` reads its current field array. The Go value fingerprint orders a
copy and its formatter retains Go's representation. The probe establishes
that normalization can change presentation; it does not identify every point
where the Tableau record is shared or normalized during exploration.

Both implementations explicitly selected polynomial index 0 in the model
runs; the direct polynomial digest test covers all Java 1.7.4 polynomial
indices separately. A different polynomial is therefore not needed to explain
these particular ID differences. The run remains a partial harness diagnostic,
not a JUnit lifecycle result.

### Record-set lifecycle follow-up

An isolated instrumentation run against the pinned jar and source revision
shows a more specific cause for this witness than fingerprint-time mutation.
The source copy of `RecordValue` was compiled separately and placed before the
unchanged jar on the classpath; it logged object and array identities, field
order, normalization flag and call stacks. In one single-worker run, record
object `1109889625` was constructed by `SetOfRcdsValue.Enumerator` with names
`[op, adr]` and `isNorm=true`. Its name array was also used by another enumerated
record with a different values array. The first object was then visited through
both `FcnRcdValue.permute` and `TupleValue.permute` during state fingerprinting,
and subsequently formatted through both containers by `DotStateWriter`. Thus
the `buf` and `memInt` occurrences in this witness share the record itself;
the enumerated record already had the Java order before either operation.
These numeric identities are only meaningful within that process.

`SetOfRcdsValue` sorts its name/value arrays on construction using
`UniqueString.compareTo` (intern token order), and its enumerator passes the
same names array to each new, already-normalized record. A permutation that
changes a field value creates another record sharing that names array. In the
Go path, `RecordSet` constructs lexically sorted pairs, and Cartesian
enumeration calls `Function`, losing record field-order provenance. This
explains the observed order difference for records from this set; it does not
prove that all record presentation differences have that cause. In a separate
no-symmetry run without DOT, the instrumented enumerator likewise constructed
an `[op, adr]` record marked normalized and state fingerprinting visited it
through both containers.

Two isolated instrumented symmetry runs with `-fp 0`, one worker and DOT had
identical output bytes (SHA-256
`e2f233cfde224fe6b07ff9b2d0fa23814cbbbafb3c33e3e498e6b7318f5b9137`),
including ID `579606404721062638` and both record occurrences. Both completed
with 779 generated and 168 distinct states. The same counts were observed in
one symmetry run without DOT. No-symmetry runs with and without DOT each
generated 5,492 and found 1,272 distinct states; the DOT trace also showed
the same record object formatted through both containers. These runs used
ordinary TLC liveness timing,
not the JUnit final-only lifecycle; instrumentation and timing can affect when
an error terminates exploration. The object trace is observational, not proof
of output stability under other schedules.

No formatting change follows from this witness alone: the original direct
record/function probe still demonstrates that other construction paths can
change order during fingerprinting. The next experiment should retain token
order as immutable record-set provenance through enumeration, then compare
Java and Go snapshots before and after fingerprinting, ordinary `Print`, DOT,
VIEW and ALIAS, with literal records, functions and nested shared values as
controls. Verify enumeration order and state counts as well as labels before
considering such a change; do not normalize shared Go values in place.

The graph-only symmetry fingerprint microbenchmark on Apple M3, three 1 s
samples, measured 54.8–57.1 ns/op and 24 B/op before this correction versus
294–351 ns/op and 400 B/op afterward for a one-variable, two-model-value
symmetry. This deliberately isolates the added representative computation;
it is not an end-to-end throughput measurement and runs only when graph events
are requested.

## Liveness DOT investigation

Using tla-go revision `75220398a63252c9afc6ddc6edd756d97333b5de` and the
pinned upstream commit and jar SHA-256 above, the snapshot's NQTest,
OneBitMutexTest, SymmetryTableauModelCheckerTest and TwoPhaseCommitTest were
reproduced in separate copies of the tracked `test-model` tree. The sweep
uses respectively `symmetry/NQ/MC`, `symmetry/OneBitMutex/OneBitMutexMC`,
`symmetry/SymmetryLivenessTableauMC` and `symmetry/TwoPhaseCommit/MC`.
No PlusCal translation was requested: OneBitMutex's dependency contains a
PlusCal algorithm and its checked-in translation. The other model files and
their dependencies were also used as checked in. Each invocation used `-tool
-deadlock -fp 0 -workers 1 -checkpoint 0 -dump dot graph.dot`; Java also used
`-coverage 1` and the sweep's wrapper sets `TLCGlobals.livenessThreshold` to
`Double.MAX_VALUE`. Go coverage is omitted. JUnit itself uses a metadata dump
path and has additional setup and teardown; OneBitMutexTest is `@Ignore`.
These runs are partial diagnostics, not JUnit results.

Two Java runs per case produced identical DOT bytes in this environment; all
four Go dumps differed from Java, and two Go OneBitMutex runs also matched each
other byte for byte. Java and Go exited 13 for the first three and 0 for
TwoPhaseCommit. This small sample does not establish stable ordering under
different scheduling or environments. For instance, OneBitMutex prints the
same state label with Java node ID `-3946579319044950398` and Go ID
`-4001293080066318631`; NQ likewise differs in IDs for equal labels.
SymmetryTableau differs both in IDs and in the order of record fields, such as
Java's `[op |-> "Rd", adr |-> e]` versus Go's `[adr |-> e, op |-> "Rd"]`.
TwoPhaseCommit has differing IDs for equal labels as well. These observations
cannot be explained solely by reordering complete DOT lines.

All four configurations enable symmetry. In upstream `TLCStateMut.fingerPrint`,
Java chooses a symmetry representative for fingerprinting and, when the
representative differs from the original, calls `deepNormalize` on the
original values. That mutation is a possible contributor to the observed
representation and fingerprint differences; the experiments do not isolate it
as the sole cause. Changing representative comparison, normalization or shared
value mutation can affect exploration, liveness and output beyond DOT, so no
general fix is justified by these observations alone. The opt-in
`TestPinnedLivenessDumpOracle` checks selected Java node witnesses and common
initial nodes without asserting full graph order or requiring the mismatch to
persist. `TestCompareDumpDetectsByteDifferences` checks equal bytes and an
order-only negative case. Existing dump-format and liveness comparisons provide
additional controls.

## Investigation of snapshot 56336d54 PlusCal 2192 differences

The snapshot records 14 `pcal` classes with `message:2192`, not 13:
Euclid3, EvenOddBad, EvenOdd, Factorial2, InnerLabeledIf, MergeSort,
MultiProc2, NoLoop2, NoLoop, NoParams, ReallySimpleMultiProc,
SimpleLoopWithProcedure, ULEvenOdd, and ULFactorial2. EvenOddBad also has
`presence:2267`; these are class-level diagnostics, not JUnit verdicts.

The pinned source and jar verified above were used to translate the real
models with `pcal.trans -unixEOL -wf -termination`. For NoLoop2 the translated
`Spec` has `WF_vars(Next)`, `Termination` is `<>(pc = "Done")`, and `Next`
reaches action `e` containing `Print(x, TRUE)`; its `b` action chooses from
`{1, 2}`. EvenOdd, Factorial2 and Euclid3 likewise reach `PrintT` through
`Next`. These are effects in the translated TLA, not merely PlusCal comments.
The inherited JUnit setup translates first, checks its mapping and translator
messages, installs the recorder, sets the liveness threshold to
`Double.MAX_VALUE`, and runs with `-deadlock -fp 0 -coverage 1 -workers 1
-checkpoint 0 -dump dot` and a class-named metadir path. The differential uses
`-tool`, a separate dump path, and no Go coverage; it compares translated TLA
and CFG files. It does not execute that JUnit lifecycle on Go.

`TestPinnedEffectfulPlusCalTableauProgressGuard` compares six real translated
cases under that partial setup, including both Print and PrintT, a branching
model, and EvenOddBad. Java's 2192 counts are checked against the original
literal JUnit assertions (respectively 12, 13, 13, 16, 9, and 94); Go emits
neither 2192 nor 2267 in these cases. Translation and dump differences are
checked separately. The pure CCallReturn1 control emits the asserted count 8
on both sides. The comparison strips only the timestamp from 2192, so an
absent message is an application-output difference, not normalization of a
count. The selected snapshot reports 2267 only where the source selected its
presence; a missing snapshot entry does not establish that the other cases
emit it on Go.

Go deliberately suppresses this progress pair when tableau replay is unsafe.
Java's late tableau expansion can regenerate successors and evaluate effects and
constraints again; Go's completed state count alone cannot establish the same
fingerprint count, effect sequence, or subsequent behavior. This also applies
to dynamic TLCGet/TLCSet and must not be fixed by suppressing their effects or
fabricating the message. The branching in NoLoop2 is model nondeterminism;
it does not by itself prove run-to-run variability in the diagnostic. No
run-to-run nondeterminism is claimed for these counts. Intermediate versus
final liveness checks, recorder completeness, coverage and lifecycle equivalence
remain separate unresolved questions, so these observations do not establish
JUnit passes or failures for Go, or classify every one of the 14 cases.

A safe next step is an isolated Java/Go replay comparison that records typed
liveness graph insertions, successor and constraint evaluations, and ordered
Print/TLCGet/TLCSet effects, including dynamic values and branching, under both
final-only and intermediate checks. Implement Java-compatible late expansion
and its timing only after these observations and liveness outcomes agree; retain
negative tests showing that effectful replay cannot be replaced by a distinct
state count.

### Record-set ordering microprobes (2026-11-01)

`TestPinnedRecordOrderProbe` compiles `testdata/record_order/RecordOrderProbe.java`
against the verified 1.7.4 jar and runs each scenario three times in fresh JVMs.
It allocates `op` and `adr` in both orders and checks the actual token numbers,
uncached and fingerprint-cached enumeration, field-name-array sharing, and a
literal record before and after fingerprinting. In these isolated runs the
first two tokens are 1 and 2. When `op` is interned first, enumerated records
use `op, adr` even though the constructor receives `adr, op`, and the second
record changes `adr`; reversing intern allocation gives `adr, op` and the second
record changes `adr` in its first position. The direct literal initially keeps
its supplied `adr, op` order, then fingerprinting normalizes it to intern
order. The two enumerated records share the Java name array. The cached set
produces the same observed sequence as the uncached set in these cases.
These token numbers describe the minimal JVM probe, not a SANY model run.

`TestRecordOrderModel` runs isolated Go and, if configured, Java TLC processes
on a tiny model with `Print`, `ToString`, a nested tuple/function containing a
shared record, and equality against a literal with reversed fields. It checks
selected DOT labels and counts with no symmetry, reversed source order,
symmetry, and VIEW. Both implementations report 8 generated/4 distinct
without symmetry and 6/2 with symmetry. With `op` parsed first Java prints
`op, adr`, while Go prints `adr, op`; the reversed source control prints
`adr, op` on both. The Java model runs use `-fp 0`, one worker, private
metadata and DOT paths. The Java model comparison is opt-in; its assertions
are selected observations, not whole-output or JUnit parity. Java 1.7.4
rejects the `ALIAS` configuration keyword (see `checker/dfid_report.go`), so
there is no Java 1.7.4 ALIAS comparison here.

```sh
TLA_GO_JUNIT_ROOT=/tmp/tlaplus-java-v1.7.4 \
TLA_GO_JUNIT_JAR=/tmp/tla2tools-v1.7.4.jar \
go test ./value ./internal/upstreamjunit -run 'Test(PinnedRecordOrderProbe|RecordSetEnumerationCanonicalIdentity|RecordOrderModel)$' -count=1
```

No production behavior is changed by these probes. The current Go record-set
constructor canonicalizes fields lexically, and Cartesian enumeration visits
factors in that order. Preserving just a display-order list would leave the
observable enumeration and, potentially, exploration order different. A safe
change must carry check-local intern provenance through construction and
enumeration while retaining canonical membership, equality and fingerprint
semantics; it must also account for permutations that currently reconstruct
functions without their presentation provenance. Literal normalization and
shared nested values cannot be modeled by globally sorting every output.
The model samples do not establish complete token-allocation fidelity, all
cache paths, permutation presentation, ALIAS behavior in later Java releases,
or scheduling-independent output; those remain open before a general fix.

### Ignored tests in diagnostic inventory

Set `TLA_GO_JUNIT_DISCOVERY=1` together with the JUnit and Hamcrest paths above
when running `TestUpstreamJUnitSweep` to add `junit-inventory` and
`junit-ignore` records for each selected diagnostic candidate. The metadata
comes from the same verified runner descriptions, including inherited methods,
parameterized leaves and nested suites. It reports unannotated case descriptions,
confirmed ignored method leaves, ignored class descriptions, empty standard suites,
uncertain descriptions and runner errors separately, with
JUnit display names and `@Ignore` reasons. A candidate can have both runnable
and ignored leaves; an ignored class description does not provide a method
count. Ignore annotations under arbitrary runners, including inherited suite
annotations, are uncertain; an empty custom description is also uncertain.
Nested suites are classified conservatively because their child runner identity
is not available from a Description. A case description does not guarantee that
a custom runner executes it. These records describe the selected runner, not whether
a model run passes or whether every method in a custom runner is visible.

For example, the pinned `tlc2.tool.TLCGetNonDeterminismTest` has an ignored
`testSpec` with reason `No known fix/By design`. Its inherited custom runner
makes the discovery classification uncertain, although a separate JUnitCore run
observed it ignored. Its model remains a diagnostic
candidate and can still be run by the sweep. The optional metadata does not
change which diagnostic runs are attempted, their counts, or the interpretation
and schema of historical snapshots. A diagnostic comparison is not a JUnit
execution or a JUnit pass. `TestUpstreamJUnitDiscovery` verifies this example
against the pinned source and jars.
