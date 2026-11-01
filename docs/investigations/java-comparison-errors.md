# Java TLC 1.7.4 comparison errors

Java oracle: `/tmp/tla2tools-v1.7.4.jar`, SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`,
upstream revision `5a47802`. The direct tests in
`cmd/tla-go/java_tlc_equality_oracle_test.go` use `TLC_JAR` to repeat the
oracle runs. They exercise plain and `-tool` output with source positions.

The checkpoint-tuple reproduction compares two tuples, rather than directly
comparing `1` and `<<1>>`: the corresponding elements are compared while
evaluating `x # <<<<1>>, FALSE, "">>`. Java's tuple `equals` uses the
elements' `equals`; Go previously used its ordering comparison. The same
distinction applies to explicit sets and function values. Java's ordering
comparisons are also used for set normalization and membership, so globally
changing the wording of `Compare` would be incorrect. Java `=`, `#`, and
numeric ordering are separate operators; the direct tests check the latter's
argument errors in both directions. This behavior was observed by running
Java, rather than inferred from the older assumption fixtures.

The reported invariant model now matches Java's 2111 message, exit 76, trace,
and counts. Nested enumerable set and function equality use equality for their
members; the comparison used to normalize a set remains unchanged. Function
keys and values are checked in corresponding pairs, and two function sets are
compared by domain and then codomain, even when either is empty. Direct
Boolean, string, record, and empty-tuple failures have their observed Java
wording. The tests also cover untyped model values and a successful comparison
whose first differing element prevents evaluation of a later incompatible pair.

## Remaining gaps

* Source provenance and exception detail are not generally compatible. With
  `Init == x = 0` and `Next == /\ <<1>> = <<<<>>>> /\ x' = x`, Java exits 75
  and emits `1000` followed by fingerprint stack `2147` at line 4, columns
  12–16, the source of the left tuple. Go exits 75 with the same root error but
  emits evaluation stack `2103` instead. Both plain and `-tool` Java runs were
  checked. Tuple, record, and function construction and conversion provenance
  must be traced through nested values before changing exception wrapping;
  assigning a stack solely from the comparison expression would be incorrect.
* Typed model values retain the existing unsupported-model behavior. For
  `CONSTANT M_1` configured as `M_1 = M_1`, `ASSUME M_1 = 1`, Java exits 10
  with `Attempted to check equality of typed model value M_1 and non-model
  value` followed by `1`; Go exits 255 with `cannot compare incompatible typed
  model values`. This change deliberately does not turn that unsupported case
  into a different model status.
* The fix covers explicit sets, intervals, and tested lazy set conversions.
  Converted functions, mixed-type normalization, and all combinations of
  nested values have not been exhaustively verified. In particular, do not
  infer that all Java comparison exceptions or tuple checkpoint models are
  compatible from these tests.

## Re-review follow-up

Java 1.7.4 accepts `Cardinality({[{} -> {1}], [{} -> {2}]}) = 1`
while direct equality of those function sets is false. Its `SetEnumValue`
normalization calls `ValueVec.sort(true)`, which inserts each element using
`compareTo` and removes a comparison-equal element. The insertion and binary
search order is significant: direct equality and ordering need not agree, and
comparison can evaluate lazy values. Set construction now follows that order;
it does not assume that direct equality is transitive or use it to deduplicate.
The oracle test covers the contrasting direct and nested results, and the value
regression checks fingerprint consistency.

For a lazy function, Java `FcnLambdaValue.equals` materializes the left function
and `FcnRcdValue.equals` converts the right before comparing. Equality now uses
the pairwise equality path after conversion. A Java oracle with `Print` checks
left-to-right body evaluation; a value test checks repeat calls use cached
results. These checks do not establish compatibility for every combination of
lazy domains, recursive bodies, or concurrent evaluation.

The re-review's two P2 cases remain open: record/non-record function equality
can return a generic error and exit 255 instead of Java's diagnostic and exit
10; `Seq({TRUE}) = Seq({1})` has the same exit-code problem for Boolean
comparison, including when nested in singleton sets. Fixing either generally
requires preserving Java's representation and directional comparison details;
changing the global ordering comparator or guessing from a function's keys
would affect unrelated operations. The source-provenance and typed-model-value
gaps above also remain open.

