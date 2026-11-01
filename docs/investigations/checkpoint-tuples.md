# Checkpoint tuple investigation

Status: **not implemented**. The existing scalar checkpoint format and runtime
are unchanged. The conditions below need to be resolved and tested before
admitting tuple-valued states; these are not a claim that every tuple subset is
impossible to support.

## Current boundary

`checkpointPreflight` checks every definition before preparation, and
`checkpointSupported` restricts INIT, NEXT and invariants to the closed pure
expression language. Only serial BFS without reductions, effects, unsupported
modules or options is eligible. Tuple syntax is allowed in preparation only in
some contexts; it is not accepted in the search expressions. Unsupported CLI
models fall back before exploration, while recovery fails closed. The checkpoint
value codec accepts int32, Boolean and byte-preserving strings, not functions.

Values are immutable through the public constructors and accessors, which copy
slices. `Tuple` creates a function with keys 1..n and marks tuple provenance;
`TupleElements` also accepts other functions with that domain. An encoder must
check `IsTuple`, not merely call `TupleElements`, to retain the representation
used in diagnostics (especially for the empty function). State keys use the
function's keys and values, irrespective of tuple provenance. With wide strings
some models use TLC's 64-bit fingerprint instead, which recursively traverses
functions and encodes strings as UTF-16; exact state keys preserve string bytes.
The existing identity hashes source, effective configuration and semantic
options. Public Go strings can contain invalid UTF-8; checkpoint input identity
rejects invalid UTF-8, whereas scalar checkpoint values are stored as bytes.

V1 uses `TLAGOCK1`, length, JSON payload and SHA-256 checksum. The reader limits
the file to 64 MiB plus framing, verifies length and checksum, rejects unknown
fields, checks identity and validates queue, counters, nodes and visited keys.
The writer serializes before checking the 64 MiB payload limit, then writes and
syncs a temporary file, renames it in the destination directory and syncs the
directory where supported. This is a serialized-size bound, not a peak-memory
bound. Checksums do not authenticate data. Go's JSON decoder permits duplicate
object members and replaces invalid UTF-8 in JSON strings; unknown-field
rejection alone is not a strict canonical or duplicate-key check. Any revised
reader must explicitly decide how to handle these cases without breaking valid
original v1 int/bool and later string files. A new version must also fail closed
in older readers and keep tuple-vs-function provenance.

## Concrete blockers

1. Simply allowing tuple expressions makes `Init == x = <<>>` and
   `Next == x' = <<x>>` eligible, although the value grows one level at every
   expansion. `x' = <<x, x>>` creates shared immutable subvalues but traversal
   or tree encoding can expand their size exponentially. Both exact-key and
   TLC-fingerprint traversal recurse before a checkpoint boundary; a codec-only
   depth limit therefore cannot provide the requested bounded recursion or
   memory. Public construction does not expose a straightforward way to create
   a cycle, but a decoder must still bound nesting and total elements before
   constructing values, and must not rely on this observation as a bound on
   deep or exponentially expanded acyclic values.
2. Java comparison errors are observable. With the model below, Java says
   `Attempted to check equality of integer 1 with non-integer:` while tla-go
   says `Attempted to compare integer 1 with non-integer:`. Both stop with the
   same two-state trace and counts. A supported language must either exclude
   such comparisons with a sound analysis or address and test this semantic
   discrepancy. Restricting tuple literals alone is not a proof that all
   comparisons of values flowing through variables are safe.
3. A versioned recursive wire format needs explicit aggregate depth, element,
   byte and allocation budgets on both save and load, including the JSON scan
   before recursive conversion and visited-key validation. It also needs tests
   for duplicate fields, conflicting fields, null/empty forms, malformed UTF-8,
   checksummed forged states, and unmodified v1 fixtures. No such design and
   end-to-end proof was completed in this investigation.

A promising narrower candidate is to admit only bounded, statically constructed
literal tuple trees, plus assignment or comparison rules proven not to construct
larger trees through variable flow. That candidate still requires a proof for
all admitted AST forms and state generation, and process-restart comparisons
covering status, counts, observations and exact error traces. Limits and their
failure behavior must be defined before implementation.

## Reproduction against Java TLC 1.7.4

The pinned `/tmp/tla2tools-v1.7.4.jar` has SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`,
matching CI, and reports upstream revision `5a47802`. Save the following as
`M.tla` and use `INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE` in
`M.cfg`:

```tla
---- MODULE M ----
VARIABLE x
Init == x = <<>>
Next == (x = <<>> /\ x' = <<1, TRUE, "a">>) \/ (x = <<1, TRUE, "a">> /\ x' = <<<<1>>, FALSE, "">>) \/ (x = <<<<1>>, FALSE, "">> /\ UNCHANGED x)
Inv == x # <<<<1>>, FALSE, "">>
====
```

Run Java with `java -XX:+UseParallelGC -Xmx256m -jar
/tmp/tla2tools-v1.7.4.jar -workers 1 -fp 0 -checkpoint 0 M.tla`; run tla-go
with `mise exec -- go run ./cmd/tla-go -workers 1 -fp 0 -checkpoint 0 M.tla`.
Java reports 2 generated, 2 distinct, depth 2, and the trace `<<>>`,
`<<1, TRUE, "a">>`; tla-go reports the same trace and counts but the different
error wording above. This is a normal-check comparison, not Java checkpoint
compatibility.

A separate bounded model using a numeric counter and empty, flat and nested
tuples produced the same three-state invariant-violation trace, 3 generated,
3 distinct and depth 3 in both implementations. With positive checkpointing,
tla-go rejected that model during preflight and checked it normally. No tuple
checkpoint was produced or resumed. Unicode, long values, malicious tuple JSON,
and fresh-process tuple recovery remain unverified for this proposed feature.

The bounded model used for that second comparison (with the same config) was:

```tla
---- MODULE M ----
EXTENDS Naturals
VARIABLES n, x
Init == /\ n = 0 /\ x = <<>>
Next == \/ /\ n = 0 /\ n' = 1 /\ x' = <<1, TRUE, "a">>
        \/ /\ n = 1 /\ n' = 2 /\ x' = <<<<1>>, FALSE, "">>
        \/ /\ n = 2 /\ UNCHANGED <<n, x>>
Inv == n < 2
====
```
