`upstream-checks.jsonl` records selected end-to-end results from tla-ts at
commit `19c763f07fda4b098e8a24405301628a424547c0`. They include complete
upstream Counter and Mutex models, operator-configuration cases, plus selected
upstream and derived examples.
Regenerate the file with `scripts/generate-checker-fixtures.ts` and a checkout
of that commit. The original files and derived cases are covered by
`UPSTREAM-LICENSE`.

`upstream-liveness.jsonl` compares 99 selected temporal cases, including the
unchanged Temporal fixture and upstream Liveness fixture with 15
configurations. Regenerate it with `scripts/generate-liveness-fixtures.ts`.
The comparison checks verdicts, property names, and state counts; valid
counterexample lassos need not match the exact path selected by tla-ts.
`upstream-fairness-matrix.jsonl` adds 120 deterministically generated graphs
with mixed weak and strong fairness, comparing verdicts and checking each
returned lasso. Regenerate it with `scripts/generate-fairness-matrix.ts`.

`upstream-general-ltl-matrix.jsonl` compares 1,536 cases with nested temporal
operators, state and recurrent action predicates, quantifiers, conditionals,
fairness, and identity views. Regenerate it with
`scripts/generate-general-ltl-matrix.ts`; returned lassos are also evaluated
independently against the configured temporal properties.

`upstream-fair-property-matrix.jsonl` adds 3,072 cases with fairness inside
properties, including quantifiers, local definitions, identity views, and
fairness assumptions. Regenerate it with
`scripts/generate-fair-property-matrix.ts`.
`upstream-fair-failure-matrix.jsonl` adds 1,920 cases, and
`upstream-spec-fair-failure-matrix.jsonl` adds 640 cases. These compare error
messages, traces, exit codes, queue sizes, and outdegree histograms, along with
the usual results and state counts. Regenerate them with the corresponding
`scripts/generate-*-matrix.ts` scripts.
`upstream-live-successor-matrix.jsonl` adds 268 cases that exercise TLC's
successor hash-set ordering, including growth beyond the initial capacity.
Regenerate it with `scripts/generate-live-successor-matrix.ts`.
`upstream-fair-level-matrix.jsonl` adds 1,152 fairness and `TLCGet("level")`
cases, checking failures, state counts, and the names reported for lassos.
Regenerate it with `scripts/generate-fair-level-matrix.ts`. The 320-case
`upstream-level-matrix.jsonl` also records names of violated properties.

`upstream-fair-register-matrix.jsonl` adds 1,152 fairness cases with a
register changed by `Next`. Results, state counts, and failure details match;
eight cases have different violated-property names because the selected
lassos differ. `upstream-lasso-replay-matrix.jsonl` records 1,152 cases with
printed `Init` and `Next` effects. Tests compare results, state counts and
failure details for all 1,152 cases, with the same eight lasso-name differences.
They also compare output for the 384 cases without fairness assumptions and
96 cases with weak fairness. Four cases without fairness have different output
due to different lasso paths. Other effectful fairness cases still have output
differences. Regenerate both with their corresponding scripts.
`upstream-lasso-special-matrix.jsonl` adds 384 cases for specialized temporal
properties with one or two weak fairness assumptions. Tests compare all
outcomes and state counts, and check printed output in 140 selected cases;
other cases still exhibit output differences. Regenerate it with
`scripts/generate-lasso-special-matrix.ts`.
`upstream-lasso-strong-matrix.jsonl` adds 384 cases with strong fairness;
`upstream-lasso-action-matrix.jsonl` adds 256 with recurrent action properties.
Tests compare all outcomes and state counts, and check output in 112 selected
action cases and 28 selected strong-fairness cases. The replay matrix also
checks 32 strong-fairness error outputs.
Strong-fairness and other effectful output cases still differ. Regenerate both
with the corresponding `scripts/generate-lasso-*-matrix.ts` scripts.

`upstream-guarded-matrix.jsonl` compares 120 finite graphs for the safety
property `[](P => []Q)`, and the tests independently validate each returned
counterexample. Regenerate it with `scripts/generate-guarded-matrix.ts`.
`upstream-guarded-fairness-matrix.jsonl` adds 120 graphs with weak and strong
fairness; each returned lasso is checked for both fairness and a finite safety
violation. Regenerate it with `scripts/generate-guarded-fairness-matrix.ts`.
`upstream-ordered-matrix.jsonl` compares 240 graphs for ordered eventuality,
half with weak and strong fairness. Its tests validate the reported lassos.
Regenerate it with `scripts/generate-ordered-matrix.ts`.
`upstream-sequence-matrix.jsonl` adds 240 graphs for four-stage ordered
eventualities; regenerate it with `scripts/generate-sequence-matrix.ts`.
`upstream-often-implication-matrix.jsonl` compares 480 graphs with state and
action antecedents and consequents, with and without fairness. Its tests also
check each returned lasso. Regenerate it with
`scripts/generate-often-implication-matrix.ts`.
`upstream-spec-recurrence-matrix.jsonl` compares 400 graphs with state or
action recurrence assumptions in the specification, including combinations
with weak and strong fairness. Regenerate it with
`scripts/generate-spec-recurrence-matrix.ts`.
`upstream-leads-staying-matrix.jsonl` compares 400 graphs for `P ~> []Q`,
including fairness and recurrence assumptions, and checks reported lassos.
Regenerate it with `scripts/generate-leads-staying-matrix.ts`.
`upstream-stays-or-often-matrix.jsonl` compares 480 graphs for a disjunction
of eventual stability and state or action recurrence, with fairness and
recurrence assumptions. Its lassos are independently checked. Regenerate it
with `scripts/generate-stays-or-often-matrix.ts`.

