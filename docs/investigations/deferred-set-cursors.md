# Deferred-set iteration and caching

These observations use Java TLC 1.7.4 at upstream commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba`, JAR SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`.
They are focused CLI and Java value-class experiments, not upstream JUnit
results. The Go implementation remains incomplete for other deferred values.

## Iterator construction is observable

`SetCupValue.Enumerator` constructs the left and right iterators before it
returns an element. `UnionValue.Enumerator` takes the first outer member and
opens its iterator in its own constructor. A model with
`Init == x = Cardinality((UNION {1..100}) \cup UNION {1})`,
`Next == UNCHANGED x`, and `-maxSetSize 10` makes Java exit 75 with an invalid
UNION-element error before checking any left member. Traversing the left side
until the limit before opening the right instead reports a size error. This is
a Java CLI observation; it is not an upstream JUnit test.

No cursor has been connected to production for this case. An implementation
that chooses a raw iterator from expression shape alone cannot tell whether an
aliased Java value was already cached. The earlier callback path stays in place
until that distinction is represented.

A general implementation needs pull iterators for all enumerable value kinds,
including their actual construction and prefetch behavior. For example,
`SetOfTuplesValue.Enumerator` opens factors in order and reads the first element
of each at construction, stopping before later factors if one is empty. It then
advances its odometer before returning each tuple.
Filters evaluate predicates during `nextElement`; Java's cap and difference
choose an enumerable operand according to Java's `Enumerable` interface. A
callback or goroutine that starts the right child only after exhausting the left
cannot reproduce these times or errors. Iterators must also preserve early
termination, reset behavior, nested failures and coverage costs.

## Conversion and aliasing are observable

In the pinned Java source, `UnionValue` and `SetCupValue` return a fresh
`SetEnumValue` from `toSetEnum()` when their cache is absent; that conversion
alone does not store the result. Their `size`, receiver `equals` / `compareTo`,
`fingerPrint` and `permute` call `convertAndCache` first. Before conversion,
`elements()` emits duplicate values. Once cached, it enumerates the cached
`SetEnumValue`, whose iterator normalizes and deduplicates it. For UNION and
cup, `member` still consults the original operands after conversion;
`SetPredValue` instead uses its converted set for membership.

The distinction is visible without relying on text formatting. Compile this
probe against the verified JAR and run it with the JAR on the classpath:

```java
import tlc2.value.impl.*;
class Probe {
  static int count(Enumerable e) {
    ValueEnumeration it = e.elements(); int n = 0;
    while (it.nextElement() != null) n++;
    return n;
  }
  static UnionValue union() {
    return new UnionValue(new SetEnumValue(new Value[] {
        new IntervalValue(1, 6), new IntervalValue(2, 7)}, false));
  }
  static SetEnumValue seven() {
    return (SetEnumValue) new IntervalValue(1, 7).toSetEnum();
  }
  public static void main(String[] args) {
    UnionValue u = union(), alias = u, independent = union();
    System.out.println(count(u));                     // 12
    u.toSetEnum().size(); System.out.println(count(u)); // 12
    seven().equals(u); System.out.println(count(u));    // 12
    u.equals(seven());
    System.out.println(count(alias));                  // 7
    System.out.println(count(independent));            // 12
  }
}
```

`SetEnumValue.equals` calls `toSetEnum` on its argument, which explains why
reversing the equality does not cache `u`. A cache keyed by mathematical set
contents or a symmetric equality cache therefore changes behavior. Copies of
one Go `Value` currently share a payload pointer; independent constructors
produce different payloads. `maps.Clone` in states and evaluator snapshots
copies values while retaining their payload references, whereas
`DeepNormalizedMembership` and `OrderSet` may copy payloads. An attempt to
preserve repeated references within one membership normalization changed
observable behavior: `checker.compareOrderedValues` short-circuits identical
Go values, so comparing two shared filtered values skipped predicate
evaluations that occurred with separate copies. Retaining the previous payload
copies avoids that unverified semantic change. Any object identity
scheme must audit these transformations, lazy argument snapshots, constants,
symmetry and checkpoint restoration. A process-global cache can also couple
independent checks; cache ownership and public API reuse need an explicit rule.

`TestPinnedJavaDeferredValueIdentity` executes the value-class probe against the
verified JAR, including independent values, `deepNormalize`, `deepCopy`, cup,
and the failed-normalization case below. It is Java-only and does not count as
a tla-go compatibility or JUnit pass.

There is more than a success/failure bit. With an unnormalized cup of `1..1`
and `{TRUE}`, `elements()` can initially yield both values. Calling `size()`
first installs its enumerated vector and then fails while normalizing mixed
values; a subsequent `elements()` attempts normalization and fails immediately.
`ValueVec.sort(true)` mutates the vector as it sorts, so the state after a
failed comparison may even be partially modified. A cache that resets after
every failure, a `sync.Once` that permanently stores all errors, or a cache
containing only a fully normalized Go `Set` does not model these transitions.
`toSetEnum` and `convertAndCache` need separate operations and the materialized
but not normalized state must be represented before enabling cache-sensitive
paths.

Java's `deepNormalize` additionally uses a `DummyEnum` marker and recursively
normalizes children. The pinned `UnionValue` comments describe a real race when
workers share a precomputed constant. Go deferred predicates capture snapshots
of state and scope, but their closures may still produce observable effects.
A concurrent implementation needs stable per-object identity, safe publication,
reentrancy handling, and no locks held while evaluating predicates or invoking
callbacks. A race-free lock alone does not prove that parallel side effects occur
in Java's order. Effectful cases need a defined scheduling strategy and oracle
coverage before broadening worker support.

Fingerprinting is a separate integration boundary: Java's fingerprint method
converts and caches, while Go's exact `Key` and TLC fingerprint paths can each
materialize a deferred value. A preliminary size walk followed by the existing
key walk can repeat predicates and perform the bound check at the wrong time.
The implementation must make the actual identity operation reuse the correct
materialization while preserving source stacks, `VIEW`, symmetry and nested
values. Required comparisons include bound 10/11 with duplicates, both operand
orders, aliases versus independent values, failure and retry, partial traversal,
`PrintT` side effects, concurrent shared values, and both exact keys and FP64.
