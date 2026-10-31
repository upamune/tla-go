# Compatibility and limitations

The goal is compatibility with the TLA+ language and the original Java TLC model checker. tla-go remains experimental: unsupported constructs and behavioral differences exist, and passing tests or a successful model check does not establish equivalence. Use Java TLC to independently verify important results.

## Model checking

Experimental single-worker random simulation supports bounded traces, invariants, action properties, state and action constraints, and deadlock checking for selected models. With `-continue`, safety violations and deadlocks are reported and the next trace starts; initial-state violations, evaluation failures, and liveness violations still stop execution. Progress counts printed with each violation may differ from Java TLC, whose worker and reporting thread run concurrently. `-aril` advances the simulation master random stream when an explicit seed is supplied; as in Java TLC 1.7.4, negative arils and arils without an explicit seed do not advance it. Selected pure state eventualities (`<>P`), recurrence (`[]<>P`), stability (`<>[]P`), and leads-to properties (`P ~> Q`) can be combined and are checked on the sampled graph when depth is positive and neither fairness nor state reduction is used. Java TLC 1.7.4's simulation graph evaluates the stability predicate for a newly discovered state on its predecessor; tla-go reproduces this behavior, which can make the reported result differ from evaluating the printed trace directly. Java TLC 1.8.0 can report different simulation liveness results and counterexamples. Other temporal properties, fairness, DOT dumps, and liveness debugging in simulation mode are not supported. Error details, side effects, random operators, and traces are not fully compatible with Java TLC. Parallel workers and checkpoint recovery are not implemented.

For leads-to simulation, tla-go follows Java TLC 1.7.4's tableau construction: previously seen product nodes are not revisited as the sampled trace advances. Its handling of constant predicates can also report a violation for a tautology such as `FALSE ~> P`. The result can therefore differ from the logical meaning of the formula and from Java TLC 1.8.0. Other temporal combinations, effectful predicates, and all error cases are not yet supported.

`-simulate file=PREFIX` writes completed nonviolating traces in the Java TLC 1.7.4 format. Tests cover zero depth, state constraints, violations, and multiple traces; other values and side effects can still differ. Java TLC 1.8.0 includes action comments and can write a different number of states. The Go API exposes completed traces through `SimulationOptions.OnTrace`.

For selected models, model checking with `-continue` reports invariant and action-property violations and keeps exploring. As in Java TLC 1.7.4, a completed exploration can return exit code 0 even when violations were reported; deadlocks, evaluation failures, and liveness violations still terminate the check. Go API callers can inspect `Result.ContinuedViolations` or receive each violation through `Options.OnViolation`. Some evaluation-error details differ from Java TLC.

Simulation currently follows Java TLC 1.7.4 for `TLCGet("level")` and state-constraint evaluation order. In Java TLC 1.8.0, an initial invariant sees level 1 rather than 0, successor invariants see the successor level, and the tested repeated-trace model evaluates its initial constraint fewer times. These version differences are not selectable yet.

Simulation's `TLCGet("config")` follows Java TLC 1.8.0, including its record field order; Java TLC 1.7.4 treats this key as undefined during simulation. Its `install` field is `"unknown"` in tla-go rather than the path to the Java JAR.

Simulation's `TLCSet("exit", TRUE)` follows Java TLC 1.8.0 for the tested single-worker cases. Java TLC 1.7.4 instead throws an exception when this value is set during simulation. Stopping and error reporting can involve concurrent workers in Java, so all combinations of side effects and errors are not yet covered.

Selected constant configured invariants, properties, and constraints are evaluated before assumptions. `FALSE` is reported as a configuration error and `TRUE` is not reevaluated during exploration. Precomputation involving TLC runtime operations, diagnostics for non-boolean expressions, and evaluation failures can still differ from Java TLC.

Selected non-boolean invariant and property failures report Java TLC's operator, value kind, source location, and exit code, including Boolean operators, conditionals, and quantifiers. Nested evaluation stacks, replayed side effects, and other contexts are not fully compatible.

