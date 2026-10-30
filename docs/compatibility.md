# Compatibility and limitations

The goal is compatibility with the TLA+ language and the original Java TLC model checker. tla-go remains experimental: unsupported constructs and behavioral differences exist, and passing tests or a successful model check does not establish equivalence. Use Java TLC to independently verify important results.

## Model checking

Supported finite-state models include `INIT`/`NEXT`, common `SPECIFICATION` forms, invariants, state and action constraints, constants, and selected quantified and parameterized actions. The checker also supports selected temporal properties, weak and strong fairness, `VIEW`, `SYMMETRY`, counterexamples, `ALIAS`, `POSTCONDITION`, and `_POSSIBLE`.

Support is partial. Complex temporal formulas, action predicates, fairness combined with state reductions, and some recursive or effectful expressions can fail or differ from Java TLC. Counterexample selection, state ordering, fingerprinting, error messages, printed values, and exit codes are not fully compatible. TLC runtime values, side effects, random choices, and duration-dependent checks can also produce different results.

Fingerprint failures report Java TLC-style tool messages for the tested initial and successor states. Intersections with explicit sets or intervals use Java TLC's reduction order, including for nested power sets. Other lazy-set failures can still evaluate operands in a different order, producing a different underlying error or source location.

Temporal formulas used in an initial predicate, next-state action, or constraint are evaluated only when reached. Like Java TLC, tla-go reports an evaluation error when one is reached, and accepts an unused temporal `NEXT` when there are no initial states. SANY-style level checks reject Boolean operators that combine an action with a temporal formula. Other temporal level rules and error details remain incomplete.

Java TLC 1.7.4 and 1.8.0 differ on some temporal formulas. For example, `~(\E i \in {} : <>[](x = i))` produces a tautology error in 1.7.4 and a temporal-action error in 1.8.0; tla-go currently follows the 1.7.4 result for this case.

Configuration syntax errors also differ: Java TLC 1.7.4 wraps them in an unexpected exception and exits 255; Java TLC 1.8.0 emits the configuration error directly and exits 151. tla-go currently follows the 1.7.4 behavior.

Java TLC 1.7.4's configuration lexer treats some invalid tokens as end of input. tla-go follows this behavior, so an invalid character or unfinished comment after valid directives may leave those directives in effect. Configuration syntax and diagnostics are still not fully compatible.

## Language and modules

The parser and evaluator support selected TLA+ expressions, operators, functions, recursive functions, and operator arguments. Module loading includes transitive `EXTENDS`, `LOCAL` definitions, and many named, parameterized, and nested `INSTANCE` forms. Substitutions into constant modules support selected state, action, and temporal expressions, with level constraints checked through operator calls; the upstream `test213` now matches Java TLC's exit code. Some instance combinations, subexpression references, capture-avoiding substitutions, and function actions with operator parameters remain unsupported or can be rejected more strictly than Java TLC. Full TLC level checking and proof obligations are not implemented.

Theorem `ASSUME`/`PROVE` statements support `NEW` declarations, including their local scope, bounded constant declarations, and references in `BY` proofs. Hierarchical proof steps support assertions, `SUFFICES`, `DEFINE`, `USE`, `HIDE`, `PICK`, `TAKE`, `HAVE`, `WITNESS`, `CASE`, `QED`, step references, and relative or unnumbered step markers, including the upstream `test207` model. Other proof forms, including proof-local instances, remain unsupported; proof obligations are not checked. Java TLC 1.7.4 rejects the upstream `test61` parameterized instance, whereas Java TLC 1.8.0 and tla-go both explore its five distinct states.

`ENABLED` and primed expressions support several action and operator-argument forms, including selected module-variable substitution boundaries. More complex nested instances and action expressions can differ. Java TLC versions themselves can differ on some of these cases.

The implementation provides selected functionality from `Naturals`, `Integers`, `FiniteSets`, `Sequences`, `Bags`, `TLC`, `TLCExt`, `Json`, and `Randomization`. The Java TLC 1.7.4 sources for `Reals` and `RealTime` are embedded and can also be supplied as local modules. Standard imports used inside an `INSTANCE` retain their own bindings; inherited operators and substitutions are not yet fully supported for every standard module. Support for infinite sets and individual standard operators is partial. Some functions require enumerable finite values.

The CLI reads and writes JSON paths relative to its working directory. Go callers can supply `Options.ReadFile` and `Options.WriteFile`; without callbacks, reads fail as missing files and writes have no effect.

## Randomness and runtime values

The default seed is 0. `-seed` selects a different seed. The default random-subset behavior follows the tested Java TLC 1.7.4 cases; `-modern-random-subsets` selects the newer algorithm tested against the 2026-10-28 TLC development build. Other seeded behavior and constant-level caching can still differ across versions.

Selected `TLCGet` and `TLCSet` registers and configuration, trace, and runtime values are supported. Complex forms and behavior involving effects are not fully compatible.

## Go API

The `checker` package exposes `Check`, context-aware variants, and options for module loading, printing, JSON I/O, random seeds, and debugging. `Result` distinguishes successful checks from violations and selected evaluation failures; unsupported constructs and other failures can return a Go error. See [package documentation](https://pkg.go.dev/github.com/upamune/tla-go/checker) and [development and testing](development.md).

`Options.OnDebug` receives the same liveness JSON lines as the CLI's `-debug-liveness`. The events describe graph exploration, successor order, fairness components, and selected cycle witnesses. This diagnostic output does not change the model-checking result.
