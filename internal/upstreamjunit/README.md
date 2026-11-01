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