When a non-boolean expression is encountered while generating initial states in simulation, tla-go reports message 2247 directly as Java TLC 1.7.4 does. Java TLC 1.8.0 nests that message inside a general error instead.

Supported finite-state models include `INIT`/`NEXT`, common `SPECIFICATION` forms, invariants, state and action constraints, constants, and selected quantified and parameterized actions. The checker also supports selected temporal properties, weak and strong fairness, `VIEW`, `SYMMETRY`, counterexamples, `ALIAS`, `POSTCONDITION`, and `_POSSIBLE`.

Support is partial. Complex temporal formulas, action predicates, fairness combined with state reductions, and some recursive or effectful expressions can fail or differ from Java TLC. Counterexample selection, state ordering, fingerprinting, error messages, printed values, and exit codes are not fully compatible. TLC runtime values, side effects, random choices, and duration-dependent checks can also produce different results.

State-only `SPECIFICATION` predicates can be checked without a next-state action. As in Java TLC, invariants and temporal properties require a next-state predicate; otherwise TLC checks initial states before reporting that no next action exists. Next-state assignments can also use zero-argument aliases such as `a == x'`; upstream `AS`, `C`, and `CoverageStatistics` models cover these forms.

After a successor invariant or action-property evaluation error, Java TLC replays successors to collect an evaluation stack. tla-go reproduces the resulting state counts, exit code, and stack for selected cases, including the upstream `BasicMultiTrace` model. Replay of effectful expressions, other errors, and all possible state reductions is not yet fully verified.

For symmetry-reduced models, selected pure Boolean combinations of recurrence properties use Java TLC's no-tableau successor ordering and transition handling. Selected state-predicate tableau and leads-to properties also regenerate successors when a tableau node appears after its reduced state was explored. Nine upstream symmetry models and additional weak- and strong-fairness cases are compared against Java TLC. Tests also compare raw tool traces for the two `ChooseTableauSymmetry` models and selected temporal properties with and without fairness, including subscripts that name individual variables. Other tableau formulas and reductions remain incomplete, and counterexample traces can differ.

Like Java TLC, model checking warns before checking assumptions when temporal properties are combined with `SYMMETRY` (message 2279) or with state or action constraints (message 2284). `-nowarning` suppresses these warnings, and simulation does not report them. Properties are classified as temporal following TLC's configuration rules for the tested forms; unusual property expressions can still be classified differently.

As in Java TLC, `AXIOM` declarations are not evaluated during the initial assumption check. Named axioms can still be referenced by the model.

Fingerprint failures report Java TLC-style tool messages for the tested initial and successor states. Intersections with explicit sets or intervals use Java TLC's reduction order, including for nested power sets. Other lazy-set failures can still evaluate operands in a different order, producing a different underlying error or source location.

Finite `SUBSET` values can be stored in states without first materializing the entire power set for the state key. The exact key still grows with the number of subsets, so large power sets can require substantial time and memory. The upstream `Bug279` model exercises a 20-element base and recursive operators.

Temporal formulas used in an initial predicate, next-state action, or constraint are evaluated only when reached. Like Java TLC, tla-go reports an evaluation error when one is reached, and accepts an unused temporal `NEXT` when there are no initial states. SANY-style level checks reject Boolean operators that combine an action with a temporal formula. Other temporal level rules and error details remain incomplete.

Java TLC 1.7.4 and 1.8.0 differ on some temporal formulas. For example, `~(\E i \in {} : <>[](x = i))` produces a tautology error in 1.7.4 and a temporal-action error in 1.8.0; tla-go currently follows the 1.7.4 result for this case.

Configuration syntax errors also differ: Java TLC 1.7.4 wraps them in an unexpected exception and exits 255; Java TLC 1.8.0 emits the configuration error directly and exits 151. tla-go currently follows the 1.7.4 behavior.

Java TLC 1.7.4's configuration lexer treats some invalid tokens as end of input. tla-go follows this behavior, so an invalid character or unfinished comment after valid directives may leave those directives in effect. Configuration syntax and diagnostics are still not fully compatible.

## Language and modules

