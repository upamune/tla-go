# Fairness effects during model checking

Oracle: Java TLC 1.7.4, upstream commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` and jar SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23c45d5aa0ff20a8b96b23d01e050e88`.

Java checks action predicates on every distinct concrete successor as it adds
the liveness graph. `-lncheck final` defers the periodic graph search; it does
not defer these checks. For the fairness action `<A>_v`, Java's `LNAction`
evaluates the subscript on both states first and returns without evaluating A
if the transition stutters. An effectful `ENABLED` occurring inside an action
check is evaluated for each successor rather than once for the source state.

For a single non-tableau liveness solution without reductions, higher-order
operators, mutable TLC state or nondeterministic operators in its fairness
expression, the model-checking incremental path now evaluates that angle
action in Java's order and re-evaluates effectful fairness enabledness for each
successor. It retains one enabledness cache entry per state. Other models
retain the prior path.
`TestJavaTLCFairnessEffectOrder` covers the ordered PrintT stream
for deterministic and nondeterministic actions under weak fairness,
with both default and final strategies. The pinned Java comparison can be run
with `TLC_JAR` set to the jar above. Register-sensitive fairness and tableau
cases remain outside this scoped change.

This is a scoped improvement, not full liveness compatibility. The existing
implementation can still share cached values across liveness solutions: with
`x' \in {0, 1} /\ PrintT(<<"next", x, x'>>)` and property
`<>(x = 0) /\ <>(x = 1)`, Java reports two branches and prints 20 markers;
the Go checker reports two branches and prints 11, unchanged by this change.
It does not reproduce all
side effects when recovering a counterexample, when an
assertion fails during recovery, or when constructing some tableau graphs.
Temporal progress counts for effectful fairness remain suppressed. The `-lncheck`
argument is still consumed without selecting a periodic graph-check strategy;
the Go checker runs its final graph search after exploration. Simulation has
separate liveness processing and is not covered by this change.

Re-evaluation costs time and allocations. On an Apple M3 with 200 states and
all 200 successors per state, two single-iteration local benchmarks measured
75–77 ms and 65.6 MB before this change, versus 163–198 ms and 88.4 MB after
it. The samples ran with other work in progress and are illustrative, not a
precise estimate; the two versions execute different numbers of effects.
For the same graph without PrintT, three single-iteration runs measured
44.8–48.8 ms and 33.67 MB before versus 43.9–45.1 ms and 33.67 MB after;
the pure fairness path retains its previous cache behavior.
