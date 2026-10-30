# Statechart compatibility plan

Statechart integration is part of tla-go's Java TLC compatibility goal. This
plan tracks the additional interfaces needed to use tla-go as a statechart
backend; matching state counts alone is not sufficient.

At tla-go commit `25cc6e0`, the statechart report supplied by the user says 32
of 33 specifications matched Java TLC's exit code and state counts. That report
has not yet been reproduced in this repository. The remaining specification and
its failure need to be recorded alongside the integration tests.

## 1. Establish the integration baseline

- Obtain the statechart repository, its exact revision and all 33 test specs.
  Record the Java TLC version, commands, configuration, and expected results.
- Run the existing integration tests with Java TLC and capture representative
  `-tool` messages, DOT files and SANY XML. Include successful exploration,
  deadlock, invariant violation, syntax or semantic error, and non-ASCII values.
- Keep the existing Java backend as the default in statechart. The alternative
  backend must be selected explicitly or when available according to the
  consumer's integration design; do not silently substitute an unverified binary.

## 2. Module and CLI compatibility

Module-level `USE DEF` and `USE DEFS` directives are parsed and checked for
defined operators and declaration order. Targeted tests also cover facts and
`HIDE` directives, imported definitions, and invalid references against Java
TLC 1.7.4. Structured proofs and all possible proof directives are not yet
supported; the statechart specifications have not yet been verified.

Named and unnamed `INSTANCE TLC` support the TLC operators implemented by
tla-go, including runtime state and side effects. Targeted tests compare their
results, failures, random behavior and DOT output with Java TLC 1.7.4; local
`TLC.tla` files retain precedence. Other standard-module instances and full
statechart coverage remain outstanding.

- Support the module search paths used by statechart, with Java-compatible
  lookup order, relative paths, transitive imports, and diagnostics. Test paths
  containing spaces and missing or conflicting modules.
- Complete named standard-module instances, including qualified operator calls
  and relevant substitutions. Retain support for user-defined modules and verify
  `X == INSTANCE TLC` with the actual statechart models.
- Verify the `USE DEF` forms used by the actual statechart models without
  changing model-checking semantics. Invalid proof directives must be rejected.
- Complete the Java-compatible behavior of the accepted `-workers 1` and
  `-fp 0` options, including fingerprint collisions, and implement `-metadir`.
  Verify the actual statechart invocations and filesystem behavior against Java
  TLC. Test invalid and unsupported values too.

## 3. TLC tool protocol

- Implement `-tool` message framing, codes, severity, ordering, output streams,
  and process exit behavior for the messages consumed by statechart. Use Java
  TLC as the oracle, including progress, success, failure, and counterexamples.
- Add direct protocol tests and tests using statechart's real message parser.
  Preserve normal human-readable output when `-tool` is not selected.

## 4. DOT state graph

The CLI now supports `-dump dot,actionlabels`, `-workers 1`, and `-fp 0` for
single-model runs. Targeted tests compare DOT bytes with Java TLC 1.7.4 for
branching, repeated transitions, constraints, VIEW, symmetry, Unicode,
liveness stuttering and failure paths. This is a partial implementation:
statechart's actual specifications have not yet been checked, other dump
formats and worker/fingerprint settings are unsupported, and state identity
and fingerprint collision behavior can still differ from Java TLC.

- Complete the `-dump dot,actionlabels` invocation for all statechart models,
  including Java-compatible state, transition, and value formatting.
- Compare parsed graphs against Java TLC for branching, repeated transitions,
  self-loops, constraints, special characters and Unicode. Exercise failure
  paths and ensure output and errors are handled correctly.
- Keep graph collection opt-in. Measure its runtime and memory costs on both
  small and larger state spaces, and avoid slowing ordinary checks unnecessarily.

## 5. SANY-compatible metadata

- Identify the exact SANY XML command and fields statechart consumes, including
  variable declaration order and operator definition levels. Implement an
  equivalent interface that its existing consumer can read, or coordinate an
  explicitly tested consumer change.
- Derive levels from semantic analysis, including imports, substitutions,
  parameters, bound names and standard modules; do not guess unsupported levels.
  Reject unsupported cases rather than emit misleading metadata.
- Compare parsed output against Java SANY for the statechart specs and targeted
  fixtures covering ordering, nested instances, scopes, and semantic errors.

## 6. End-to-end acceptance

- Run all 33 statechart specifications through both backends. Compare the
  consumer-visible tool messages, graph and SANY metadata, final charts, exit
  codes, and explored state counts. Investigate any differences rather than
  accepting a count match as proof of equivalence.
- Test Java fallback, tla-go selection, missing binaries and errors in the actual
  statechart integration. Preserve the Java default until compatibility is
  verified and the consumer chooses to change it.
- Run the repository's race tests, quality and security checks, the Java TLC
  differential tests, native binary smoke tests, and relevant benchmarks. The
  Java baseline and any newer TLC version used must be identified explicitly.
