# Java TLC function tuple enumeration

The reference tests in `cmd/tla-go/java_tlc_function_tuple_reference_test.go`
exercise Java TLC 1.7.4 at upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`.
They describe observed Java behavior; they do not assert that tla-go matches it.

`FcnParams.Enumerator` primes a tuple-bound iterator using the index of the
source bound instead of the flattened parameter index. After a grouped bound,
this can advance an earlier iterator. The consequences depend on the earlier
iterator: a class-cast exception, a null-containing function, an array-index
exception, or a successful empty function are all possible. Direct application
can succeed because it evaluates the function without materializing its graph.

A compatibility implementation must also preserve *when* the graph is
materialized. In the deferred-effects reference test, a grouped bound ranges over
`{z \in SUBSET {1,2} : PrintT(z)}`. Java's `SetPredValue.size()` caches the
fully enumerated set, so each predicate effect appears once. Its failed function
rendering falls back to the source description without repeating those effects.
In the tested tla-go revision, the same model prints each element repeatedly
and renders a function graph instead of Java's source description.

The relevant upstream code is in `FcnParams.java`, `FcnLambdaValue.java`,
`SetPredValue.java`, and `FcnRcdValue.java` under
`tlatools/org.lamport.tlatools/src/tlc2/value/impl/`. The following cases need
separate comparison with Java, not just a constructor-level exception check:

- Deferred function materialization through equality, rendering, and state
  fingerprinting, as well as direct lookup and `DOMAIN` without materialization.
- Side effects in deferred set bounds and the lifetime and sharing of cached
  sets and functions, including diagnostic replay.
- `PrintT` deep copies of functions, records, and tuples, and Java's sharing of
  enumerable sets; `ToString` can observe a different cache lifetime.
- Empty and singleton domains, tuple and scalar elements, error context and
  timing, and `-terse` rendering.
- Ordinary function comparison, rendering, fingerprints, effects, race safety,
  and the cost of the unaffected path.

A local prototype modeled the iterator and passed selected Java comparisons,
but repeated deferred-set effects and broke the existing
`TestJavaTLCUnionFirstNonEnumerableContexts/print` error path. It is not part
of the implementation. Fixing the iterator alone is insufficient; a replacement
needs to model the Java value cache and copy behavior without altering ordinary
values or evaluations.
