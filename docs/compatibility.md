# Compatibility and limitations

The goal is compatibility with the TLA+ language and the original Java TLC model checker. tla-go remains experimental: unsupported constructs and behavioral differences exist, and passing tests or a successful model check does not establish equivalence. Use Java TLC to independently verify important results.

## Semantic validation

Java SANY 1.7.4 rejects references to a user-defined operator before its declaration, including non-recursive self references and forward references inside `LET`, even when the containing definition is never evaluated. tla-go rejects non-recursive self references in ordinary operator definitions, including local definitions and operator arguments. It still accepts some forward references: for example, `Bad == Later(1)` followed by `Later(x) == x`, or `LET F == G G == 1 IN F`. Self references in generated instance definitions are also not checked by this rule. Successful model checking therefore does not establish that SANY accepts the module. Declared recursion, local names, imported modules, instance substitutions, and generated definitions require scope-aware declaration ordering; a line-number comparison on linked definitions is insufficient. The original small models in `TestJavaTLCNameResolutionOrder` compare SANY and TLC tool mode against the pinned 1.7.4 JAR, with valid controls and known gaps.
Java SANY also rejects a `RECURSIVE` declaration whose operator is never defined or whose declared arity differs from its definition; tla-go can accept both cases.

## Model checking

Experimental single-worker random simulation supports bounded traces, invariants, action properties, state and action constraints, and deadlock checking for selected models. With `-continue`, safety violations and deadlocks are reported and the next trace starts; initial-state violations, evaluation failures, and liveness violations still stop execution. Progress counts printed with each violation may differ from Java TLC, whose worker and reporting thread run concurrently. `-aril` advances the simulation master random stream when an explicit seed is supplied; as in Java TLC 1.7.4, negative arils and arils without an explicit seed do not advance it. Temporal properties and the `WF`, `SF`, and `[]<>` fairness conjuncts of the specification are checked at the end of each sampled trace, following Java TLC 1.7.4's `LiveCheck1`: the properties are negated and conjoined with the fairness conditions, put in TLC's disjunctive normal form, and each resulting solution is checked on a behavior graph of the trace in which every state can stutter. Liveness is thus checked on the sampled trace only. In the tested cases, tla-go follows its tableau, graph construction, strongly connected component search, and counterexample output, including recovering each printed state and its action from the previous one with `TLCGet("level")` at that state's position, merging states under `VIEW` and `SYMMETRY`, and the exceptions and sampled behavior that Java TLC reports when a state cannot be recovered, a check cannot be evaluated, or the depth is 0. This construction can disagree with the logical meaning of a formula and with evaluating the printed trace directly: for example, without a tableau a newly discovered state receives the state predicates of its predecessor, and with a tableau a revisited state ends the graph. Java TLC 1.8.0 can report different simulation liveness results and counterexamples. TLC splits a property only at conjunctions and zero-arity definitions, so in simulation a `[]P` below a quantifier or a parameterized operator is checked as a temporal property and `[][A]_v` there is rejected, as in Java TLC; model checking still treats these as an invariant and an action property. Java TLC also treats other temporal conjuncts of the specification, such as `<>P`, as liveness assumptions; tla-go rejects them as an unsupported `SPECIFICATION` form. Properties and fairness that use `TLCGet`, state dumps, and liveness debugging in simulation mode are not supported, and side effects of temporal predicates are not compared. Records in counterexamples are printed with sorted fields, whereas Java TLC prints a recovered state's records in the order in which they were built; long values are laid out over several lines in both. A failed `Assert` while generating successors is reported as Java TLC 1.7.4 reports it, with the behavior so far and statistics, as in the upstream `etest8` model; so are `-maxSetSize` failures. As in Java TLC, a non-boolean action or a failed Java override such as `SelectSeq` is followed by the behavior but not by the statistics. Other error details, side effects, random operators, and traces are not fully compatible with Java TLC. Simulation accepts only one worker. Model checking accepts Java TLC's `-workers` values, including `auto`, and rejects invalid values with Java TLC's messages and exit code 1, but it explores states sequentially. Its `-tool` mode message reports the requested worker count. Complete searches therefore report Java TLC's state counts, while statistics and counterexamples at a violation can differ from a parallel Java TLC run. `-checkpoint` is validated like Java TLC's option, including its 32-bit millisecond overflow, but checkpoints are not written and checkpoint recovery is not implemented. Invalid options are reported as Java TLC's `printErrorMsg` reports them, on standard output with exit code 1: as message 1102 with its usage line when `-tool` precedes the option, and otherwise as `Error: ` followed by the message and the usage line, without Java TLC's version banner. As in Java TLC, an unwritable `-dump` file is reported as an invalid option.

