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
   values. Go now reports the same nested 2154 error for this finite model.
   This is only error classification: Go still opens the right operand after
   enumerating the left, so left-side effects in other models can differ.
   Checking each operand only when its first item is requested changes both
   the error and the observable output.

For `Cardinality(UNION {1})`, the error is raised while opening the UNION
enumerator and wrapped by the `Cardinality` Java override. Go now reports the
same error, including for nested UNIONs and successor evaluation. The same
first-element validation applies to the Go value API without assuming a
particular outer-set representation. Error reporting still differs when an
earlier expression has a side effect: Java replays the expression while building
the error stack, whereas Go does not. Membership in an invalid UNION is also a
separate path: for `1 \in UNION {1}`, Java reports a fingerprint error and Go
still emits a generic enumeration error.

When the invalid UNION itself is stored in an initial state, Java first fails
fingerprinting and then tries to print that state. Ordinary printing attempts
the same enumeration again; this second failure replaces the fingerprint
report and exits 255 without statistics. With `-terse`, the UNION prints
symbolically and Java instead reports the initial state and fingerprint stack
with exit 75. A successor fingerprint failure also exits 75. Go follows those
paths for the tested initial and successor states, including the exact-key and
FP64 checker paths. Nested record and tuple expressions now retain their
fingerprint source frames, and a UNION passed as an operator argument retains
its application location. These paths are compared with Java for the tested
initial and successor failures; other container and operator forms need their
own comparisons.

These matching exit codes do not imply matching side effects. With `-terse`,
putting `UNION {i \in 1..3 : PrintT(i)}` in an initial or successor state
prints `1`, `2`, `3` twice in Java, which replays evaluation while building
the error report; Go prints them once. This discrepancy predates the error
classification change. Likewise, for `UNION {SUBSET Nat}` with `-terse`,
Java exits 75 and Go exits 255, both before and after that change. Java's
`Enumerable` interface does not imply that every particular value can actually
be enumerated. By contrast, the tested `UNION {Nat}`, `UNION {Int}`, and
`UNION {STRING}` errors match in normal and terse output.

For an initial VIEW `V == IF x = 0 THEN UNION {1} ELSE {}`, Java and Go now
report 2102, 2103, and 2199 with exit 75. For a successor VIEW
`V == IF x = 0 THEN 0 ELSE UNION {1}` and `Next == x' = x + 1`, both report
1000, 2103, and 2199. TLC evaluates values constructed in a VIEW with its
FastTool even during replay, but a value reused from the state can have a
CallStackTool source and report 2147 instead. Tested direct, nested, higher
order, constant and lazy-function cases cover both paths. Replay also matters
when `TLCSet` makes a previously failing VIEW succeed: Java can return exit 0
after an initial-state failure or 255 after a successor failure. These specific
cases now match, including state counts and the order of error headers and
`PrintT` output. Other VIEW and replay combinations remain incomplete. In
particular, `TLCGet("level")` during ordinary VIEW evaluation can differ;
the tested replay itself reads level zero in both implementations.

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
