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
reproducible oracles. Go now supports the restricted direct-assignment and
UNCHANGED cases, including repeated calls and declaration spacing; the guard
and range-assignment cases still reject coverage. Their snapshots establish that:

* `A(n) == x'=n`, called as `A(2)`, has one action cost node; changing its
  body to `x<n /\ x'=n` exposes separate costs for `x` and the parameter `n`.
* `Next == A(2) \/ A(3)` combines both invocations under one action wrapper,
  with 6 generated successors, rather than reporting one wrapper per call.
* Reversing two distinct calls does not reverse the reported action order.
  An unused argument and a range-valued assignment have their own saved cases.
* For `A( n  )` and a multiline parameter list, Java's action declaration range
  ends at the actual closing parenthesis. Deriving it from the name or parameter
  count loses whitespace and multiline positions.
* `TestJavaTLCCoverageDistinctInstanceOriginsBoundary` instantiates the same
  definition twice with both equal and unequal constant substitutions. Java
  reports separate `I!Act` and `J!Act` roots in both cases, each with the same
  `Inner` expression range and its own counts. The repeated `A(2) \/ A(3)`
  case instead uses one root. Thus a range or original definition alone cannot
  identify a reportable action. These are Java CLI comparisons; Go rejects
  these coverage models.
* `TestJavaTLCCoverageSharedArgumentOriginsBoundary` uses an action containing
  `n=n` and calls it with `x`. Java reports the two distinct formal locations
  beneath one action even though Go's expansion currently places the same
  caller-expression pointer in both positions. Normal Go exploration and Java
  both produce five generated and two distinct states; coverage remains rejected.

## Where the current collector stops

[`checker/coverage.go`](../../checker/coverage.go) builds each action from a
name, a source range and a flat expression list. The limited parameterized-call
path binds each call occurrence to a shared predicate root and tracks its
single supported expression at runtime; other expressions remain keyed by
action name and range. `sortCoverageActions` only sorts distinct ranges in the same module.
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

For a baseline before origin tracking, on an Apple M3 with `-benchtime=200ms`
and three samples, `BenchmarkCoverageDisabledSubsetInvariant` used about
368–369 KB and 4,289–4,290 allocations per operation (0.74–0.84 ms), while
`BenchmarkParseModule` used about 16.3 KB and 54 allocations (4.47–4.59 µs).
These are reference measurements on this machine, not a before/after
comparison; competing work can affect elapsed time.

For the later limited parameterized-action change,
`BenchmarkCoverageParameterizedCalls/disabled` was run against its parent and
the change in five alternating one-second pairs with `GOMAXPROCS=1`. Both used
11,218 allocations and about 1.55 MB per operation. Elapsed samples ranged
from 3.65 to 5.05 ms for the parent and 3.65 to 8.05 ms for the change while
Java and other processes were active. These samples cannot establish a timing
change or rule out a regression. The disabled path still checks whether
coverage is configured; the coverage-only deferred cleanup is in a separate
helper.

For property coverage reporting, `BenchmarkCoveragePropertyReportDeep` measures
an artificial chain with alternating counts, so each ancestor must print its
children. On an Apple M3, three 200 ms samples for a depth-1024 chain changed
from 3.64–3.85 ms to 0.078–0.116 ms after replacing repeated descendant scans
with a postorder walk; both versions used 174,208 bytes and 11 allocations per
report. These measurements cover report construction, not a complete model
check. A fully collapsible depth-256 chain keeps the root-level fast path; both
versions used 80 bytes and one allocation per report. Elapsed samples ran under
different background loads and do not establish end-to-end speedups.
An unequal root above an otherwise collapsible chain of 1,024 nodes used 224
bytes and two allocations per report both before and after the change; elapsed
samples were 0.028–0.034 ms and 0.033–0.035 ms respectively under overlapping
load. These samples do not establish a timing difference.

## Origin tracking obstacle and first implementation boundary

No general production origin tracker has been added yet. There are two independent
identities to preserve: a source semantic origin and a position in an action's
cost-model graph. The same source expression may appear in multiple INSTANCE
roots, while several calls may share one Java action root. A source range, a
definition name, a Go pointer, or a single per-expression origin tag cannot
represent both relationships. The two CLI cases above are regression fixtures
for this distinction; they do not establish a general rule for all instances.

