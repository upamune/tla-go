# Java TLC test inventory

The compatibility target is the original TLA+ tools, including Java TLC.
The snapshot below is a starting point for an audit, not a pass-rate estimate.
It is based on TLA+ Tools 1.7.4, commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba`, and tla-go commit `a5d5155`.

## Scope of the snapshot

Counting lines containing `@Test` in the upstream `tlatools/org.lamport.tlatools/test`
Java sources yields 730 occurrences in 331 files. Of those, 605 occur under
`tlc2/`:

| Upstream test area | `@Test` occurrences |
| --- | ---: |
| `tlc2/tool/` | 418 |
| `tlc2/value/` | 74 |
| `tlc2/util/` | 69 |
| `tlc2/module/` | 21 |
| `tlc2/output/` | 11 |
| `tlc2/model/` | 3 |
| Java files directly under `tlc2/` | 9 |
| **Total `tlc2/`** | **605** |

The remaining 125 occurrences in `test/` are under `pcal/` (105),
`tla2sany/` (5), and `util/` (15). Three additional source trees contain
18 occurrences in seven files: `test-concurrent/` (9), `test-long/` (7),
and `test-verify/` (2). Together these four source trees have 748 occurrences
in 338 files. The snapshot is an annotation count: ignored tests, comments,
parameterized cases, inherited tests, and tests that do not use this annotation
can make the number of executable cases different. It includes Java-internal
unit tests as well as end-to-end TLC models.

At `a5d5155`, tla-go contains 187 Go test functions named `TestJavaTLC...`
under `checker/` and `cmd/tla-go/`. One function can exercise many models or
cases, so this number cannot be compared directly with 605. The current CI
runs these Go comparisons with the 1.7.4 JAR; it does **not** run every upstream
JUnit test against tla-go. No complete mapping of upstream test cases to
passing, failing, or unimplemented tla-go tests exists yet.

To reproduce these counts from the upstream checkout and this repository:

```sh
rg -n '@Test\b' "$TLA_TOOLS_SOURCE/tlatools/org.lamport.tlatools/test/tlc2" -g '*.java' | wc -l
rg -l '@Test\b' "$TLA_TOOLS_SOURCE/tlatools/org.lamport.tlatools/test" -g '*.java' | wc -l
rg -n '^func TestJavaTLC' checker cmd/tla-go -g '*_test.go' | wc -l
```

## First mapped model group

`checker.TestJavaTLCOriginalModels` compares 12 models from upstream JUnit
fixtures against Java TLC 1.7.4. It checks success, generated and distinct state
counts, and graph depth. It does not check the original coverage assertions,
so this is partial behavioral coverage for those models, not 12 fully passing
upstream JUnit tests. The models are `AssignmentInit`, `MinimumDiameter`,
`SubsetEq`, `MinimalSetOfNextStates`, `AssignmentInitNeg`,
`AssignmentInitExpensive`, `AssignmentNext`, `AssignmentNext2`,
`AssignmentNext3`, `MinimalSetOfInitStates`, `DieHardTLA`, and
`CodePlexBug21`.

The upstream `BasicMultiTrace` simulation fixture is also exercised by
`TestJavaTLCSimulationUpstreamModels` and `TestJavaTLCSimulationUpstreamTool`.
These tests compare selected successful runs, invariants, action properties,
constraints, incomplete states, evaluation failures, and the `MCLivenessProp`
eventuality with Java TLC 1.7.4.
They check exit codes, generated-state counts when reported, and selected tool
messages and traces; they do not establish equivalence for all output or for
the remaining original liveness and `-continue` tests. The separate
`TestJavaTLCSimulationContinue` and `TestJavaTLCSimulationContinueUpstream`
compare selected continuation cases against Java TLC, including five original
`BasicMultiTrace` configurations and invariant, action, deadlock, evaluation,
and liveness behavior. They do not execute the JUnit methods themselves.

The simulation recurrence tests also compare small models for `[]<>P` and
`<>[]P`, including branching traces and combinations of temporal properties,
against Java TLC 1.7.4. They cover selected results and tool traces, not all
temporal formulas. A separate regression test records a known difference in
simulation stability results between TLC 1.7.4 and 1.8.0.

`TestJavaTLCSimulationTraceFiles` compares generated trace files for normal
traces, zero depth, state constraints, and invariant violations with Java TLC
1.7.4. A separate comparison records selected format and length differences in
Java TLC 1.8.0. This does not cover all value formats or side effects.

`TestJavaTLCSimulationLeadsTo` and related tests compare one leads-to property
over selected pure state predicates with Java TLC 1.7.4, including branching
traces, varying depths, constraints, constant predicates, and a bounded
quantifier. They check exit codes and selected tool messages and traces, not
all liveness formulas or error handling. A separate test records a result
difference with Java TLC 1.8.0.

`TestJavaTLCSimulationAril` compares selected seeded traces, including negative
and large arils, against Java TLC 1.7.4. A separate test checks selected seeds
against 1.8.0. These comparisons cover selected action and liveness behavior,
not the full simulation test suite.

`TestJavaTLCModernSimulationConfig` and `TestJavaTLCModernSimulationConfigRecord`
compare selected simulation configuration values and record output with Java TLC
1.8.0. `TestJavaTLCSimulationConfigVersionDifference` records that Java TLC
1.7.4 leaves this configuration undefined. The comparisons exclude the `install`
path, which is specific to the Java JAR.

`TestJavaTLCModernSimulationExit` compares selected stop requests in assumptions,
initial states, actions, invariants, and deadlocks with Java TLC 1.8.0.
`TestJavaTLCModernSimulationExitConstraint` checks a stop from a state constraint.
`TestJavaTLCModernSimulationExitLiveness` checks one stopped eventuality trace.
`TestJavaTLCModernSimulationExitEvaluationError` exercises an evaluation error
after a stop request; `TestJavaTLCSimulationExitVersionDifference` records the
exception in Java TLC 1.7.4. These cases do not cover every interaction with
simulation errors or temporal properties.

`TestJavaTLCConstantPredicates` and `TestJavaTLCConstantPredicateOrder` compare
selected constant invariants, properties, and state and action constraints
with Java TLC 1.7.4 and 1.8.0. They cover `TRUE`, `FALSE`, evaluation order,
unused constant definitions, and `PrintT` side effects.
`TestJavaTLCConstantAssignmentPrecomputation` checks the ordering before a
configured constant is rejected.
`TestJavaTLCModernConstantPostcondition` checks the cached value of a constant
postcondition with Java TLC 1.8.0. Non-boolean expressions and all evaluation
errors are not covered by these comparisons.

`TestJavaTLCNonBooleanInvariant`, `TestJavaTLCNonBooleanSuccessor`,
`TestJavaTLCNonBooleanJunctionRanges`, `TestJavaTLCNonBooleanProperty`, and the
simulation variants compare selected type errors with Java TLC 1.7.4 and
1.8.0. They check error messages, locations, exit codes, and selected tool
messages, not complete evaluation stacks or all value representations.
`TestJavaTLCNonBooleanGeneration` compares selected errors while generating
initial and successor states. Their surrounding simulation messages can differ
between Java TLC versions.

`TestJavaTLCModelContinue` compares selected model-checking continuation cases
with Java TLC 1.7.4, including initial and successor violations, constraints,
deadlocks, and temporal properties. `TestJavaTLCModelContinueDOT` compares DOT
graphs for three of these cases. `TestJavaTLCModelContinueUpstream` also runs
the original `Continue` and `DoInitFunctorInvariantContinue` model fixtures and
checks their JUnit state counts and selected output with one worker. It does not
run the original three-worker JUnit test or its coverage assertions. Evaluation
error details and other models are not fully mapped.

## Audit work

- Enumerate executable upstream tests, including parameterized and inherited
  cases, and record their expected result and required runtime configuration.
- Map each end-to-end test to a tla-go invocation and compare observable
  behavior with the pinned Java version. Record pass, mismatch, unsupported,
  not yet run, and cases needing a separate test harness.
- Identify Java-internal unit tests whose behavior needs equivalent Go tests;
  keep those distinct from command-line model tests.
- Include simulation, checkpoint and recovery, parallel and distributed modes,
  liveness, fingerprinting, coverage, diagnostics, SANY, and standard modules.
  Some are not implemented in tla-go yet.
- Publish reproducible counts only after the mapping exists. Do not infer full
  compatibility from selected passing tests or from statechart's 33-model corpus.
