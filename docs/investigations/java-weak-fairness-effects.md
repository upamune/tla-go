# Java TLC weak-fairness effects investigation

The initial observations below record the baseline investigation on 2026-11-01.
A later, limited model-checking change is described in [Fairness effects during
model checking](model-fairness-graph-insertion.md). That change does not cover
the tableau, register-sensitive or trace-recovery cases discussed here.
Simulation has a separate liveness implementation.

## Current simulation status

The simulation tool streaming experiment uses the existing graph and reporting
path for models containing ALIAS, or a VIEW with effects, runtime TLCGet reads,
unstable reads, or reachable higher-order operators or operator constants. For other
models, the opt-in path
reevaluates graph leaves at each graph operation and streams prefix and cycle
states at recovery boundaries. After recovery fails it emits the error and 2121,
then recovers sampled actions before printing each nonstuttering state; even
suppressed stutters evaluate their actions. Java oracle comparisons cover initial
and cycle recovery failures, sampled stutters, and a separate division error
during sampled-behavior recovery. Selected weak and strong fairness assumptions,
multiple fairness conditions, an effectful fairness subscript, and fairness in a
property also match the pinned oracle for their selected tool messages. These
examples do not establish equivalence for general fairness formulas or failures.
Effectful and runtime-dependent VIEW remain known mismatches: state fingerprints are evaluated eagerly
before liveness checks and printed VIEW evaluations are grouped differently from
Java. Set `TLA_GO_KNOWN_VIEW_EFFECTS=1` with `TLC_JAR` and run
`TestJavaTLCSimulationToolEffects/view-effects`, `view-level`, or `view-higher-order`
to reproduce them. Other recovery
errors and presentation callbacks also need further verification. The historical
observations below describe earlier revisions.

When an eligible simulation's counterexample recovery fails, Java's subsequent
behavior printer first recovers each action and then compares the current and
previous states' `TLCStateMut.fingerPrint()` values. The comparison applies
symmetry and VIEW and can suppress distinct states with identical FP64 values.
When printing a `-difftrace` state, Java compares it with the immediately
previous sampled state, even if that predecessor was suppressed.
`TestJavaTLCSimulationBehaviorReduction` compares ordered user output and tool
messages with the pinned JAR for selected VIEW, SYMMETRY, combined, and unreduced
model-value cases. This does not establish parity for effectful VIEW, unknown
model-value ordering, or all fingerprint and recovery errors.

For simulation liveness graphs, a narrower pure VIEW case now uses Java FP64
keys in both the default Go API and CLI, including normal and `-tool` output.
It applies when VIEW is one variable or a flat tuple of variables, every
sampled state variable is an integer or boolean, and ALIAS is absent. This
includes SYMMETRY: its model-value permutations leave these sampled states
unchanged. Recovery candidates outside this scalar subset use the full Java
fingerprint calculation, including symmetry. Other VIEW expressions, non-scalar
sampled states, ALIAS, and specifications without VIEW retain the old graph
path. In particular, this does not fix effectful or failing VIEW evaluation.
Pinned-oracle recovery tests sample scalar states and then enumerate a
model-valued or tuple-valued VIEW candidate before the matching scalar initial
state, with and without SYMMETRY. They also compare an error while generating a
recovery candidate. These tests do not cover every fingerprint error or model
value ordering.
Eligibility scans all variables in every sampled state, and each graph solution
scans the variables again when fingerprinting those states. Thus a scalar VIEW
of one variable can still add CPU work for a wide state. The benchmark
`BenchmarkSimulationScalarViewWideGraph` covers 301 variables and depth 1000;
timing should be measured without competing workloads before drawing a
performance conclusion.

This changes verdicts for actual FP64 collisions, as in Java. Under `-fp 0`,
`<<0,0,0>>` and `<<-1130370423,1673453457,0>>` both have fingerprint
`b89a76409771b44d`: a regression model succeeds in Java and in the new path,
where the previous Go graph found a liveness violation. Under `-fp 1`, both
implementations find the violation. The Java comparison covers normal and tool
output, and selected symmetry cases; the default Go API is tested separately.
There are still known Java FP64 collision verdict differences without VIEW.
Tableau product-node keys have an additional collision gap even within the
limited VIEW case: Java's `LiveCheck1` uses `FP64.Extend(stateFP, tableauIndex)`
as a single key, while Go keeps the state fingerprint and tableau index as a
pair. Two different pairs that collide after Java's extension can therefore be
merged by Java but remain distinct in Go. The selected tests do not establish
tableau collision parity; no concrete verdict divergence has been demonstrated
for this case.