A short local normalization benchmark on Apple M3, with reverse-ordered integer
sets, measured the previous stable-sort/dedup path versus the Java-order
insertion path at 5.93 µs versus 3.76 µs for 100 elements, and 58.5 µs versus
170 µs for 1,000 elements (150 ms per case, one run). Insertion can move a
quadratic number of elements; large reverse-ordered sets may regress. The
comparison order cannot be changed generally without changing observable Java
behavior, so any optimization needs a safe fast path and separate validation.

## Scalar normalization and lazy LET side effects

Homogeneous integer, Boolean, and string set normalization now sorts and
removes duplicates without the insertion path's quadratic element movement.
The fast path preserves the first comparison-equal representative, including
its string payload. Mixed and compound sets still use Java's insertion search,
so their comparison errors and lazy effects retain their previous order. Java
1.7.4 fingerprints for integer, Boolean, and string examples are checked with
FP64 polynomial 0. Java orders strings by `UniqueString` intern order; the
string fingerprint example interns `""`, `"a"`, `"z"` in that order to match
Go's existing lexical order. This does not establish general string ordering or
fingerprint compatibility when Java's intern order differs.

`TestJavaTLCStringInternEqualityGap` records an actionable failure:

```tla
ASSUME {<<"z", TRUE>>, <<"a", 1>>} = {<<"z", 1>>, <<"a", TRUE>>}
```

Java reports Boolean `TRUE` versus `1`; Go reports integer `1` versus `TRUE`.
Java compares intern identities in this context, while Go orders string text.
Matching this requires carrying Java-compatible intern order through parsing,
value construction, normalization, and fingerprinting, including dynamically
created strings. A global change to ordinary string comparison without that
context would change unrelated behavior; this example is a regression target
for that separate design.

The interval size overflow mismatch predates this work: HEAD's `Equal` calls
`Compare`, whose interval comparison uses 64-bit sizes without Java's limit.
For two intervals, equality now follows Java 1.7.4: it checks the left size,
checks the right size only when the left is empty, and otherwise compares the
bounds. The oracle tests include reversed operands, empty intervals, and the
32-bit boundary. Conversion between an interval and an explicit set checks the
interval size in both operand orders. Java also allocates the interval's
elements during conversion; resource exhaustion for very large representable
intervals is not reproduced by the Go comparison.

The following pre-existing difference is reproduced by
`TestJavaTLCLazyLetSideEffectsGap` with Java TLC 1.7.4 and Go:

```tla
EXTENDS Integers, TLC
ASSUME LET F == [i \in 1..2 |-> Print(i, i)]
       IN /\ F[1] = 1 /\ F = F
```

Java prints the arguments `1, 1, 2`; Go prints `1, 1, 2, 1, 2` in that
order. `F` is an ordinary zero-argument operator whose body creates a function
constructor: Go's `evaluateOperatorBody` reevaluates that body on each
reference, producing separate lazy function instances and caches. Go's
`functionDefinition` cache applies to function definitions, and a lazy
function's per-instance cache is additionally keyed by `keepLazy`; it does not
make repeated operator references share the constructed function. Equality
materializes its two operands, accounting for the final four prints. The test
records the observed output, not a complete account of Java's caching rules.
Failed materialization, other definition forms, and concurrent effects have not
been characterized; changing caching needs separate Java-oracle investigation.

For `LET f[x \in Nat, y \in Nat] == x IN f = f`, Java TLC 1.7.4
reports that it cannot compute the number of elements in `Nat` and exits 10.
Go now matches this for products of explicit sets, intervals, nested products,
and the overridden infinite sets. Java sizes factors from left to right even
after an empty factor, before evaluating any function body; all bound expressions
are evaluated first. Tests compare Nat, Int, ANY, tuple bounds, nested products,
empty and finite factors, and effectful bounds against the pinned JAR. A direct
Java API probe of `FcnParams.size()` and `SetOfTuplesValue.size()` with empty,
Int, and Nat factors reports Int in both cases. Go's equality, distinct-function
comparison, pairs, key, and fingerprint paths have matching value-level tests.

