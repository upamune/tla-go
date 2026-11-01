# Development and testing

## Build from source

The repository uses [mise](https://mise.jdx.dev/) to pin Go and the development tools:

```sh
git clone https://github.com/upamune/tla-go.git
cd tla-go
mise trust
mise install
mise exec -- go build -o tla-go ./cmd/tla-go
./tla-go -deadlock examples/Counter.tla
```

The repository's `Counter` example has no successor after reaching 3. Without `-deadlock`, the checker reports the expected deadlock and exits with code 11.

## Checks

```sh
mise run format       # format source
mise run lint         # vet, complexity, duplicate code, and workflow lint
mise run test         # Go tests with the race detector
mise run ci           # formatting check, lint, and tests
mise run similarity   # review additional similarity candidates
```

Production functions are limited to cognitive complexity 10 and cyclomatic complexity 8 by cccc. gosimilarity rejects exact normalized matches of at least 12 lines; the separate similarity task reports candidates of at least 8 lines at 95% similarity. Similarity is a prompt for review, not proof of equivalent behavior. Both commands exclude tests and generated files by default.

Run `mise run secrets`, `mise run security:dependencies`, and `mise run security:workflows` for local security checks. GitHub Actions also runs CodeQL, OSV-Scanner, dependency review, OpenSSF Scorecard, parser fuzzing, and native binary smoke tests on Linux, macOS, and Windows. Dependabot monitors Go modules and GitHub Actions.

For performance changes, compare benchmarks before and after:

```sh
mise exec -- go test -bench=. -benchmem ./checker ./evaluator ./value
```

`BenchmarkUpstreamFastMutex` exercises the Java TLC `FastMutexWithGoto2`
model, including fairness and 15,900 distinct states. To compare this model
without running every benchmark, use:

```sh
mise exec -- go test ./checker -run '^$' -bench '^BenchmarkUpstreamFastMutex$' -benchmem -benchtime=1x -count=3
```

`BenchmarkUpstreamDetlefs` checks the Java TLC `Detlefs` PlusCal model with
two addresses: 31 variables, symmetry reduction and 10,149 distinct states,
without liveness. It mainly measures successor generation and state keys:

```sh
mise exec -- go test ./checker -run '^$' -bench '^BenchmarkUpstreamDetlefs$' -benchmem -benchtime=1x -count=3
```

`BenchmarkParameterizedOperatorFunctionAction` explores 5,000 states through
a parameterized instance whose function action receives an operator argument.
It measures repeated action expansion and state exploration together:

```sh
mise exec -- go test ./checker -run '^$' -bench '^BenchmarkParameterizedOperatorFunctionAction$' -benchmem -benchtime=1x -count=3
```

`BenchmarkSimulationTraces` measures 100 seeded traces of depth 20 through
branching actions. `BenchmarkSimulationTraceConstraint` measures the same
trace count and depth while evaluating `TLCExt.Trace` in a state constraint.
Both report generated states as `states/op`.

```sh
mise exec -- go test ./checker -run '^$' -bench '^BenchmarkSimulation(Traces|TraceConstraint)$' -benchmem -benchtime=1x -count=3
```

`BenchmarkSetConstruction` measures set maps, filters, set operations, and `UNION`, which check `-maxSetSize`:

```sh
mise exec -- go test ./evaluator -run '^$' -bench '^BenchmarkSetConstruction$' -benchmem -count=5
```

## Compare with Java TLC

CI downloads the official Java TLC 1.7.4 and 1.8.0 JARs, verifies their SHA-256 hashes, and runs selected comparisons. To run them locally, install a Java Development Kit (including `java` and `javac`), point `TLC_JAR` to an absolute path to the JAR, and run:

```sh
TLC_JAR=/absolute/path/to/tla2tools-1.7.4.jar TLC_MODERN_JAR=/absolute/path/to/tla2tools-1.8.0.jar mise exec -- go test ./checker ./cmd/tla-go -run '^TestJavaTLC' -count=1 -timeout=30m
```

CI also fetches a pinned external statechart corpus and runs selected
`TestJavaTLCStatechart...` and `TestJavaSANY...` tests with `TLC_JAR` and
`STATECHART_DIR` set. The Java comparisons use TLC/SANY; the statechart
corpus and bridge tests also invoke Bun to run the external statechart parser
or tool. The standalone bridge test uses Bun without Java. CI installs Bun
1.3.11 for this step. Locally these checks are optional: the ordinary
`mise run test` and `mise run ci` do not need Bun or Java when the optional
comparison environment variables are unset.

Java TLC and SANY extract standard modules into `java.io.tmpdir`. The Java
model comparison helpers, PlusCal comparison, and upstream JUnit sweep use
private temporary directories. These Java subprocesses do not inherit
`_JAVA_OPTIONS`, because the JVM applies it after command-line options and it
could override the private directory. Other JVM options, including
`JAVA_TOOL_OPTIONS` and `JDK_JAVA_OPTIONS`, remain available. The parent
environment is unchanged. Java agents or model code that alter JVM properties
or launch their own processes are outside this isolation guarantee. The sweep
removes its directory when the direct process finishes,
including on failure or timeout; test helpers using `t.TempDir` remove it when
the associated test finishes. The sweep waits at most one extra second for
inherited output pipes to close after a timeout or direct process exit. It does
not terminate descendant processes, which can outlive their JVM and continue
using the directory. On Windows an open file may prevent removal; cleanup
failures are returned. The CI-pinned [statechart runner](https://github.com/cursor/plugins/blob/4dd1d05818d19792e7cd6e818f1667f95f7e1dbe/pstack/skills/statechart/scripts/run.ts)
also creates a separate Java temporary directory for each run. The test-only
statechart bridge forwards JVM arguments unchanged; other callers must provide
their own isolation. Java value-only probes do not load TLA+ modules.
JUnit discovery itself does not run models, but it can construct custom JUnit
runners. Discovery does not isolate the JVM temporary directory, so custom
runner constructors that invoke TLC can still share that directory.

The PlusCal translator can also be compared with `pcal.trans` from the same JAR. The normal test run compares tla-go with recorded Java outputs; with `TLC_JAR` set, the test also runs Java, which takes a few minutes. This direct comparison is not among the CI commands above. `PCAL_UPDATE_GOLDEN=1` rewrites the recorded outputs from Java; see [the fixture README](../pcal/testdata/README.md).

```sh
TLC_JAR=/absolute/path/to/tla2tools-1.7.4.jar mise exec -- go test ./pcal -run '^TestJavaPlusCalCorpus$' -count=1
```

Without either JAR variable, the tests still run tla-go assertions but skip direct Java comparisons. These selected tests cover only part of TLC; see the [upstream test inventory](upstream-test-inventory.md) for the audit scope and its limits. Java TLC versions can also differ; selected newer-version comparisons use `TLC_MODERN_JAR` when supplied.

Fixtures from Java TLC 1.7.4 are traceable to TLA+ Tools commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`. Fixture-specific provenance is recorded alongside the test data. The `OneBitMutexNoSymmetryMC.trace.json` fixture records the counterexample asserted by that version's Java test.

The upstream `test-model/symmetry` directory includes general temporal properties with distinct concrete successors that share a symmetry-reduced state. In Java TLC's `LiveCheck.TableauLiveChecker`, successors are checked for tableau consistency before transitions are deduplicated by reduced fingerprint and tableau node. Deduplicating all transitions by reduced state alone misses violations in `ChooseTableauSymmetryMC`; keeping all concrete transitions reports false violations in `April21MC` and `MC`. These models now have regression tests for verdicts and state counts; other tableau forms and exact counterexample traces remain incomplete. Use those models together with the existing symmetry matrices when changing the product construction.

For supported reduced tableau checks, `-debug-liveness` emits `tableau-node` events with product-node, reduced-state and tableau indices, plus their outgoing edges. It also emits `tableau-regenerated` when a node's successors are regenerated. These events inspect the constructed graph without re-evaluating model expressions. Counterexample construction follows TLC's component order and removes repeated reduced states; compare both the loop marker and the recovered concrete states when changing it.

### Recorder diagnostics

For recorder investigations, set `TLA_GO_DEBUG_RECORDER` to a new file path
when running one model in `-tool` breadth-first model-checking mode. Active
checkpointing and recovery are unsupported; `-checkpoint 0` is accepted.
The file is created exclusively with mode 0600. Translation, metadata,
version, help, simulation, and DFID modes reject the variable without creating
a file. JSON lines start with a version 1 header marking the stream incomplete,
followed by selected observations in emission order: SANY start/end, starting,
initial-state start/progress/completion, selected success probabilities, statistics,
search depth, and finish.
Their `strings` arrays come from Go events and results rather than parsed tool
text. A Java TLC 1.7.4 recorder probe compares these selected calls and argument
types across successful and failing models, in tool and non-tool mode; finish
durations naturally differ.

For success message 2193, Java 1.7.4 records a second probability when its
unrounded optimistic probability is at least `1e-10`. That second value depends
on the Java fingerprint set's minimum distance, which tla-go does not yet
compute for breadth-first checking. In that case the debug stream emits an
`omission` for 2193 instead of a partial single-argument record; the stream's
header remains incomplete in all cases. The normal and tool outputs still
omit Java's observed probability.

The selected recorder calls are compared against Java TLC 1.7.4, not 1.8.0.
For example, on an empty specification, 1.8.0 records search depth 2194 as
`0`, while 1.7.4 and tla-go record `1`. On a next-state division-by-zero error,
1.8.0 records 2194 with `1`, while 1.7.4 and tla-go omit that call. CI also
uses the pinned 1.8.0 JAR to compare the printed SANY start/end messages in
selected successful and load-error cases; it does not compare these recorder
events against 1.8.0.

Other recorder calls, hidden calls, Java object values, and ordering relative
to unobserved calls are not represented. This debugging aid is not a complete
Java recorder or evidence that the upstream JUnit suite passes. The EmptyTest
bridge consumes the selected observations while retaining its incomplete verdict.
Diagnostic file failures cause a nonzero exit. The option is off by default
and does not reevaluate model expressions.

Additional lexer, parser, evaluator, and checker fixtures come from tla-ts commit `19c763f07fda4b098e8a24405301628a424547c0`. They provide regression coverage but do not establish Java TLC compatibility. See the fixture READMEs and [UPSTREAM-LICENSE](../UPSTREAM-LICENSE).

The auxiliary TypeScript generators call tla-ts and are not run by the normal Go build or CI. See [scripts/README.md](../scripts/README.md) for their purpose and usage.

## Releases

The release workflow builds CGO-free binaries for Linux, macOS, and Windows on amd64 and arm64 when a `v*` tag is pushed. It publishes archives and SHA-256 checksums to a GitHub release after the checks pass.

## Profiling the CLI

The standalone CLI has opt-in Go runtime diagnostics. Paths must be new files;
the CLI refuses to overwrite existing files or create `.tla` or `.cfg` files.
These options are specific to tla-go and are not Java TLC options:

```sh
tla-go -debug-trace run.trace -debug-cpu-profile cpu.pprof -debug-heap-profile heap.pprof Model.tla
go tool trace run.trace
go tool pprof -http=localhost:0 ./tla-go cpu.pprof
go tool pprof -http=localhost:0 -sample_index=alloc_space ./tla-go heap.pprof
```

Any combination can be used. The trace includes coarse parsing/configuration,
model initialization, and exploration/liveness regions; the latter combines
both kinds of work. The CPU profile samples execution. The heap profile is
captured after a garbage collection when the first search stops, while its model
and search graph are still live (including on failure, cancellation, or checkpoint
pause and recovery). With multiple specifications, it captures the first one
that reaches search; later specifications do not overwrite it. If no search is
reached, it captures the live heap at shutdown instead. Its default `inuse_space`
shows live heap, while `alloc_space` estimates allocations over the process lifetime.
Profiling starts after option parsing and stops before process exit, including
when model loading or checking fails or is interrupted with Ctrl-C. Abrupt
termination such as SIGKILL cannot flush profiles. The heap collection
and CPU profiler shutdown add time, and tracing and profiling perturb execution
and memory use; compare elapsed times with an unprofiled run. CPU samples may be
sparse for short runs. Profile files can contain function names, source paths,
and other execution details; treat them as potentially sensitive.

For Go tests, the standard tooling is separate from these CLI options:

```sh
mise exec -- go test ./checker -run '^$' -bench '^BenchmarkCheckCounter$' -cpuprofile cpu-test.pprof -memprofile mem-test.pprof
go tool pprof cpu-test.pprof
```

The `final liveness check` trace region is nested within exploration when a
final graph check runs. Incremental liveness checks can also occur during
exploration, outside that region.
