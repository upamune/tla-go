# Java TLC 1.7.4 set-bound observations

These observations use the 1.7.4 release `tla2tools.jar` (SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`),
with `-workers 1 -maxSetSize 10` and the following model, replacing `EXPR`:

```tla
---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = EXPR
Next == UNCHANGED x
====
```

The configuration is `INIT Init`, `NEXT Next`, and `CHECK_DEADLOCK FALSE`.

| EXPR | Java result | Observed tla-go result before this change |
| --- | --- | --- |
| `SUBSET (1..4)` | success | success |
| `(1..3) \X (1..4)` | exit 255, set-bound exception | success |
| `[a |-> (1..3) \X (1..4)]` | exit 255, set-bound exception | success |
| `(SUBSET (1..4)) \cup {99}` | exit 75, set-bound exception during fingerprinting | exit 75, a different error |

The power set exception is intentional Java behavior, not evidence that state
sets are always exempt. In the pinned source, `SubsetValue.toSetEnum` constructs
`ValueVec(this.size())`, so adding those elements does not trigger its growth
check. By contrast, `SetOfTuplesValue`, `SetOfFcnsValue`, `SetOfRcdsValue`,
`SetCupValue`, `SetCapValue`, `SetDiffValue`, and `UnionValue` use the default
`ValueVec()` capacity of 10 when converting their deferred sets. Intervals
fingerprint directly without this vector. See the corresponding classes and
`ValueVec.ensureCapacity` under `tlatools/org.lamport.tlatools/src/tlc2/value/impl`
at the `v1.7.4` tag.

The nested and deferred-state discrepancies above remain unresolved; the
narrow top-level case is addressed below. With `-tool`, the
product and record examples emit message 1000, identifying
`util.WrongInvocationException` and the bound, and exit 255 without a state or
fingerprint trace. For a successor `x' = (1..3) \X (1..4)`, Java instead emits
message 1000 naming a `java.lang.RuntimeException`, followed by message 2147
with the product expression's source location, and exits 75. The deferred
union above also emits 2147 and exits 75, but its initial-state report includes
the state. These outcomes were reproduced with the JAR hash above; the
Java-only reference tests record the initial product and nested forms.

The pinned `ModelChecker` evaluates the state constraint before fingerprinting
an included state. `SetOfTuplesValue.fingerPrint` and `SetCupValue.fingerPrint`
call `convertAndCache`, which enumerates into a default-capacity `ValueVec`;
their `deepNormalize` methods instead mark the conversion as pending. Error
handling can then attempt to print an initial state, which itself can expand a
lazy value. A constraint, comparison, VIEW, symmetry permutation, or error
report can therefore trigger conversion at a different point than a simple
preflight traversal would. Go's canonical keys and lazy membership handling do
not model all of these effects. A complete fix must preserve the conversion,
evaluation, and diagnostic order, including nested values and reductions,
rather than applying a blanket cardinality test to every value in a state.

The separate deferred `Cardinality` bound check now compares its 64-bit count
directly: calculating `limit + 1` overflowed for the largest Go API limit and
could incorrectly reject a small set. This does not fix state fingerprinting.

## Incremental implementation

For breadth-first initial states, an explicitly configured bound now applies
while the ordinary state key converts a top-level Cartesian product of interval
factors to an enumerated set. This is restricted to a single variable, no
VIEW, symmetry, or state constraints, and the exact-key path; it runs at
the point where the included candidate would be keyed. The conversion counts
elements as
TLC's default-capacity vector does, so up to `max(bound, 10)` elements are
accepted. The bound exception is reported as Java's 255 / message 1000 without
state, statistics, or fingerprint trace. The default-option path is untouched.

This is deliberately incomplete: successors, nested products, non-interval
factors, multiple variables, state constraints, reductions, the FP64 key path,
and the implicit default bound still use the previous behavior. In particular, Java can try to
expand the initial state while reporting a constraint error: with an oversized
product and `C == x[1] = 0`, the expansion replaces that error with the bound
exception (255). This diagnostic conversion is not yet modeled. Even below the
bound, that constraint error currently has a different initial-state tool
message in Go. Extending the bound must account for those diagnostic paths and
conversion caching, rather than checking cardinality before evaluation.

`-terse` is also excluded: Java prints the product symbolically in the initial
state and reports exit 75 with messages 2102 and 2147, replaying initial-state
evaluation and its side effects. A simple 255 bound failure would be wrong in
that mode. The existing Go mismatch in that mode remains unresolved.