`-simulate file=PREFIX` writes completed nonviolating traces in the Java TLC 1.7.4 format, through `VIEW` under `-view`. Tests cover zero depth, state constraints, violations, and multiple traces; other values and side effects can still differ. Java TLC 1.8.0 includes action comments and can write a different number of states. The Go API exposes completed traces through `SimulationOptions.OnTrace`.

For selected models, model checking with `-continue` reports invariant and action-property violations and keeps exploring. As in Java TLC 1.7.4, a completed exploration can return exit code 0 even when violations were reported; deadlocks, evaluation failures, and liveness violations still terminate the check. Go API callers can inspect `Result.ContinuedViolations` or receive each violation through `Options.OnViolation`. Some evaluation-error details differ from Java TLC.

Experimental depth-first iterative deepening (`-dfid N`, `Options.DepthFirst`) follows Java TLC 1.7.4's single-worker `DFIDModelChecker`. Each iteration restarts from the initial states, expands only states first found at a shallower level, and reports TLC's level messages and its final progress and summary messages, including the collision probability based on the closest pair of state fingerprints. TLC's timed progress reports during an iteration, first after three seconds, are not printed. TLC reports success only in the iteration after the search ends, so a search that reaches depth N first exits with code 0 without the success message; the Go API returns `DepthLimit`. Below depth 2, TLC stops after the initial states with exit code 255 and no message. After an error, TLC finishes the search below the current initial state before stopping, so later `Print` output and counts follow the error report. Error traces are TLC's search stack, without action labels; like TLC, an error found while expanding a later initial state shows that state twice. Java TLC chooses initial states and successors at random, so its traces and generated counts can vary between runs when several states are eligible. tla-go always takes the first eligible state in generation order, which is one of TLC's possible choices. Tests compare the upstream `DepthFirstDieHardTest`, `DepthFirstErrorTraceTest`, and `DepthFirstTerminate` models with one worker, and small models with deadlocks, depth limits, invariants, action properties, constraints, `-continue`, and evaluation errors, with Java TLC. As in Java TLC, `TLCGet("level")` is 0 and `TLCGet("diameter")`, `"distinct"`, and `"queue"` are -1 in this mode; `"generated"` and the counts in `"stats"`, which Java TLC 1.7.4 does not define, are -1 as in Java TLC 1.8.0; `TLCSet("exit", TRUE)` fails, and `TLCSet("pause", TRUE)` has no effect. As in Java TLC, more than one worker is rejected in this mode with exit code 255. `POSTCONDITION`, `_POSSIBLE`, state dumps (`-dump`), liveness debugging, checkpoints, and coverage are not supported with `-dfid`, and tla-go does not apply `ALIAS` to its DFID traces. Java TLC 1.7.4 rejects the `ALIAS` configuration keyword entirely; Java TLC 1.8.0 (revision `090e85c31a6d7fb3d34a33a9f1a46310cdc2aa7f`) accepts `ALIAS` but does not apply it to DFID traces for invariant violations, deadlocks, and next-state evaluation errors. These cases are compared in both normal and `-tool` output, with a non-alias control. As in Java TLC, `-view` and `-terse` apply to depth-first traces but `-difftrace` does not. When a successor leaves a variable unassigned, the generated count includes only the successors of its action generated before it, while TLC counts the whole action. Unexpected exceptions name TLC's Java exception class and nest coded messages in tool output as TLC does. Nested evaluation stacks after replayed errors list only the expressions enclosing the error, where the evaluator records their positions; TLC also lists conjuncts evaluated before the error, and after an error in the initial predicate it prints the name of an internal object instead. The fingerprint-based probability is omitted when a state cannot be fingerprinted like TLC.

Simulation currently follows Java TLC 1.7.4 for `TLCGet("level")` and state-constraint evaluation order. In Java TLC 1.8.0, an initial invariant sees level 1 rather than 0, successor invariants see the successor level, and the tested repeated-trace model evaluates its initial constraint fewer times. These version differences are not selectable yet.

