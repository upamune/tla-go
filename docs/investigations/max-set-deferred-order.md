# Deferred set normalization and enumeration order

These observations use the Java TLC 1.7.4 JAR with SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`,
upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`, and the
single-worker CLI with `-maxSetSize 10 -checkpoint 0 -tool`. They are not
upstream JUnit executions. Tests in
`cmd/tla-go/java_tlc_max_set_cache_test.go` distinguish matched Go behavior
from Java-only observations.

The following behavior prevents a stateless, one-pass replacement for
`Cardinality` from being generally compatible:

1. **A comparison can cache a shared value.** With
   `F(S) == IF S = {1,2,3,4} THEN Cardinality(UNION {S, 2..7}) ELSE 0`,
   `F(UNION {1..3, 2..4})` succeeds in Java and Go. Java's
   `UnionValue.equals` first calls `convertAndCache`, so the second use of
   the same lazy argument sees its normalized four-element set. When the
   equality is reversed, `SetEnumValue.equals` calls `S.toSetEnum` without
   caching the result: Java later raises the size-bound error, while Go
   currently succeeds. The lazy argument itself is cached; its value's
   normalization state and the operation that produced it also matter.
2. **A set operation can swap its operands for reduction.** Java
   `Tool.eval` calls `Reducible.cup` on a reducible right operand if the left
   operand is not reducible. For `(UNION {1..4,2..5}) \cup {100}`, a filter
   calling `PrintT` therefore sees `100` first, followed by the deferred
   union's eight values, including duplicates. Reversing the operands gives
   the same order. Without a reducible operand, Java keeps the source order.
   Intersections can also select a reducible right operand; difference does
   not swap its operands. Java-only tests record both directions and eager
   and deferred controls. Current Go does not reproduce all of these print
   sequences.
3. **Iterator construction can fail before either side yields.**
   `SetCupValue.Enumerator` obtains both operands' `elements()` in its
   constructor. For a union with `UNION {2}` on the right, Java reports that
   a union member is not enumerable before a `PrintT` filter sees any left-side
   values. Current Go also emits no values for this finite model, but its error
   details differ. Checking each operand only when its first item is requested
   changes both the error and the observable output.

An experimental implementation counted deferred UNION and set-operation
elements before deduplication and stopped at the bound. It correctly handled
some uncached duplicate cases, but incorrectly failed the cached comparison,
chose the wrong assertion for the reducible-right union, and printed elements
before the iterator-construction failure. The experiment was not integrated.
The regression tests pass on the existing Go implementation where explicitly
stated; they do not claim that Go's general deferred set behavior is complete.

Any general replacement needs to preserve per-value identity and Java's
operation-specific cached normalization, while representing iterator creation
and element retrieval separately. It must account for aliases, repeated
arguments, nested set operators, side effects, errors, concurrent readers and
the use of values as Go map keys. Adding a shared mutable cache without
reviewing those semantics, independently re-enumerating to check a bound, or
counting the final distinct elements cannot reproduce the cases above. State
fingerprinting and its error-source stacks remain separate unfinished work.
