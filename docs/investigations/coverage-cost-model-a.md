# Coverage cost model: upstream A

This note describes the work needed to support `ACoverageTest`; A remains
unsupported by tla-go's `-coverage`. The pinned oracle is Java TLC 1.7.4,
upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`, JAR SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`.
The original [model, configuration, and full Java report](../../cmd/tla-go/testdata/java-tlc/upstream-coverage/)
are saved as `A.tla`, `A.cfg`, and `A.coverage`. `TestJavaTLCUpstreamACoverageBoundary`
runs Java's CLI with selected JUnit arguments: `-deadlock -fp 0 -workers 1
-checkpoint 0 -coverage 9999`. The Java CLI report matches the coverage text
asserted in the upstream test source, and its tool messages show 7 generated,
3 distinct, depth 2, finished, no `GENERAL` or coverage-mismatch message. Go
without coverage has the same statistics; with coverage it rejects A. A passing
boundary test does not mean A or its JUnit test is supported.

These are CLI observations, not execution of `ACoverageTest`. The JUnit
`@Before` also configures a recorder and resolver, sets the liveness threshold,
supplies an earlier coverage argument, and enables a DOT dump; `@After` checks
the exit status. The CLI test does not reproduce that setup, the JUnit
assertions, teardown, or dump lifecycle. Therefore it does not establish Java
JUnit execution or Go JUnit parity.

The model needs both the `Init` call to `Op1(1..17)` and action calls
`A(10)` and `B(3)` through `Op2`, which contains `CHOOSE` and a nested
quantifier. Java reports 131,072 evaluations for the initial universal body;
the nested action body reports 162 and 15 evaluations for A and B respectively.
These counts depend on actual enumeration and short-circuiting, not just the
number of generated states.

`TestJavaTLCCoverageParameterizedActionsBoundary` saves smaller, separately
reproducible oracles for the next implementation slice. They all still reject
Go coverage. Their snapshots establish that:

* `A(n) == x'=n`, called as `A(2)`, has one action cost node; changing its
  body to `x<n /\ x'=n` exposes separate costs for `x` and the parameter `n`.
* `Next == A(2) \/ A(3)` combines both invocations under one action wrapper,
  with 6 generated successors, rather than reporting one wrapper per call.
* Reversing two distinct calls does not reverse the reported action order.
  An unused argument and a range-valued assignment have their own saved cases.
* For `A( n  )` and a multiline parameter list, Java's action declaration range
  ends at the actual closing parenthesis. Deriving it from the name or parameter
  count loses whitespace and multiline positions.

## Where the current collector stops

[`checker/coverage.go`](../../checker/coverage.go) builds each action from a
name, a source range and a flat expression list. `addCoverageAction` rejects
parameterized calls; expression counters are looked up by action name and
range. `sortCoverageActions` only sorts distinct ranges in the same module.
Those assumptions cannot express one predicate shared by two action contexts,
or one expression appearing under two different call paths. `collapseAction`
and `finishAction` implement useful tested subsets, but are not a general
`OpApplNodeWrapper` report algorithm.

[`checker/coverage_property.go`](../../checker/coverage_property.go) has a
separate property tree with primary and secondary counts, using evaluator
`OnEvaluate` and `OnBoundVisit` callbacks. It intentionally rejects repeated
definition calls and other ambiguous cases. The callbacks identify an
expression or domain, not the call-path-specific cost-model cursor. They do
not cover initial and successor generation. [`checker/call.go`](../../checker/call.go)
expands actions using [`checker/substitute.go`](../../checker/substitute.go),
which copies expression nodes and replaces formal arguments with caller
expressions. Pointer identity after expansion is therefore not the SANY
semantic-node identity, and a source range alone cannot restore call context.
The parser's definition location covers the complete definition; it does not
currently retain the precise declaration head separately.