The parser and evaluator support selected TLA+ expressions, operators, functions, recursive functions, and operator arguments. Module loading includes transitive `EXTENDS`, `LOCAL` definitions, and many named, parameterized, and nested `INSTANCE` forms. Substitutions into constant modules support selected state, action, and temporal expressions, with level constraints checked through operator calls; the upstream `test213` now matches Java TLC's exit code. Function actions in parameterized instances support selected operator arguments, including lambdas and local operators. Some instance combinations, subexpression references, and capture-avoiding substitutions remain unsupported or can be rejected more strictly than Java TLC. Full TLC level checking and proof obligations are not implemented.

Theorem `ASSUME`/`PROVE` statements support `NEW` declarations and the `CONSTANT`, `STATE`, `ACTION`, and `TEMPORAL` forms, including their local scope, bounded constant declarations, and references in `BY` proofs. Hierarchical proof steps support assertions, `SUFFICES`, `DEFINE` and implicit definitions, `USE`, `HIDE`, `PICK`, `TAKE`, `HAVE`, `WITNESS`, `CASE`, `QED`, step references and selected subexpressions, and relative or unnumbered step markers, including the upstream `test207` model. Other proof forms, including proof-local instances, remain unsupported; proof obligations are not checked. Java TLC 1.7.4 rejects the upstream `test61` parameterized instance, whereas Java TLC 1.8.0 and tla-go both explore its five distinct states.

`ENABLED` and primed expressions support several action and operator-argument forms, including selected module-variable substitution boundaries. More complex nested instances and action expressions can differ. Java TLC versions themselves can differ on some of these cases.

The implementation provides selected functionality from `Naturals`, `Integers`, `FiniteSets`, `Sequences`, `Bags`, `TLC`, `TLCExt`, `Json`, and `Randomization`. The Java TLC 1.7.4 sources for `Reals` and `RealTime` are embedded and can also be supplied as local modules. Standard imports used inside an `INSTANCE` retain their own bindings; inherited operators and substitutions are not yet fully supported for every standard module. Support for infinite sets and individual standard operators is partial. Some functions require enumerable finite values.

The CLI reads and writes JSON paths relative to its working directory. Go callers can supply `Options.ReadFile` and `Options.WriteFile`; without callbacks, reads fail as missing files and writes have no effect.

## Randomness and runtime values

The default seed is 0. `-seed` selects a different seed. The default random-subset behavior follows the tested Java TLC 1.7.4 cases; `-modern-random-subsets` selects the newer algorithm tested against Java TLC 1.8.0 and the 2026-10-28 development build. Other seeded behavior and constant-level caching can still differ across versions.

Selected `TLCGet` and `TLCSet` registers and configuration, trace, and runtime values are supported. Complex forms and behavior involving effects are not fully compatible.

## Command-line options

`-difftrace` follows Java TLC 1.7.4: a model-checking trace prints its last state in full unless that state is a checked successor of an initial state, while liveness and simulation traces print every later state as a difference. `-terse` renders `SUBSET`, function sets, record sets, and Cartesian products as formulas in `Print`, `PrintT`, and traces. Java TLC also leaves some functions and other lazily evaluated sets unexpanded and applies `-terse` to `ToString`, `Assert`, and error messages; tla-go does not. As in Java TLC, `-userFile` writes `PrintT` values to the file without line wrapping.

`-fpmem`, `-fpbits`, `-gzip`, `-modelcheck`, and `-debug` are validated as Java TLC validates them but have no other effect; Java's internal debugging output is not reproduced. Invalid option values exit with code 1 and Java TLC's message, which tla-go writes to standard error without Java's version banner and usage line.

## Go API

The `checker` package exposes `Check`, context-aware variants, and options for module loading, printing, JSON I/O, random seeds, and debugging. `Result` distinguishes successful checks from violations and selected evaluation failures; unsupported constructs and other failures can return a Go error. See [package documentation](https://pkg.go.dev/github.com/upamune/tla-go/checker) and [development and testing](development.md).

`Options.OnDebug` receives the same liveness JSON lines as the CLI's `-debug-liveness`. The events describe graph exploration, successor order, fairness components, and selected cycle witnesses. This diagnostic output does not change the model-checking result.