The existing [`DefinitionOrigin`](../../syntax/parser/ast.go) records a
definition's module, name and parameter prefix for configuration overrides. It
does not identify each expression or the occurrence in which it is evaluated.
[`substitute`](../../checker/substitute.go) clones nodes, but replaces a
formal identifier with the caller's expression; lambda substitution can return
an expanded body. [`alphaRenamer.expression`](../../checker/alpha.go) makes a
separate copy, while [`instanceDefinition`](../../checker/instance_definition.go)
clones and rewrites it again. `partialInstanceOperator` can replace a node in
place with a synthetic lambda and synthetic children. Tracking only the final
AST pointers or copying one field in `substitute` would lose origin information
at those other transformations, and would not tell an argument's source from
the formal occurrence that caused its evaluation.

A concrete example is `F(n) == n=n` followed by `F(x)`: the two formal `n`
occurrences have different source ranges, but `substituteActionCall` currently
places the very same caller `x` pointer in both result positions. A map from
result pointer to one origin cannot recover both formal occurrences. The
origin representation needs a parent edge or occurrence path in addition to
the argument's own origin; that context must survive until the collector or a
debug event consumer receives it. Merely constructing an unused registry after
linking cannot reconstruct information already discarded by these rewrites.

The public `parser.Expression` and `parser.Definition` structs currently have
exported fields. Adding a field can break callers using unkeyed composite
literals, even if it is omitted from JSON. Mutating parsed nodes to attach
metadata can also interfere with concurrent checks or callers that reuse a
module. An implementation that preserves the existing Go API and JSON
projection should instead keep per-check metadata outside the public AST, with
an explicit registry passed through the transformation paths. A global map is
not sufficient: it would couple checks, complicate lifetime management and
introduce concurrency hazards. Parsed modules supplied through the Go API or a
JSON round trip may lack `SourceTokens`; missing head provenance must be
represented as unavailable rather than synthesized from name length.

The next concrete implementation step is to design a per-check origin registry
and thread it through the existing instance-resolution session, alpha renaming,
subexpression rewriting and action substitution. Each rewrite must record both
the source node and the transformation/call occurrence, including synthetic
nodes and substituted arguments; consumers must not infer semantic identity
from their ranges. The registry should be optional when coverage/debugging is
disabled, and tests should exercise concurrent checks over the same parsed
module as well as nested instances, operator-argument substitution and clones.
Separately retain declaration-head ranges from parser tokens with explicit
handling for whitespace, multiline parameters and unavailable tokens. Only
after those cases match the pinned Java observations should the event/tree
consumer use the registry to enable additional coverage models.
The registry must be introduced before instance resolution, which currently
precedes model construction, and threaded through the real transformation and
execution paths. A useful first integration should have a consumer, such as an
explicitly enabled debug trace that reports source and occurrence separately;
unknown rewrites must be reported as unsupported, never guessed from a range.
Only then can tests establish shared predicate roots and distinct INSTANCE
roots in the registry itself, rather than just observing the Java CLI reports.

In the current flow, `checkParsedWithOptions` creates `instanceSession` and
resolves extensions and instances before `prepareWithSources` constructs the
model. An opt-in registry must therefore be created at that earlier boundary,
then passed through the resolver's alpha/substitution helpers and into the
model; constructing it in `newCoverage` is too late. During execution,
[`workPath`](../../checker/work_path.go) is shared and keyed by action names
for recursion detection, so it cannot itself distinguish two call sites or
serve as an occurrence cursor. Action-body caching in
[`checker/call.go`](../../checker/call.go) also means cached expression pointers
can be revisited under different contexts.

For the first debug consumer, carry a separate optional occurrence cursor on
the execution work/continuation and propagate it into actual operator argument
evaluation. Record a child occurrence for each parent edge, even when both
edges refer to the same result pointer. Add balanced enter/leave or equivalent
scoped events for evaluator calls: the existing `OnEvaluate` callback reports
entry only and does not identify a return to the parent or a lazy argument's
formal occurrence. The current `Options.OnDebug` contract is for liveness, so
silently adding coverage events to it would change that API's behavior. The
debug channel and its ownership need an explicit design that leaves normal
checks and the existing public structs unchanged. This requires an integrated
change across resolver, generator and evaluator rather than a stand-alone map;
the fixtures above can verify the resulting events before any coverage gate is
relaxed.