The sizing pass also handles powersets, function sets and record sets whose
component sizes it can determine without enumeration, and stops at the first
overflow. It checks a single function-set domain as well as product domains.
Go sorts record fields, whereas Java sizes them in `UniqueString` intern order.
For multiple fields, the pass reports a single field error if all other sizes
are known and their nonzero product cannot overflow. It reports product overflow
if all sizes are known and nonzero; otherwise it accepts a size only when every
field order gives that size, and falls back to enumeration for ambiguous cases.
For example, `[a : Nat, b : Int]` can report either overridden value depending
on prior field-name interning; a zero alongside overflowing factors can suppress
overflow depending on order. Fallback can still miss the Java error, evaluate a
body early, or enumerate a large set in these cases. Exact behavior requires
intern provenance, including prior field names, across parsing and evaluation.
`TestJavaTLCRecordSetMultipleErrorsInternOrder` captures three Java orderings.
Union, set operations, filtered sets and sequence sets with complex bases remain
unknown: determining their sizes may enumerate values, invoke predicates, or
change errors and effects. An
unknown factor stops the pass, so subsequent overflow is not guaranteed to be
detected before enumeration or body evaluation. Go's overflow
diagnostic omits the product or parameter bindings Java prints: formatting
arbitrary elements can itself materialize lazy values and evaluate their bodies.
The direct ordering API still has its identity shortcut for a function itself.

Finite multi-bound function bodies have a separate ordering gap. For
`LET f[x \in {2,1}, y \in {4,3}] == Print("body", <<x,y>>) IN f = f`, Java
TLC 1.7.4 evaluates `<<1,3>>, <<2,3>>, <<1,4>>, <<2,4>>`, while Go evaluates
`<<1,3>>, <<1,4>>, <<2,3>>, <<2,4>>`. This can change the first body error.
Go's Cartesian enumerator is shared by other set consumers, and a function
domain can also contain a nested Cartesian product as a single bound. Reversing
that shared enumerator would change those operations too; the function-specific
enumeration and nested-bound semantics require separate oracle coverage before
changing their order.

For a single record domain `[b : Int, a : Nat]`, Java's first cardinality
error depends on field intern order; Go currently falls back and reports a
generic enumeration error. The nested record domain
`[a : [p : {}, q : Seq({1})], b : {1}]` now reports Java's `Seq({1})` cardinality error before evaluating the body.
Java 1.7.4 represents `Seq` as a `UserValue`; its `size()` always fails,
even for `Seq({})`, without checking the mathematical cardinality of the
base. It formats the value to construct the error. Oracle probes also report
that error for `Seq(Nat)`, `Seq(SUBSET Nat)`, `Seq([{} -> Nat])`, and
`Seq(Seq({}))`. Formatting a filtered base evaluates its predicate: for
`Seq({i \in {1,2} : Print("predicate-marker", TRUE)})`, Java prints twice
before the error. The function body is not evaluated.

The sizing shortcut covers direct sequence domains as well as nested ones,
when the base is an interval, overridden infinite set, explicit empty or
singleton scalar set, or a canonically ordered homogeneous integer or Boolean
set. Java and Go both render `Seq({2,1,2})` as `Seq({1, 2})` and
`Seq({TRUE,FALSE})` as `Seq({FALSE, TRUE})` in the size error. Explicit
iteration order, multiple strings or model values, symbolic sets, and sets
containing lazy functions remain unknown: string and model ordering can depend
on Java intern identity, and formatting complex values can have effects.
For `[a : {}, b : Seq(SUBSET Nat)]`, Java reports the sequence size error,
whereas Go's fallback can complete successfully. For a filtered sequence base
whose predicate prints, both can execute the predicate, but Go can return a
generic enumeration error instead of Java's sequence size error. When a record
contains both `Seq({1})` and `Nat`, the
first Java error depends on field interning, including prior record field
names; Go leaves that combination ambiguous and can return a generic enumeration
error. With `[a : 1..50000, b : 1..50000, c : Seq({1})]`, Java reports
overflow, but putting `c` first makes it report the sequence error; Go falls
back. A Go value-level test checks that inspecting unknown bases does not run
a lazy function body; it does not guarantee that the subsequent fallback will
avoid materialization or effects.

On Apple M3, a focused benchmark of the nested record sizing path measured
227–274 ns/op and 168 B/op (three runs). The old unknown-size path measured
11–19 ns/op and zero allocations but returned unknown rather than constructing
an error, so the timings represent different outcomes, not equivalent work.