`upstream-modules.jsonl` compares 71 models, including external `EXTENDS`,
transitive and shared dependencies, nested and parameterized instances,
operator and lambda substitutions, name capture, imported state and local
definitions, and unchanged upstream O, Counters, NestedModules, Instances, Operators,
Capture, Binders, ParameterizedSpec, and scoped override fixtures. For the parameterized
specification violation, the trace states are compared but internal action
labels differ. Regenerate it with `scripts/generate-module-fixtures.ts`.

`tlc-parameterized-function-action` records a separate comparison against TLC
2.19 (tla2tools 1.7.4). The pinned tla-ts reports an unassigned-variable error
for this model, while TLC explores it. Run
`scripts/verify-parameterized-function-action.sh /path/to/tla2tools.jar` to
recheck TLC's counts and counterexample.

These cases cover only the checker's current subset. They do not cover all
upstream unit tests, general liveness, module instances, proof
obligations, TLC output formatting, or every value and action form.

`bags/BagsTest`, `bags/test36`, and `bags/test50` and their configurations are unchanged upstream fixtures.
`upstream-bags-checks.jsonl` compares five state-space models using bag
operations; regenerate it with `scripts/generate-bags-checks.ts`.

`upstream-printing.jsonl` records output order and state counts for assumptions,
initial states, actions, invariants, and higher-order calls. Regenerate it with
`scripts/generate-printing-fixtures.ts`.

`upstream-pretty.jsonl` compares nested and long-value output against tla-ts.
Regenerate it with `scripts/generate-pretty-fixtures.ts`.

`upstream-any.jsonl` records 28 directed comparisons with TLC's `Any` set.
Regenerate it with `scripts/generate-any-fixtures.ts`.

`tlc-statistics`, `tlc-action`, `runtime`, `tlc-set`, `tlc-live-level`, and
`postconditions` contain unchanged upstream fixtures. `upstream-level-matrix.jsonl`
compares 320 generated graphs with TLC level expressions in temporal properties;
regenerate it with `scripts/generate-level-matrix.ts`. Tests also verify lasso
transitions and fairness. Counterexamples can differ from TLC's chosen path.

`possible/` and the postcondition configurations are unchanged upstream fixtures.
`upstream-possible-matrix.jsonl` compares witness counts, evaluation order,
violations, and state counts across 240 generated graphs; regenerate it with
`scripts/generate-possible-matrix.ts`.

`tlc-spec/` contains the five unchanged upstream `TLCGet("spec")` models and
their imported modules. `upstream-spec-description-matrix.jsonl` compares 132
generated specifications, parameterized aliases, and properties with the pinned
upstream; regenerate it with `scripts/generate-spec-description-matrix.ts`.

`tlc-ext/` contains unchanged upstream `Defer` and `AssertEq` models.
`upstream-tlc-ext-matrix.jsonl` compares output and search statistics across 48
generated models using TLCExt registers, assertions, and deferred expressions;
regenerate it with `scripts/generate-tlc-ext-matrix.ts`.
`tlc-constraint-order/` contains the unchanged upstream model and expected
output for constraint, invariant, and property evaluation order.

The unchanged `TLCExtTrace`, `TraceNext`, and `TraceAssume` fixtures exercise
`Trace` during exploration and an incomplete state.
`upstream-tlc-trace-matrix.jsonl` compares 36 branching models, including
constraint and property evaluation of `Trace` and `Trace'`; regenerate it
with `scripts/generate-tlc-trace-matrix.ts`.

`upstream-tlc-trace-liveness.jsonl` checks 24 temporal models using `Trace`;
regenerate it with `scripts/generate-tlc-trace-liveness.ts`. The unchanged
`CounterExample` model and five configs test postconditions after violations.
`upstream-counter-example-matrix.jsonl` checks 24 more models including full
state and action records; regenerate it with
`scripts/generate-counter-example-matrix.ts`.