In Java, [`CostModelCreator`](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/src/tlc2/tool/coverage/CostModelCreator.java)
builds a graph of wrappers from the semantic tree. It shares next-action models
by predicate semantic node, builds separate roots for initial predicates and
invariants, and orders/deduplicates action reports by predicate location.
[`ActionWrapper`](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/src/tlc2/tool/coverage/ActionWrapper.java)
uses different counter interpretations for INIT, NEXT, and PROP.
[`OpApplNodeWrapper`](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/src/tlc2/tool/coverage/OpApplNodeWrapper.java)
retains ordered children, handles recursion and LET edges, and collapses or
suppresses nodes when reporting. Its `get` resolves the next wrapper from the
current cursor; a previously unseen missing semantic-node UID can emit 2776,
and a missing mapping falls back to the current wrapper rather than silently
inventing a new counter. Each missing UID is reported at most once per wrapper.

Java's [`Tool`](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/src/tlc2/tool/impl/Tool.java)
passes that cursor through `getInitStates`, `getNextStates`, continuations and
operator contexts. For example, a fully assigned initial state rejected by a
later guard increments the initial root's secondary count; next-action root
invocations include generated successors. Counting every AST visit the same
way is incorrect. Arguments can be lazy and cached, so a parameter must be
charged at the actual force under its stored cursor, not when substituted or
when the call is merely entered. Go already has lazy binding and caching in
[`evaluator/operators.go`](../../evaluator/operators.go) and
[`evaluator/evaluator.go`](../../evaluator/evaluator.go); changing their
evaluation order to simplify accounting would change model behavior.

## Implementation sequence

1. Preserve semantic origin and declaration-head ranges through parser, module
   linking, substitution and lazy arguments. Keep an occurrence path/cursor
   separate from source identity: two calls can share a predicate but place
   other expressions under different wrappers. Define how copied and
   instantiated nodes map back to the original semantic identity. Keep added
   parser metadata out of the established JSON projection, and test spacing,
   multiline heads, repeated modules and substituted instances.
2. Add an immutable coverage graph and per-run counters behind the existing
   collector boundary. Model INIT/NEXT/PROP roots, ordered children, recursion
   edges and shared predicate roots explicitly. A debug-only event trace should
   identify the wrapper cursor, semantic origin and primary/secondary update;
   it must not force lazy values or alter execution. Keep the existing collector
   as the default while traces are compared. Neither pointer equality nor
   `parser.Range` by itself is a sufficient key.
3. Port Java's report/collapse rules against synthetic counter graphs first,
   including zero and primed nodes, secondary costs, child order, recursion,
   same-predicate sharing and 1:N action labels. Replay recorded events for the
   seven small oracles above. Check normal and `-tool` output and retain the
   existing snapshot and concurrency tests. Periodic reports need a coherent
   snapshot without holding locks across evaluation or callbacks.
4. Thread an optional cursor through the generator, evaluator, continuation and
   bound iterator; charge only at Java's corresponding operations. Start with
   the simple `direct`, `unused`, `reverse`, and `dup` action cases, then the
   guard and range cases. Establish behavior for duplicate arguments, missing
   definitions, operator overrides, parameter effects, multiple modules and
   recursion before allowing them. Preserve disabled-coverage behavior and
   benchmark it as well as the enabled path.
5. Add `CHOOSE`, set difference, nested quantifiers and lazy operator arguments
   using Java differential tests for empty/singleton sets, early success,
   total exhaustion, repeated calls and failing operands. Then instrument the
   initial `Op1` call, including the actual subset enumeration, and run original
   A with the full JUnit lifecycle. Do not advertise A as passing until its
   complete report, message sequence and statistics match.
6. Before widening eligibility, compare initial and successor errors, failure
   replay, constraints, invariants, repeated/shared nodes, cross-module source
   ordering and substituted instances. Java can evaluate more expressions while
   recovering an error trace; the current collector deliberately omits coverage
   for selected replay failures. Preserve that boundary until event ordering and
   partial reports match. Run the existing coverage oracle suite and `-race`
   after each integration step, not only A.