Oracle: tla2tools v1.7.4, upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`, JAR SHA-256 `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`. `TestJavaTLCFairnessEffectsLifecycle` exercises the isolated models; set `TLC_JAR` to the pinned JAR to run the Java comparisons. Each Java invocation uses one worker, fingerprint polynomial 0, and its own temporary model and metadata directory. Plain and `-tool` were compared. The original was also run three times with a fresh metadir per run and one worker: Java printed 10 times and Go 7 times on all three runs, with 3 generated and 2 distinct states and a liveness violation. This is not a determinism claim for multiple workers, arbitrary fingerprint polynomials, periodic checks, or other models.

In additional single-worker runs with fingerprint polynomial indices 0 through 5, the labeled model produced the same ten values in the same order. This sample does not establish a general ordering guarantee.

For `Next == /\ x' = (x+1)%2 /\ PrintT(<<x,x'>>)` and `WF_x(Next)`, an isolated copy of the pinned Java `tlc2.module.TLC` class was instrumented to print the Java stack at `PrintT`. With one worker and `-fp 0`, the ten calls were:

| Position | Value | Java caller / purpose |
| --- | --- | --- |
| 1 | `<<0, 1>>` | `Worker.run`: generate successors |
| 2 | `<<0, 1>>` | `LNStateEnabled.eval`: fairness check for the real successor |
| 3 | `<<0, 1>>` | `LNAction.eval`: fairness action for the real successor |
| 4 | `<<0, 1>>` | `LNStateEnabled.eval`: fairness check for the stuttering successor |
| 5 | `<<1, 0>>` | `Worker.run`: generate successors |
| 6 | `<<1, 0>>` | `LNStateEnabled.eval`: fairness check for the real successor |
| 7 | `<<1, 0>>` | `LNAction.eval`: fairness action for the real successor |
| 8 | `<<1, 0>>` | `LNStateEnabled.eval`: fairness check for the stuttering successor |
| 9 | `<<0, 1>>` | `LiveWorker.printTrace`: recover the next trace state |
| 10 | `<<1, 0>>` | `LiveWorker.printTrace`: recover the closing action |

The instrumented class was compiled from the pinned source against the pinned JAR and placed before it on the classpath; the JAR itself was not modified. `Worker.doNextCheckLiveness` adds stuttering to the successor set, and `LiveCheck.addNextState` evaluates each solution's action checks on every concrete successor before state checks. In this example the check includes `~ENABLED <<Next>>_x \/ <<Next>>_x`; both successor checks evaluate `ENABLED`, while only the real transition evaluates the angle action. Set iteration, other graphs, and short-circuiting can change the order and number of evaluations. Trace reconstruction can run concurrently with prefix reconstruction in Java; the position table is evidence for this model and run, not a universal schedule.

At that baseline, Go's `evaluateIncrementalAtom` cached state atoms by source state across successor choices. That suppressed one effectful `ENABLED` evaluation per source state in this example. On this violation its `replayLassoPath` also did not evaluate the closing transition for the printed loop label. A separate control with fairness action independent of `Next` still shows the missing closing transition; removing fairness entirely selects a stuttering counterexample with no closing `Next` replay. A disabled fairness action and the satisfied property show why neither the existence of fairness nor a violation alone predicts replay. State-valued `PrintT(x)` and `Print` reproduce the same lifecycle. State and action constraints rejecting the only non-stuttering successor both produce two prints and success in both implementations. A `TLCSet`/`TLCGet` counter reaches 8 before Java's trace reconstruction; replay of `Init` resets it and the two trace evaluations print 1 and 2. The baseline Go counter reached 6 before reconstruction. Thus these are observable runtime changes, not just extra output.

No production fix was included at the time of this investigation. The later change covers only the cases specified in the linked model-checking note; it does not establish general fairness compatibility. Full support requires Java's per-solution, per-concrete-successor check execution, including short-circuiting and the ordering of effectful state subexpressions, while retaining the actual check results for SCC analysis. Separately, model trace recovery of the closing action using Java's actual successor generation and failure behavior, including nondeterministic candidates and reductions. Differential cases are also needed where register writes change fairness or successor selection, as well as for failed recovery, multiple successors and constraints, and error/termination behavior. Avoid simply invalidating all caches or adding unconditional replay: either can change reachability, verdicts, registers, errors, or termination. Existing tests deliberately assert Java's oracle and matching controls but log the known Go output divergence; they do not claim general parity.

Additional pinned-oracle cases in `TestJavaTLCFairnessRegisterSensitiveOracle` put a counter update and a counter-dependent guard or assertion inside `Next`. The guard makes Java reject the later successor after fairness checks have advanced the counter; the assertion fails during those checks. These characterize the Java sequence and state counts in fresh directories in plain and tool mode, repeated with one worker. The ordinary lifecycle test also includes pure satisfied and violated controls and a fixed register read. These do not establish safe eligibility for replay: even a print-only action can contain a failing or nondeterministic expression, and output in the fairness action can call definitions with register effects. A safe implementation needs a conservative, transitive effect/totality and determinism classification, Java-compatible per-solution successor checks with separately stored results, and trace recovery that actually generates and selects the closing successor and propagates failures. Until those pieces are available, there is no general basis for turning on an extra evaluation solely from the presence of `Print` or a fairness condition.

## Earlier scoped LiveCheck1 change and remaining model-checking gap

Simulation's `LiveCheck1.constructBEGraph` evaluates each check at the points
where it builds the sampled behavior graph. It reuses those check results for
product nodes and edges as appropriate, but does not memoize individual leaves
across operations or solutions. The Go `liveTrace` leaf cache incorrectly
suppressed repeated evaluations, including `ENABLED` nested in an action
check; it could also reuse an old truth value or hide a later error. Disabling
that cache for one configured property was an earlier experiment; multiple
configured properties then retained the previous cache. The current opt-in
gating and selected multiple-property tests are described above.
`TestSimulationLivenessFairnessEffects` compares the print sequence and state
count of a deterministic weak-fairness simulation to the pinned JAR. The unit
regression also verifies a later error is propagated. This increment does not
implement model-checking `LiveCheck` graph construction or late expansion.

The earlier model-checking investigation found remaining gaps in the
weak-violation, register, guard and assertion models above. The later limited
change handles selected effectful fairness checks; other cases can still cache
`ENABLED` across concrete successors or omit a closing transition. Late tableau insertion must
regenerate successors and apply constraints at insertion time, not after the
entire state search. Outside the limited pure VIEW case described above,
simulation computes all trace VIEW keys before any solution check, whereas
Java computes fingerprints as it constructs each solution; effectful or failing
VIEWs can therefore have different order or results. TLCGet in simulation
liveness predicates remains unsupported. Do not
infer general fairness, error or counterexample fidelity from the new case.

On an Apple M3, two short runs of `BenchmarkSimulationFairness` before this
change used about 10.83 MB and 133,076 allocations per operation; after it,
about 12.17 MB and 157,500 allocations. Elapsed samples were noisy (18.9–24.3
ms before, 25.7–26.3 ms after). Both reported 4,812 generated states per
operation. The added evaluation is observable and required here; no claim is
made that this establishes performance for other models.

## Simulation violation names and graph evidence

`TestSimulationLivenessViolationEffectsOracle` compares the pinned JAR on
single and multiple properties, with and without a tableau. Java reports a
violation without evaluating `Q` during name reporting. The opt-in now skips
that evaluation and returns no names; this test compares the resulting effects
with Java. Java itself prints no names. This does not establish complete tool
output compatibility. `TestJavaTLCSimulationToolEffects/fairness-cycle`
originally exposed extra graph evaluation effects and a recovery ordering
difference. The eligible streaming path now compares the selected tool messages
for this case against Java; the other limitations above remain.

Provenance alone cannot recover all names from the current graph. DNF merges
`[]<>` disjuncts into a single `[]<>(P \/ Q)` check; short-circuiting can
leave the value of `Q` unknown. A successful PEM therefore establishes the
merged disjunction, not which source properties it satisfies. Only the first
accepted solution and PEM are explored, so another violated property might
have checks or a tableau in an unbuilt solution. Even a proof for that PEM
concerns its product-graph walk: plain graph nodes can store state checks made
on the preceding trace state, and recovery can choose a different concrete
state for a reduced fingerprint. The returned lasso can thus differ from the
behavior for which those checks were recorded. Tableau consistency checks may
also repeat effectful evaluations, and are not stored as an expression-by-state
truth table. Keying results by expression and state or edge would conflate
different evaluations and change short-circuiting, errors and effects.

The existing fallback returns every property when replay fails or finds no
violation; it is preserved for compatibility, including its known potential
to name a satisfied property. Directly assigning the sole configured name
would avoid replay effects, and splitting or quantifier expansion retains that
name. However, the API documents names as properties violated by the reported
lasso. Graph checks can describe a different behavior from the recovered
lasso, particularly with VIEW or symmetry; effectful predicates also do not
have a stable truth value across evaluations. A graph violation therefore does
not prove the documented claim about the returned lasso. No direct-assignment
fix is included until that contract is resolved or the correspondence proven.
The opt-in tool path omits lasso property names instead of claiming a name
without replay. Theorem and finite-trace cases need independent verification
before changing the default naming contract.
