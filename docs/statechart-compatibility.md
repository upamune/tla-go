# Statechart compatibility plan

Statechart integration is part of tla-go's Java TLC compatibility goal. This
plan tracks the additional interfaces needed to use tla-go as a statechart
backend; matching state counts alone is not sufficient.

The statechart corpus from `upamune/skills` draft PR #29 was identified at
commit `cbc22d03dc6090cbba8b3a1b5cc13a0267bd4596`. Its statechart directory
has the same Git tree as `cursor/plugins` commit
`4dd1d05818d19792e7cd6e818f1667f95f7e1dbe` (`a79a48c6970be3b63d6efefb0a306b7c87de88d0`).
For all 33 corpus models with their original configurations, tla-go and Java
TLC 1.7.4 now agree on exit codes and the verdicts, initial and distinct state
counts, and traces read by statechart's actual parser. This comparison does not
establish SANY or full single-binary equivalence.

The hybrid test runs statechart's generated models and compares all 33 charts
and reports with the pinned corpus, using Java SANY and tla-go for TLC. A second
test routes both calls to tla-go and obtains the same 33 charts and reports with
a nonexistent JAR path; its bridge rejects all other Java invocations. Both use
the original library path and `-metadir` without copying modules. These are
test-only adapters, not a change to statechart's released backend selection.

## 1. Establish the integration baseline

- Maintain the pinned statechart corpus, Java TLC comparison and hybrid chart
  tests in CI; replace the test bridge with the final single-binary integration.
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
TLC 1.7.4. Theorem `ASSUME`/`PROVE` statements support local `NEW` declarations,
and selected hierarchical proof forms validate `SUFFICES`, `DEFINE`, `USE`,
`HIDE`, `PICK`, `TAKE`, `HAVE`, `WITNESS`, `CASE`, and step-reference scopes
against Java TLC. Relative and unnumbered step markers are also supported. Other proof forms and directives
remain unsupported; the statechart specifications have not yet been verified
for all proof forms.

Named and unnamed `INSTANCE TLC` and `INSTANCE Naturals` support their
implemented operators, including TLC runtime state and side effects. Targeted
tests compare results, failures, random behavior and DOT output with Java TLC
1.7.4; local module files retain precedence. Other standard-module instances
and the final statechart integration remain outstanding.

- The `-DTLA-Library=paths` option accepts the same platform-separated library
  paths as Java's system property. Tests compare local and library precedence,
  missing modules, paths containing spaces, and transitive imports against Java
  TLC 1.7.4. Verify remaining lookup and diagnostic differences, including
  relative paths, standard-module overrides and classpath-only modules.
- Complete named standard-module instances, including qualified operator calls
  and relevant substitutions. Retain support for user-defined modules and verify
  `X == INSTANCE TLC` with the actual statechart models.
- Verify the `USE DEF` forms used by the actual statechart models without
  changing model-checking semantics. Invalid proof directives must be rejected.
- `-metadir` creates a timestamped run directory, expands `${metadir}` in a
  DOT dump path, and removes the run directory on success. Java comparison tests
  cover this filesystem behavior, and the hybrid statechart test passes the
  actual option through. Checkpoint and recovery metadata are not implemented.
  `-workers` accepts Java TLC's positive counts and `auto` and rejects
  invalid values as Java TLC does, but exploration remains sequential.
  Complete the Java-compatible behavior of `-fp 0`, including fingerprint
  collisions.

## 3. TLC tool protocol

`-tool` emits framed messages for initial and final counts, success, common
violations, traces, and selected errors. The direct comparison above runs in CI
using statechart's real message, verdict and trace parsers. This is incomplete:
not every Java TLC message, error code, timing, or progress event is reproduced,
and the output has not yet been validated across the full TLC suite.

- Implement `-tool` message framing, codes, severity, ordering, output streams,
  and process exit behavior for the messages consumed by statechart. Use Java
  TLC as the oracle, including progress, success, failure, and counterexamples.
- Add direct protocol tests and tests using statechart's real message parser.
  Preserve normal human-readable output when `-tool` is not selected.

To run the corpus test locally, check out the pinned `cursor/plugins` revision
and set `STATECHART_DIR` to its `pstack/skills/statechart` directory and
`TLC_JAR` to the Java TLC 1.7.4 jar. Run
`mise exec bun@1.3.11 -- go test ./cmd/tla-go -run '^(TestJavaTLCStatechart(Corpus|Marks|Hybrid|Standalone)|TestJavaSANY(MetadataLevels|RealTimeMetadata|InheritedNaturalsMetadata|StatechartMetadata|TLCLevels|TLCInstanceLevels|TheoremNewLevels|HigherOrderMetadata|HigherOrderInstances|HigherOrderLevelMatrix|HigherOrderLevelConstraints))$' -count=1 -timeout=15m`.

## 4. DOT state graph

The CLI now supports `-dump` in Java TLC's text and DOT formats, `-workers`, and `-fp N` (0–130) for
single-model runs. Targeted tests compare DOT bytes with Java TLC 1.7.4 for
branching, repeated transitions, constraints, VIEW, symmetry, Unicode,
liveness stuttering and failure paths. The statechart integration tests also
compare the consumer-visible charts for the pinned corpus. This is a partial
implementation: byte-for-byte DOT equivalence for that corpus has not been
established, other dump formats and additional worker settings are unsupported,
and state identity and fingerprint collision behavior can still differ from Java TLC.

- Complete the `-dump dot,actionlabels` invocation for all statechart models,
  including Java-compatible state, transition, and value formatting.
- Compare parsed graphs against Java TLC for branching, repeated transitions,
  self-loops, constraints, special characters and Unicode. Exercise failure
  paths and ensure output and errors are handled correctly.
- Keep graph collection opt-in. Measure its runtime and memory costs on both
  small and larger state spaces, and avoid slowing ordinary checks unnecessarily.

## 5. SANY-compatible metadata

- `-sany-metadata` emits an XML projection accepted by statechart's existing
  SANY reader. It includes variable order, constants and user-defined operator
  levels; it is not SANY's complete XML semantic graph. The 33 corpus models
  compare Go's variables and exported definition levels directly with Java SANY
  1.7.4, and the no-Java bridge runs the complete chart and report tests.
- Derive levels from semantic analysis, including imports, substitutions,
  parameters, bound names and standard modules; do not guess unsupported levels.
  The current analysis handles selected higher-order operator calls but rejects
  recursive definitions; other SANY constructs and standard-module definitions
  are incomplete. Continue checking semantics and reject unsupported
  cases rather than emit misleading metadata.
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