Simulation's `TLCGet("config")` follows Java TLC 1.8.0, including its record field order; Java TLC 1.7.4 treats this key as undefined during simulation. Model checking's record also follows Java TLC 1.8.0 and reports the requested `-workers` count, although exploration is sequential; Java TLC 1.7.4 treats the key as undefined there as well. Its `install` field is `"unknown"` in tla-go rather than the path to the Java JAR.

Simulation's `TLCSet("exit", TRUE)` follows Java TLC 1.8.0 for the tested single-worker cases. Java TLC 1.7.4 instead throws an exception when this value is set during simulation. Stopping and error reporting can involve concurrent workers in Java, so all combinations of side effects and errors are not yet covered.

`-view` prints trace states through `VIEW` as Java TLC 1.7.4 does: a tuple-valued view is labeled by the constants and variables it mentions, in variable order, even when that order differs from the tuple's, and other values are printed directly, as in the upstream `ViewMapTest`. State dumps print states through `VIEW` as well. With `-difftrace`, states printed as differences list the changed variables instead, as Java TLC's `TLCStateMut.toString(TLCState)` does, while the states printed in full go through `VIEW`. States in other messages, such as an error's initial state, are still printed in full.

`-maxSetSize` follows Java TLC 1.7.4 for the values that Java TLC builds element by element: set maps (`{e : x \in S}`) and filters over enumerated sets and intervals, `\cup`, `\cap`, and `\` when Java TLC reduces them eagerly (as for intervals and enumerated sets), `UNION` of enumerated sets, `SelectSeq`, `@@`, `BagUnion`, `BagOfAll`, and `Cardinality` of a deferred union, set operation, or filter. As in Java TLC, a value fails only when it needs more elements than both the bound and 10, duplicate map results count toward the limit, and `SUBSET` and set operations involving other deferred sets stay unlimited while they are evaluated. Tests compare the error, exit code, and evaluation stack with Java TLC for selected expressions in initial states, successors, invariants, constraints, and assumptions. Java TLC also limits deferred sets, such as `[S -> T]`, `S \X T`, or `UNION {1..10, 11..20}`, when it fingerprints a state that contains them, and counts duplicate elements when it enumerates a deferred union; tla-go does neither yet. An evaluation error in a temporal property, including a `-maxSetSize` failure, is reported after the initial states have been computed, with exit code 255; Java TLC reports one raised on an initial state while computing the initial states, as message 2102, and exits with code 0.

As in Java TLC, a configured `INIT` or `NEXT` that reads no variable is rejected as a constant (message 2233), also in a model without variables; such operators reached through `SPECIFICATION` are not yet reported as Java TLC reports them. In `-tool` mode, initial-state generation reports Java TLC's progress messages (2269) before the third, fifth, ninth, and later initial states; the plain output keeps the format of the tla-ts fixtures and omits Java TLC's initial-state messages.

Selected constant configured invariants, properties, and constraints are evaluated before assumptions. `FALSE` is reported as a configuration error and `TRUE` is not reevaluated during exploration. Precomputation involving TLC runtime operations, diagnostics for non-boolean expressions, and evaluation failures can still differ from Java TLC.

Selected non-boolean invariant and property failures report Java TLC's operator, value kind, source location, and exit code, including Boolean operators, conditionals, and quantifiers. Nested evaluation stacks, replayed side effects, and other contexts are not fully compatible.

When a non-boolean expression is encountered while generating initial states in simulation, tla-go reports message 2247 directly as Java TLC 1.7.4 does. Java TLC 1.8.0 nests that message inside a general error instead.

Supported finite-state models include `INIT`/`NEXT`, common `SPECIFICATION` forms, invariants, state and action constraints, constants, and selected quantified and parameterized actions. The checker also supports selected temporal properties, weak and strong fairness, `VIEW`, `SYMMETRY`, counterexamples, `ALIAS`, `POSTCONDITION`, and `_POSSIBLE`.

Support is partial. Complex temporal formulas, action predicates, fairness combined with state reductions, and some recursive or effectful expressions can fail or differ from Java TLC. Counterexample selection, state ordering, fingerprinting, error messages, printed values, and exit codes are not fully compatible. TLC runtime values, side effects, random choices, and duration-dependent checks can also produce different results.

For the tested quantified safety properties, model checking follows Java TLC 1.7.4's classification: `\A v \in S : []P(v)` is a temporal property, and a quantified `[][A]_v` is rejected with message 2214 when `S` is nonempty. Java TLC 1.8.0 classifies these tested forms as state and action safety properties instead, which can change the exit code and counterexample.

In breadth-first `-tool` output, tla-go reports liveness solution counts (message 2212) for pure formulas supported by its liveness normalizer. It omits this diagnostic for effectful expressions, formulas the normalizer cannot convert, and formulas whose estimated normal form exceeds 1,024 terms; the checking result is unaffected by the diagnostic.

Successors are generated and labeled by next-state action as in Java TLC's `Tool.getActions`, which splits disjunctions and existentials with constant bounds, binding the first bound fastest. Like TLC, tla-go does not split a definition from an `INSTANCE` of a module with constants or variables, and labels it with the `INSTANCE` statement. A next-state relation whose disjuncts are all existentials with an empty constant bound has no action, which TLC reports after the initial states. Other splitting rules, such as for operators with constant arguments, are covered only for tested forms.

State-only `SPECIFICATION` predicates can be checked without a next-state action. As in Java TLC, invariants and temporal properties require a next-state predicate; otherwise TLC checks initial states before reporting that no next action exists. Next-state assignments can also use zero-argument aliases such as `a == x'`; upstream `AS`, `C`, and `CoverageStatistics` models cover these forms.

