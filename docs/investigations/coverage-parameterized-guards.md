# Parameterized action guard coverage

Go still rejects parameterized action guards in coverage mode. The seven
`guard-*.coverage` fixtures and `TestJavaTLCCoverageParameterizedGuardBoundary`
record Java TLC 1.7.4 CLI output for false and mixed guards, repeated calls to
one action, repeated formal parameters, multiline declarations, an evaluation
error, a state constraint, and two substituted instances. They compare both
normal and `-tool` reports using separate metadata directories. These are CLI
comparisons, not execution of the upstream JUnit lifecycle. For successful
models the tests also check Go without coverage; the coverage request must
still be rejected without publishing an incomplete report.

The oracle is the pinned `tla2tools.jar` 1.7.4, SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`,
upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`. The JAR hash is
checked by the tests. The CLI uses `-deadlock -fp 0 -workers 1 -checkpoint 0
-coverage 9999`.

The ordinary reports alone show why a per-action counter or a formula based on
state counts is insufficient. For `A(n) == x<n /\ x'=n`:

* When `A(0)` is always false, Java reports the guard at 2 and omits its two
  children, while the primed assignment remains visible at 0.
* With `A(1) \/ A(2)` sharing one action root, the guard is reported at 3,
  both children at 6, and the assignment at 3.
* For `A(n) == n=n /\ x'=n` with those same calls, the guard is reported at
  12, both formal occurrences at 6, and the assignment at 6.

To distinguish reported values from underlying counters, a separate local
diagnostic compiled the pinned Java `CostModelNode.java` against the pinned JAR
with `System.err` logging immediately before the four primary and secondary
increment methods. The compiled class was placed first on the Java classpath;
the JAR was not modified. The normal coverage sections of the three diagnostic
runs exactly matched the saved uninstrumented reports. For the false model,
the two comparison children each received two primary increments and the
guard received none: report collapse displayed their count at the parent. In
the shared model the guard received three increments and each child six; in
the repeated-formal model the guard received twelve and each child six. This
single-worker diagnostic does not establish general concurrency or evaluation
ordering. It is evidence about these cases, not a replacement Java oracle.

The failing `A(TRUE)` guard produces Java messages 2266, 2121 and 2103, and
Java still reports a partial coverage section. In the saved observation, the
guard is reported at 2 after failure/replay. Counters cannot be synthesized
from successful successors alone. The constrained and instance examples show
additional independent roots, source locations and filtered states; their
reports are recorded without claiming Go coverage support.

## Why the existing hooks are not a general implementation

[`coverageParameterizedRoot`](../../checker/coverage_parameterized.go) binds
call occurrences to one shared root but currently represents only one
cost-bearing expression. [`generator.cover`](../../checker/coverage.go)
increments that expression for each atomic work item; allowing a conjunction
without changing this would merge guard and assignment events.
[`generator.guard`](../../checker/solve.go) evaluates the guard before it knows
whether the continuation emits a state, while
[`coverGuardSuccess`](../../checker/coverage.go) updates existing nullary
action counts only after that continuation. The root, guard, operand and
assignment updates therefore need separate nodes and event points.

[`substituteActionCall`](../../checker/call.go) replaces formal expressions
with caller expressions. In `n=n` both formal positions can become the same
caller pointer even though Java reports two distinct formal source locations.
The existing `OnEvaluate` observer in
[`evaluator.Context`](../../evaluator/evaluator.go) sees expression entry but
not a child edge or a matching exit; it cannot generally reconstruct which
formal position was evaluated, especially through lazy arguments and
short-circuiting. Matching on source range or call name alone also fails for
the saved shared-root and instance cases. The existing property coverage tree
in [`coverage_property.go`](../../checker/coverage_property.go) does not
model all Java collapse rules or retain zero-count primed nodes, so simply
reusing it would misreport the false and error cases.

## Safe integration boundary

1. Record source origin **and parent occurrence edge** through action
   substitution, alpha-renaming and instance linking; do not mutate a
   caller-owned AST or change its JSON projection. Preserve distinct formal
   positions even when both use one argument pointer. Reject a rewrite whose
   provenance is unavailable rather than falling back to a range match.
2. Carry an optional per-check cost cursor through work items, continuations
   and actual evaluator calls. Event instrumentation needs balanced scope or
   explicit child edges, and must follow argument forcing/caching rather than
   re-evaluate expressions. Keep it disabled on the ordinary path and measure
   the disabled and enabled paths separately.
3. Build an ordered root/child graph with primary, secondary and primed-node
   state. Test Java's general collapse/suppression and independent snapshots
   against the saved models before wiring it to action generation. A periodic
   observer must take a consistent snapshot without holding a lock while
   invoking evaluation or callbacks.
4. First enable only proven pure, total guard operations, while continuing to
   reject higher-order operators, substituted instances, constraints, errors,
   and other unverified constructs. Add tests for both normal and tool output,
   repeated roots, repeated formal uses, default mode and race. Then expand
   support only after the error/replay and cross-module oracles match.

No production event/tree implementation is included in this investigation.
The existing direct-assignment and UNCHANGED parameterized subset is unchanged;
`ACoverageTest` as a whole remains unsupported.
