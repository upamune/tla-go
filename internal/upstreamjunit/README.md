# Upstream JUnit differential sweep (incomplete)

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
2192 while Go omits the message. `safeLivenessDiagnostic` rejects the fairness
action `Next`, which reaches `lp` and its `PrintT`; even without that first
guard, `safeTableauProgress` rejects the effectful definition. Java's late tableau
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

On 2026-11-01, OpenJDK 21.0.11 discovered **322 executable leaf
cases** within the 340 lexical non-abstract candidate classes: 319 classes
had executable cases, and 21 had none. Twenty descriptions were ignored methods
and `tlc2.tool.suite.Test61` was ignored at class level. No runner errors were
reported. `tlc2.tool.Github432Test` accounts for four leaves in one class;
the other candidate classes each had one leaf or one class-level ignore. The
command prints the individual names, including all ignored cases. This is the
denominator only for these explicitly selected candidate classes, not for all
upstream tests or the upstream Ant suite: Ant selects source filenames and uses
multiple batches and exclusions, and the lexical inheritance scan can miss
classes. The 322 cases have not been mapped individually to Go runs or proven
equivalent; all remain unresolved for full differential purposes. Existing
class-based diagnostic runs and extracted observations are not JUnit executions,
and their counts cannot be substituted for this case denominator.