After a successor invariant or action-property evaluation error, Java TLC replays successors to collect an evaluation stack. tla-go reproduces the resulting state counts, exit code, and stack for selected cases, including the upstream `BasicMultiTrace` model. Replay of effectful expressions, other errors, and all possible state reductions is not yet fully verified. Initial-state and successor evaluation errors report Java TLC's expression stack for selected cases, as in the upstream `etest4` model: earlier conjuncts stay on the stack, inline conjunctions are nested as SANY parses them, and top-level disjunctions and existential quantifiers of an action are split off as Java TLC's actions are. Applying a non-function reports Java TLC's message. Java TLC re-evaluates the initial predicate to collect this stack, repeating side effects such as `Print`; tla-go does not repeat them.

For symmetry-reduced models, selected pure Boolean combinations of recurrence properties use Java TLC's no-tableau successor ordering and transition handling. Selected state-predicate tableau and leads-to properties also regenerate successors when a tableau node appears after its reduced state was explored. Nine upstream symmetry models and additional weak- and strong-fairness cases are compared against Java TLC. Tests also compare raw tool traces for the two `ChooseTableauSymmetry` models and selected temporal properties with and without fairness, including subscripts that name individual variables. Other tableau formulas and reductions remain incomplete, and counterexample traces can differ.

Like Java TLC, model checking warns before checking assumptions, before rejecting a temporal tautology (message 2253), and in depth-first mode before rejecting the liveness properties themselves, when temporal properties are combined with `SYMMETRY` (message 2279) or with state or action constraints (message 2284). `-nowarning` suppresses these warnings, and simulation does not report them. Properties are classified as temporal following TLC's configuration rules for the tested forms; unusual property expressions can still be classified differently.

`-dump` writes Java TLC 1.7.4's text and DOT formats, including colored edges and the action legend; the upstream `DumpAsDotTest` and `Github407Test` outputs are reproduced byte for byte. As in Java TLC, the text format repeats a state for each stuttering step recorded while checking liveness, an initial state is written before it is checked, states buffered before an initial invariant violation are not written, leaving only the DOT header, and DOT node identifiers are fingerprints of the `VIEW` value when one is configured. DOT snapshots are not supported.

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

As in Java TLC, an empty function that was not written as a tuple, such as `[x \in {} |-> 0]` or the element of `[{} -> S]`, prints as `<< >>`, while an empty tuple or sequence prints as `<<>>`.

The CLI reads and writes JSON paths relative to its working directory. Go callers can supply `Options.ReadFile` and `Options.WriteFile`; without callbacks, reads fail as missing files and writes have no effect.

## PlusCal translation

