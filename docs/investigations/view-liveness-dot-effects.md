# VIEW side effects in liveness DOT output

This records an unresolved compatibility gap, not a fix. The reference is Java
TLC 1.7.4 at upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`
(the `tla2tools.jar` SHA-256 is
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`).
`TestJavaTLCViewLivenessWriterOracle` checks the Java observations below when
`TLC_JAR` points to that jar; it does not assert that tla-go matches them.

For a single-worker breadth-first run, use `Init == x = 0 /\ TLCSet(0, 0)`,
`Next == x' \in 0..2`, `V == IF TLCSet(0, TLCGet(0) + 1) THEN x ELSE x`,
`C == TLCGet(0) < 8` and `P == []<>(x = 0)`, with `VIEW V`, `CONSTRAINT C`,
`PROPERTY P` and deadlock checking disabled. With `-tool -workers 1 -fp 0
-lncheck final -dump dot out.dot`, Java exits 0 after 4 generated and 1 distinct
state. At base `095d18c`, tla-go exits 13 after 7 generated and 2 distinct
states. Without the dump, both report a liveness violation with 10 generated
and 3 distinct states in this example, but their VIEW evaluation sequences
still differ. These results are observations of one model, not a general rule.

Java's `LiveCheck` creates a separate `DotLivenessStateWriter` for each
liveness solution when the state writer is DOT; that writer targets
`out_liveness.dot`. A non-tableau `LiveChecker.addInitState` calls the inherited
`DotStateWriter.writeState(state)`, which evaluates the fingerprint twice. For
the example above these are two extra calls to VIEW between the normal DOT
initial-state write and exploration of the initial state's successors. The
`-view` option also affects label rendering. `TableauLiveChecker.addInitState`
instead writes once per consistent initial tableau node, using
`DotLivenessStateWriter.writeState(state, tableauNode)`; node consistency and
label rendering can themselves evaluate expressions. Multiple solutions and
tableau nodes therefore cannot be represented by a fixed extra call count.

For each expanded state, Java `Worker.doNextCheckLiveness` first reevaluates
its fingerprint, inserts the stuttering state into a `SetOfStates`, writes the
stuttering edge, then calls `LiveCheck.addNextState`. The latter evaluates
checks and iterates stored successor states per liveness solution; its non-tableau
`LiveChecker` fingerprints each successor and writes it to the liveness DOT
writer. Tableau graph construction takes a different path. `SetOfStates`
stores concrete states with hash placement and can grow, at which point it
reevaluates stored fingerprints. The number, order, evaluation context and
errors of these operations matter when VIEW or liveness checks have effects.

At `095d18c`, tla-go neither emits the liveness DOT file nor reproduces all
these calls. Its `fingerprintViewSuccessors` tracks visited reduced targets
across source expansions; this is not equivalent to Java's per-expansion
`SetOfStates` of concrete successors. An isolated experiment adding only the
missing worker fingerprint changed existing evaluation-order tests and still
failed to match later Java calls, so that experiment was discarded. A correct
fix needs to model the liveness writer and successor collection together,
including `-view`, duplicate concrete states, tableau consistency, several
solutions, diagnostics, and runtime side effects; it must also verify the
additional file and both graph contents. A fixed-count workaround would not
establish that behavior.
