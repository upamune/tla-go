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

The deferred-state discrepancies above remain unresolved. A fix must preserve
conversion and error order, including nested values and `VIEW`/symmetry, rather
than applying a blanket cardinality test to every value in a state.