`tla-go -translate` ports `pcal.trans` 1.11 from the Java TLA+ tools at the pinned commit, including its error messages, automatic labels, checksums, and `.cfg` updates. Tests compare exit codes, standard output, and every written file with Java TLA+ tools 1.7.4 for 1134 runs: the 106 upstream files that contain an algorithm, the inline sources of the upstream `pcal` JUnit tests, randomly mutated upstream algorithms, and inputs written for tla-go, under several option sets. All of them currently match. Other inputs can still differ; for example, internal Java failures that the tests do not reach are reported differently.

The translator does not produce the TLA+-to-PlusCal source mapping that the Toolbox uses, and it rejects `-spec`, `-myspec`, `-spec2`, and `-myspec2`, which run TLC on PlusCal's TLA+ specification. Java prints messages in the locale's charset, so non-ASCII characters can appear as `?` under the C locale; tla-go always writes UTF-8. Java's `-unixEOL` has no effect on current Java versions, which write the platform's line ends; tla-go writes `\n` with `-unixEOL` and the platform's line ends otherwise. Like Java, tla-go counts columns in UTF-16 code units, so a character outside the Basic Multilingual Plane counts as two columns, and an error message that quotes half of such a character shows `?`.

Because the translations of the tested models are identical, tla-go checks them exactly as it checks the translations Java produces.

## Randomness and runtime values

The default seed is 0. `-seed` selects a different seed. The default random-subset behavior follows the tested Java TLC 1.7.4 cases; `-modern-random-subsets` selects the newer algorithm tested against Java TLC 1.8.0 and the 2026-10-28 development build. Other seeded behavior and constant-level caching can still differ across versions.

Selected `TLCGet` and `TLCSet` registers and configuration, trace, and runtime values are supported. Complex forms and behavior involving effects are not fully compatible.

## Command-line options and warnings

Model checking and simulation report Java TLC's warning 2300 when a symmetry set has fewer than two elements, naming the set as Java does, and warning 2144 when an `EXCEPT` path is outside a function's domain, in which case that update is skipped. As in Java TLC, the whole path is checked, also for lazily evaluated functions, and constant definitions containing `EXCEPT` are evaluated before the initial states, so their warnings come first. `-nowarning` suppresses both warnings.

As in Java TLC, trace states, simulation trace files, DOT state labels, and `SpecTE.tla` lay out values wider than 80 columns over several lines.

`-difftrace` follows Java TLC 1.7.4: a model-checking trace prints its last state in full unless that state is a checked successor of an initial state, while liveness and simulation traces, including the part of a simulation counterexample printed before an exception, print every later state as a difference. `-terse` renders `SUBSET`, function sets, record sets, and Cartesian products as formulas in `Print`, `PrintT`, traces, `VIEW` text, state dumps, DOT labels, and simulation trace files. A `SUBSET` of an interval prints as `SUBSET 1..2`, as in Java TLC 1.7.4; Java TLC 1.8.0 adds parentheses. The same rendering applies to `ToString` and failed `Assert` values, including nested sets; it avoids calculating the cardinality of a symbolic set such as `SUBSET Nat`. A lazy function over `Nat` remains a description in these contexts, and evaluating a finite function can still fail before rendering. Error contexts differ: comparing `SUBSET {1, 2}` with `1` fails with the expanded set in Java TLC both with and without `-terse`; tla-go currently reports a different comparison error. Other error messages and some other lazily evaluated values do not yet fully follow Java TLC's `-terse` behavior. For example, the ordinary `ToString(SUBSET Nat)` error text differs from Java TLC, while both reject it; with `-terse` both return `SUBSET Nat`. As in Java TLC, `-userFile` writes `PrintT` values to the file without line wrapping.

`-fpmem`, `-fpbits`, `-gzip`, `-modelcheck`, and `-debug` are validated as Java TLC validates them but have no other effect; Java's internal debugging output is not reproduced. Invalid option values exit with code 1 and Java TLC's message, which is reported like other invalid options.

