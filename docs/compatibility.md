# Compatibility and limitations

The goal is compatibility with the TLA+ language and the original Java TLC model checker. tla-go remains experimental: unsupported constructs and behavioral differences exist, and passing tests or a successful model check does not establish equivalence. Use Java TLC to independently verify important results.

## Model checking

Supported finite-state models include `INIT`/`NEXT`, common `SPECIFICATION` forms, invariants, state and action constraints, constants, and selected quantified and parameterized actions. The checker also supports selected temporal properties, weak and strong fairness, `VIEW`, `SYMMETRY`, counterexamples, `ALIAS`, `POSTCONDITION`, and `_POSSIBLE`.

Support is partial. Complex temporal formulas, action predicates, fairness combined with state reductions, and some recursive or effectful expressions can fail or differ from Java TLC. Counterexample selection, state ordering, fingerprinting, error messages, printed values, and exit codes are not fully compatible. TLC runtime values, side effects, random choices, and duration-dependent checks can also produce different results.

## Language and modules

The parser and evaluator support selected TLA+ expressions, operators, functions, recursive functions, and operator arguments. Module loading includes transitive `EXTENDS`, `LOCAL` definitions, and many named, parameterized, and nested `INSTANCE` forms. Some instance combinations, subexpression references, capture-avoiding substitutions, and function actions with operator parameters are not supported. Full TLC level checking and proof obligations are not implemented.

Theorem `ASSUME`/`PROVE` statements support `NEW` declarations, including their local scope, bounded constant declarations, and references in `BY` proofs. Numeric hierarchical proof steps support assertions, `SUFFICES`, `DEFINE`, `QED`, and step references, including the upstream `test207` model. Other proof forms, including automatic step names and proof-local instances, remain unsupported; proof obligations are not checked. Java TLC 1.7.4 rejects the upstream `test61` parameterized instance, whereas Java TLC 1.8.0 and tla-go both explore its five distinct states.

`ENABLED` and primed expressions support several action and operator-argument forms, including selected module-variable substitution boundaries. More complex nested instances and action expressions can differ. Java TLC versions themselves can differ on some of these cases.

The implementation provides selected functionality from `Naturals`, `Integers`, `FiniteSets`, `Sequences`, `Bags`, `TLC`, `TLCExt`, `Json`, and `Randomization`. Support for infinite sets and individual standard operators is partial. Some functions require enumerable finite values.

The CLI reads and writes JSON paths relative to its working directory. Go callers can supply `Options.ReadFile` and `Options.WriteFile`; without callbacks, reads fail as missing files and writes have no effect.

## Randomness and runtime values

The default seed is 0. `-seed` selects a different seed. The default random-subset behavior follows the tested Java TLC 1.7.4 cases; `-modern-random-subsets` selects the newer algorithm tested against the 2026-10-28 TLC development build. Other seeded behavior and constant-level caching can still differ across versions.

Selected `TLCGet` and `TLCSet` registers and configuration, trace, and runtime values are supported. Complex forms and behavior involving effects are not fully compatible.

## Go API

The `checker` package exposes `Check`, context-aware variants, and options for module loading, printing, JSON I/O, random seeds, and debugging. `Result` distinguishes successful checks from violations and selected evaluation failures; unsupported constructs and other failures can return a Go error. See [package documentation](https://pkg.go.dev/github.com/upamune/tla-go/checker) and [development and testing](development.md).

`Options.OnDebug` receives the same liveness JSON lines as the CLI's `-debug-liveness`. The events describe graph exploration, successor order, fairness components, and selected cycle witnesses. This diagnostic output does not change the model-checking result.