`alias/` contains unchanged upstream safety, lasso, and stuttering models.
`upstream-alias-matrix.jsonl` checks 72 models against upstream, including
invalid aliases, postcondition output, and record field order; regenerate it
with `scripts/generate-alias-matrix.ts`.

`view/` contains unchanged upstream `View` and `Recover` fixtures.
`upstream-view-matrix.jsonl` compares 96 reduced state spaces, fairness
conditions, and temporal properties; regenerate it with
`scripts/generate-view-matrix.ts`. The upstream symmetry-plus-view fixtures are covered by `TestUpstreamSymmetryFixtures`.
`upstream-state-view-reduced-matrix.jsonl`, `upstream-state-view-multi-matrix.jsonl`
and `upstream-state-view-y-matrix.jsonl` compare 1,184 general and specialized
state-formula cases with identity and merging views, including predicates on
variables hidden by the view. The symmetry-general matrix adds 144 cases with
symmetry and optional views. Tests compare results, state counts and violated
property names; the chosen lasso can differ. Regenerate these with the
corresponding `scripts/generate-*-matrix.ts` scripts.

`symmetry/` and the `ViewSymmetry` fixtures are unchanged upstream models.
`upstream-symmetry-matrix.jsonl` compares 156 cases, including configured
model-value order and TLC variable order. Regenerate it with
`scripts/generate-symmetry-matrix.ts`. That script works around Bun stripping
calls to a local `declare` function in a temporary copy of the pinned source,
which otherwise changes TLC variable ordering compared with upstream's Node
test runner. `scripts/generate-tlc-order-data.ts` similarly regenerates the
standard-module symbol metadata used by the checker.

`strings/` includes the unchanged upstream string-order and overridden-set
fixtures. `upstream-string-order-matrix.jsonl` compares 96 models involving
source-token order, configuration values, and runtime-generated strings;
regenerate it with `scripts/generate-string-order-matrix.ts`. The imported
module and preinterned string tests also compare their expected output with the
pinned upstream implementation. Regenerate the startup, SANY, and runtime
string tables with `scripts/generate-tlc-string-order-data.ts`.
`upstream-standard-string-order-matrix.jsonl` checks 196 combinations of
standard and external modules with configuration values; regenerate it with
`scripts/generate-standard-string-order-matrix.ts`. The standard module token
and dependency data is regenerated with
`scripts/generate-tlc-standard-source-data.ts`.
`upstream-nested-string-order-matrix.jsonl` compares 72 cases with nested
modules and shared external dependencies; regenerate it with
`scripts/generate-nested-string-order-matrix.ts`.

`json/` contains the unchanged upstream `JsonTests` and `JsonValues` models,
configurations, and input files. The checker test supplies an in-memory write
callback and verifies the final output against the upstream test.

`randomization/` contains unchanged upstream random-choice fixtures. Tests
check Java-compatible random streams, fixed-seed traces and state counts,
model-value fingerprints, large-product sampling, and constant definitions.
`upstream-large-interval-random.jsonl` records tla-ts seeded subset output for
sizes 2^31−1, 2^31, and 2^32; regenerate it with
`scripts/generate-large-interval-random.ts`. Only the 2^31−1 cases are used
as Go expectations: Java TLC rejects the larger intervals.

`java-tlc/` contains 25 unchanged model files, including dependencies,
and 29 configurations from Java TLC v1.7.4, commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba`, with its MIT
license. The Go tests check the status, state counts and search depth and can
also run the same models against the official Java JAR. Counts come from the
corresponding JUnit tests; depths are also checked against the Java JAR.
The `SystemLoop`, `BidirectionalTransitions`, and `codeplex` tests additionally
compare counterexamples and fairness results. The external-module tests also
compare action source locations. The `one-bit` test checks the status, counts,
variable order, and action locations, but its counterexample differs from Java
TLC. The CLI tests also compare the incomplete-successor errors and traces from
`IncompleteNextTest` and `IncompleteNextMultipleActionsTest`, plus fingerprint
errors from `EmptySubsetEqTest`, `FingerprintExceptionInitTest` and
`FingerprintExceptionNextTest`. `TLCGetLevelTest` also checks the full trace and
stuttering loop. The tests do not yet check JUnit's coverage or on-disk graph assertions,
or the rest of the suite.

`let-instances/` contains unchanged upstream `LET` instance models and their
dependencies. `upstream-let-instance-matrix.jsonl` compares 96 generated
models, including parameterized instances, action substitutions, and exact
violation traces; regenerate it with `scripts/generate-let-instance-matrix.ts`.

`subexpressions/` contains the unchanged upstream subexpression-reference
fixtures. Tests cover the original success, invariant-violation and modified
assumption-violation cases. `upstream-subexpression-matrix.jsonl` adds 34
operand, binder, label, LET and operator-argument cases, including rejected
selectors; regenerate it with `scripts/generate-subexpression-matrix.ts`.