`-generateSpecTE` follows Java TLC 1.7.4's trace-exploration writer for invariant, deadlock, action-property, evaluation-error, and liveness traces, including stuttering and back-to-state loops, model-value constants, and embedded modules. `SpecTE.tla` begins with tla-go's own tool output, and its `def_ov_` identifier and generation date depend on the time, so the files are never identical to Java's; for the tested models they otherwise match. Java TLC writes the files while another thread is still parsing its output, so it can omit trailing states or stop responding; tla-go always uses the whole trace. When the first error has no trace, both report that the output could not be parsed and write no files. Java orders the `EXTENDS` list by internal SANY node identifiers, which tla-go approximates, so names that share a hash bucket, such as `Naturals` and `TLC`, can appear in a different order. tla-go copies the configuration it checked, whereas Java TLC copies `SPEC.cfg` even with `-config`, and it writes full states even with `-difftrace` or `-terse`. Simulation and depth-first mode write no files: Java TLC's simulator never writes them, and in depth-first mode its output parser fails on the trace states, so both report that the output could not be parsed. In depth-first mode Java TLC then also reports a `NullPointerException` from the SpecTE writer and stops responding, which tla-go does not reproduce. tla-go cannot check the generated files yet because it lacks Java's `Toolbox` module; Java TLC can.

## Go API

The `checker` package exposes `Check`, context-aware variants, and options for module loading, printing, JSON I/O, random seeds, and debugging. `Result` distinguishes successful checks from violations and selected evaluation failures; unsupported constructs and other failures can return a Go error. See [package documentation](https://pkg.go.dev/github.com/upamune/tla-go/checker) and [development and testing](development.md).

`Options.OnDebug` receives the same liveness JSON lines as the CLI's `-debug-liveness`. The events describe graph exploration, successor order, fairness components, and selected cycle witnesses. This diagnostic output does not change the model-checking result.

## Experimental coverage

`-coverage N` collects action and expression counts during breadth-first
exploration. It is compared line-for-line with Java TLC 1.7.4 on the upstream
`CoverageStatistics` model, including a changed state constraint. The first
number on an action is the number of new in-model states and the second is
its generated-state count; expression costs follow TLC for the tested forms.
Zero-count expressions are included.

An implicit variable-free model with no INIT, NEXT, or SPECIFICATION and no
invariants or constraints also supports coverage: like Java TLC 1.7.4's
`EmptyTest`, it reports zero states and no coverage section. Assumptions are
still evaluated normally. Explicit variable-free specifications can have
coverage actions and remain unsupported by this empty-model case.

Coverage is restricted to a named initial predicate and a disjunction of
named, nullary actions with simple conjunctions, assignments, comparisons,
`\in Nat` guards, range membership assignments, and `UNCHANGED`. Range
assignment coverage counts enumerated members and domain evaluations separately;
independent, dependent, singleton, and empty initial ranges and next-state
range assignments have Java 1.7.4 differential fixtures. State invariants
support a limited expression subset: conjunctions and binary disjunctions,
comparisons and membership with simple operands, and nested single-bound
quantifiers over literal integer ranges. Counts are recorded during evaluation,
including short-circuiting. Invariants also support selected calls to nullary
state operators and operators with literal numeric arguments; repeated calls to
the same definition and other unmodeled call forms are rejected. The property
fixtures include direct Java 1.7.4 comparisons and saved reports.
Selected invariants may also quantify over `SUBSET` of a literal integer
range and then quantify over each resulting subset. The reported secondary
enumeration cost includes TLC's prefetch behavior; empty sets, repeated
evaluation, and short-circuiting have Java 1.7.4 differential fixtures.
One supported quantified action shape has a single preceding `x < n` guard and
a final `\E i \in a..b : x' = i`, for a single variable and literal bounds.
For evaluation failures, failures after this existential has been evaluated,
or failures with a successor trace, coverage is omitted because TLC can charge
replay to its cost model; the diagnostic and exit status are retained. Failures
before evaluation that do not require replay retain their normal coverage behavior.
For explicitly enabled coverage with a nonnegative effective interval, the
standard breadth-first CLI can emit periodic progress and coverage when both output sinks are `os.File`s whose
`Stat` reports regular files, and
checkpoint/recovery, debugging/profiling, state dumps, user output files,
SpecTE, simulation, DFID and continuation are absent. Coverage model validation
still applies. The timer begins after initial-state generation, first reports
progress after three seconds and then at one-minute ticks. It reports coverage
after the tick count derived from Java's 32-bit millisecond interval, including
wrapped intervals. An effective zero reports coverage at every progress tick;
omitting `-coverage` does not enable periodic reporting. Reports and exploration
callbacks are committed as whole output transactions; the reporting goroutine is stopped and joined before
final or error output. The counts and coverage are sampled separately, while
exploration continues, so their values need not describe the same instant.

Other positive-interval cases retain the conservative deadline: the earlier of
the effective interval and three seconds plus that interval rounded down to
whole minutes. If it elapses, checking is canceled with an error. For ineligible
outputs or options, `-coverage 0` and arguments whose 32-bit millisecond conversion
wraps to zero retain their previous behavior: no timeout and a final coverage
report, but no periodic output. Java can wake its timed waits spuriously, so
exact reporting times are not guaranteed.
Pipes, terminals, other non-regular files, and files that cannot be statted use
the fallback path (including a deadline only for positive intervals): a blocked
periodic write cannot otherwise be canceled and joined safely. Ordinary CLI
writes can still block if their consumer stops reading.
Java 1.7.4 starts its progress loop after initial-state generation and waits up
to three seconds before the first progress report. Its progress report reads the
fingerprint set size, generated count, level and queue size separately while
workers continue. Coverage uses `LongAdder.sum()` for each counter and walks
the action and expression tree while workers continue; even two readings of
the same counter can differ. Thus a report is not necessarily a single
instantaneous snapshot, and relationships between different counters cannot
be assumed to hold at every report. Successful termination can also wake the
wait before the tick. See Java 1.7.4's
[`AbstractChecker`](https://github.com/tlaplus/tlaplus/blob/v1.7.4/tlatools/org.lamport.tlatools/src/tlc2/tool/AbstractChecker.java),
[`ModelChecker`](https://github.com/tlaplus/tlaplus/blob/v1.7.4/tlatools/org.lamport.tlatools/src/tlc2/tool/ModelChecker.java),
[`ActionWrapper`](https://github.com/tlaplus/tlaplus/blob/v1.7.4/tlatools/org.lamport.tlatools/src/tlc2/tool/coverage/ActionWrapper.java), and
[`CounterStatistic`](https://github.com/tlaplus/tlaplus/blob/v1.7.4/tlatools/org.lamport.tlatools/src/tlc2/util/statistics/CounterStatistic.java).
The live oracle test checks message order and presence rather than fixing
concurrent count values. The checker publishes progress counters at its own
search mutation points; its depth is the maximum discovered depth, which can
differ from Java's independently sampled reporting level. The coverage report
uses a synchronized snapshot of individual collector updates, while Java's
report can sample counters independently. No replay is performed for a periodic
report, and a slow successor evaluation does not hold the collector lock.
These restrictions and sampling differences mean periodic reporting is not full
Java TLC parity.

`Options.LiveProgress` can opt a coverage check into publishing copies of its
generated, distinct, queued and depth counters. A fresh `LiveProgress` belongs
to one check, and its `Snapshot` can be read concurrently or after a failure
or cancellation. It starts at zero, including when setup fails before search;
`InitialComplete` is published before the corresponding search event. The
counts follow the existing search result's mutations, including failure replay;
updates to different counters are not a single state transition. Queue excludes
the state being expanded, and depth is the existing result's maximum depth,
not a guarantee of Java's independently sampled level. Successful empty search
publishes its final depth of one. There is no completion signal: callers must
coordinate check completion and stop and join their own observers. These
snapshots do not read the mutable result or queue from the observer. The option
requires coverage and is unsupported with checkpoints, simulation and DFID.
It does not change the existing integer overflow behavior of those counters.

Coverage rejects temporal properties, action constraints, symmetry, VIEW, continuation, simulation, and depth-first search. Tool mode emits coverage messages with the
Java TLC codes and ordering for the supported models. Complex expressions, other operator calls,
other action quantification, parameterized or inline actions, initial or action disjunctions,
substituted instances, other nested cost-model expressions, and coverage during error
replay and liveness are not implemented or fully checked. The upstream `C`
model therefore remains unsupported for coverage. Reporting for arbitrary
models and early failures has not been established as Java-compatible.

Action guards following a deterministic assignment, before any range
assignment, are rejected: their preceding assignment costs are not yet modeled.
A guard or assignment after `UNCHANGED` is also rejected because its continuation
can change the cost TLC attributes to `UNCHANGED`, even for an empty range.
