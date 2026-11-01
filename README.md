# tla-go

A TLA+ model checker written in Go. It aims to be compatible with the original [Java TLC](https://github.com/tlaplus/tlaplus), and runs as a standalone binary without Java.

> **Experimental:** TLA+ and TLC support is incomplete. A successful check does not yet guarantee the same result as Java TLC. Verify important models with Java TLC; see [compatibility and limitations](docs/compatibility.md).

## Install

Install Go 1.27 or newer, then run:

```sh
go install github.com/upamune/tla-go/cmd/tla-go@latest
tla-go -version
```

Make sure Go's binary directory (`$(go env GOPATH)/bin`, unless `GOBIN` is set) is on your `PATH`. This installs the latest available revision; the project has not published a versioned release yet.

## Quick start

Save this specification as `Counter.tla`:

```tla
---- MODULE Counter ----
EXTENDS Naturals
VARIABLE count

Init == count = 0
Next == IF count < 3 THEN count' = count + 1 ELSE UNCHANGED count
TypeOK == count \in 0..3
====
```

Save the configuration beside it as `Counter.cfg`:

```text
INIT Init
NEXT Next
INVARIANT TypeOK
```

Run the checker from that directory:

```sh
tla-go Counter.tla
```

The checker finds four distinct states (`count` from 0 to 3) and reports `Model checking completed. No error has been found.` At 3, `Next` leaves the value unchanged, so the model has no deadlock.

To see an invariant violation, change `TypeOK` to `count \in 0..2` and run the command again. The output includes a trace to `count = 3` and the process exits with code 12.

## Check your own model

```sh
tla-go path/to/Model.tla                       # uses path/to/Model.cfg
tla-go -config path/to/Other.cfg path/to/Model.tla
tla-go 'models/*.tla'                          # checks matching models with configs
```

The module name must match the `.tla` filename. External modules referenced by `EXTENDS` or `INSTANCE` are searched in the root module's directory, followed by the directories supplied with `-DTLA-Library` and `-I`, in the resulting list order. A `.cfg` path can also be passed directly.

| Option | Purpose |
| --- | --- |
| `-debug-trace FILE`, `-debug-cpu-profile FILE`, `-debug-heap-profile FILE` | Write opt-in Go runtime diagnostics to new files; see [profiling](docs/development.md#profiling-the-cli). These are tla-go options, not Java TLC flags. |
| `-config FILE` | Use a different TLC configuration file. As in Java TLC, `.cfg` is appended unless `FILE` already ends with it, and a relative name is also searched in the specification's directory. |
| `-DTLA-Library=PATHS`, `-I DIR` | Set or extend the module search list. `-DTLA-Library` replaces all previously supplied paths; separate its paths with `:` on Unix or `;` on Windows. Each `-I` appends one directory, so option order matters. |
| `-metadir DIR` | Create a timestamped run directory under `DIR`; remove it on success. |
| `-deadlock` | Disable deadlock reporting. |
| `-seed N` | Set the random seed; model checking defaults to 0 and simulation chooses a random seed. |
| `-aril N` | Advance the simulation's random stream by N double draws when `-seed` is supplied. |
| `-simulate [num=N,file=PREFIX]` | Run experimental single-worker random simulation; omit `num=N` to run until interrupted or a violation is found. `file=PREFIX` writes completed traces as `PREFIX_0_0`, `PREFIX_0_1`, and so on. |
| `-dfid N` | Check safety with experimental depth-first iterative deepening up to depth N, like Java TLC's `-dfid`. Successors are taken in generation order, not at random. |
| `-continue` | Continue model checking after invariant or action violations, or simulation after safety violations and deadlocks. |
| `-depth N` | Set the simulation trace length (default 100; `-1` means no limit). |
| `-workers N` | Accept a positive worker count or `auto`, as Java TLC does. Eligible BFS models generate successors concurrently and merge them in serial order; other models fall back to serial exploration. Simulation accepts only one worker, and with `-dfid` more workers are rejected with Java TLC's error and exit code 255. |
| `-maxSetSize N` | Limit the sets and other values that Java TLC builds element by element, such as `{e : x \in S}` and `(1..n) \cup S` (default 1,000,000). When an explicit positive bound is supplied, deferred sets in states are limited only for a single variable holding an explicit top-level Cartesian product of intervals in ordinary breadth-first checking, without constraints, VIEW, symmetry, or TLC fingerprint mode. Initial states additionally require non-terse output. Other deferred state values, nested products, and multiple variables are not yet covered. |
| `-coverage N` | Report experimental action and expression counts for a restricted set of breadth-first models. Eligible regular-file output supports periodic reports, including every progress tick for zero; other positive intervals use a conservative deadline. See [limitations](docs/compatibility.md). |
| `-checkpoint MINUTES` | Periodically save checkpoints for supported serial models; zero disables writes. |
| `-recover DIR` | Resume a supported model from a tla-go checkpoint directory. |
| `-fp N` | Select a TLC fingerprint polynomial by index (0–130; default 0). |
| `-tool` | Emit experimental TLC-style machine-readable messages. |
| `-view` | Print trace states, dumped states, and simulation trace files through the configured `VIEW`, as Java TLC does. |
| `-nowarning` | Suppress TLC warnings, such as those for symmetry or constraints with liveness checking. |
| `-dump FILE` | Write each new state in Java TLC's text format; `.dump` is appended if absent. |
| `-dump dot[,colorize][,actionlabels] FILE` | Write a DOT state graph during exploration, optionally with colored edges and action labels; `.dot` is appended if absent. The `snapshot` option is not supported. |
| `-sany-metadata [-I DIR] FILE` | Emit experimental SANY-shaped XML containing variable order and definition levels, without checking a model. |
| `-debug-liveness` | Write liveness debugging events as JSON lines to standard error. |
| `-lncheck STRATEGY` | Accept a liveness strategy argument for CLI compatibility; it does not change checking or reproduce Java's scheduling. |
| `-modern-random-subsets` | Use the newer TLC random-subset algorithm. |
| `-difftrace` | In error traces, print only the variables that changed from the previous state, following Java TLC's rules for which states are printed in full. |
| `-terse` | Print `SUBSET`, function-set, record-set, and Cartesian-product values as formulas instead of enumerating them in `Print`/`PrintT` output, traces, and state dumps. |
| `-userFile FILE` | Write `Print` and `PrintT` output to `FILE` instead of standard output. |
| `-cleanup` | Remove the `states` directory in the working directory before checking. |
| `-generateSpecTE [nomonolith]` | Enable `-tool` and, after a breadth-first model-checking error with a trace, write a trace-exploration specification `SpecTE.tla` and `SpecTE.cfg` beside the specification, as Java TLC does. `nomonolith` keeps dependent modules in `EXTENDS` instead of embedding them. |
| `-fpmem X`, `-fpbits N`, `-gzip`, `-modelcheck`, `-debug` | Accepted with Java TLC's argument checks. They configure Java TLC internals and have no effect, except that `-fpmem` above 1 prints Java's deprecation notice. |
| `-version`, `-help` | Show the version or usage. |

The exit code is 0 on success, 11 for deadlock, 12 for invariant or property violations, and 13 for liveness violations. Other errors can have different codes; exact Java TLC output and exit-code compatibility is still in progress. The `-tool` protocol and state dumps are compared with Java TLC for representative models, but full compatibility is not yet established.

Parallel BFS is experimental and applies only to restricted models; other cases use serial exploration. Workers are capped by GOMAXPROCS and 32, and small queues or expressions run serially. The reported worker count is the requested count, not the number actually running. Statistics and counterexamples can differ from Java TLC, as can state identity and fingerprint collision behavior. See [parallel model checking limitations](docs/compatibility.md#model-checking).

The worker implementation is experimental. On an Apple M3, dedicated `go test ./checker -run '^$' -bench '^BenchmarkParallel(BFS|WideBFS|HeavyBFS)$' -benchtime=1s -count=3` runs measured the inexpensive models at 0.25–0.32 ms and 3.4–4.7 ms per check for both worker counts. A finite model with 1,000 initial states and 200 arithmetic terms per successor took 125–153 ms with one worker and 67–90 ms with four. The heavy model is synthetic; these measurements do not establish a general speedup, and parallel execution allocated more memory.

With `-continue`, model checking can finish with exit code 0 even after reporting violations, matching Java TLC. Check the reported messages or the Go API's violation count.

With `-dfid N`, each iteration restarts from the initial states and searches one level deeper, so the generated-state count includes repeated states. As in Java TLC, a check that reaches depth N before the search ends exits with code 0 but does not report `No error has been found`, and liveness properties are rejected. See [compatibility](docs/compatibility.md) for other differences.

Simulation samples randomly chosen behaviors; a successful run
does not prove a model correct. Temporal properties and the `WF`, `SF`, and
`[]<>` fairness conjuncts of a specification are checked on each sampled
trace as Java TLC 1.7.4 does, not on the whole state space. Other temporal
conjuncts of a specification, such as `<>P`, properties that use `TLCGet`,
multiple workers, state dumps, and liveness debugging in simulation mode are not
supported. Simulation messages and errors are not fully compatible with Java
TLC. For example:

```sh
tla-go -simulate num=100 -depth 50 -seed 1 path/to/Model.tla
tla-go -simulate num=10,file=trace -depth 50 -seed 1 path/to/Model.tla
```

Trace files follow the tested Java TLC 1.7.4 format. Java TLC 1.8.0 writes
different trace lengths and action comments in some cases.
For temporal properties, tla-go follows Java TLC 1.7.4's sampled-trace
graph, including cases where it disagrees with the formula's logical
meaning. Java TLC 1.8.0 can give different results.

`-sany-metadata` emits only the metadata consumed by statechart, not the complete SANY XML semantic graph. It supports selected higher-order operator calls but rejects some constructs that Java SANY supports, including recursive definitions. Its level analysis and semantic validation are experimental; use Java SANY when complete XML or authoritative analysis is needed.

## Translate PlusCal

`tla-go -translate` takes the arguments of Java's `pcal.trans` and translates the PlusCal algorithm in a module to TLA+. `-translate` must be the first argument:

```sh
tla-go -translate -wf -termination path/to/Model.tla   # writes Model.tla and Model.cfg
tla-go path/to/Model.tla                               # checks the translation
```

Like Java, it replaces the text between `\* BEGIN TRANSLATION` and `\* END TRANSLATION`, or inserts those lines after the comment containing the algorithm. It keeps the previous file as `Model.old` and writes `SPECIFICATION Spec` and the statements the options require to `Model.cfg`, preserving the lines that follow its `\* Add statements after this line.` comment. Options can also be given in the module with `PlusCal options (wf, termination)`. As in Java, errors are printed to standard output, and the exit code is 0 after a translation or `-help`, 255 after an error or `-writeAST`, and 1 after an internal failure.

| Option | Purpose |
| --- | --- |
| `-wf`, `-sf` | Conjoin weak or strong fairness of each process to `Spec`. |
| `-wfNext` | Conjoin weak fairness of `Next` to `Spec`. |
| `-nof` | Conjoin no fairness formula; this is the default except with `-termination`. |
| `-termination` | Add `PROPERTY Termination` to the `.cfg` file; implies `-wf` unless another fairness option is given. |
| `-nocfg` | Do not write the `.cfg` file. |
| `-noDoneDisjunct` | Omit the `Terminating` disjunct of `Next`. |
| `-label`, `-reportLabels` | Add missing labels; `-reportLabels` also prints them. |
| `-labelRoot NAME` | Name added labels `NAME1`, `NAME2`, and so on (default `Lbl_`). |
| `-lineWidth N` | Try to keep lines at most `N` characters wide (default 78, minimum 60). |
| `-version V` | Translate as the given PlusCal version, for example `1.4`. |
| `-unixEOL` | Write `\n` line ends; this is the default except on Windows. |
| `-debug` | Include source positions in the AST, which changes the checksum. |
| `-writeAST` | Write the parsed algorithm to `AST.tla` in the working directory instead of translating. |
| `-help` | Show Java's help text. |

`-spec`, `-myspec`, `-spec2`, and `-myspec2` make Java run TLC on PlusCal's own TLA+ specification of the translation. tla-go rejects them.

## Checkpoints

The checkpoint format is versioned, checksummed and limited to 64 MiB. Files
are written through a temporary file and renamed in the same directory. On
non-Windows systems the directory is also synced. On Windows, checkpoint writes
require support for POSIX-style file replacement; filesystems or Windows versions
without it return an error. Durability depends on the filesystem.

Explicit `-checkpoint N` (positive minutes) supports a restricted pure serial
breadth-first search: integer, Boolean, and string states, scalar string
configuration constants, and a restricted pure expression subset with INIT/NEXT
and invariants. String literals, equality, inequality, and finite enumeration
membership are supported within that subset; string operations and other
constant forms remain unsupported. Unsupported models or options
are checked normally with a diagnostic and no checkpoints. `-checkpoint 0`
disables checkpoint writes, including when recovering; an omitted interval on
recovery uses Java TLC 1.7.4's default of 30 minutes plus 42 milliseconds. Checkpoints use tla-go's own format,
not Java TLC's. Existing version 1 integer/Boolean snapshots remain readable;
older tla-go versions fail closed on snapshots containing string values.
Checkpoint creation and recovery require valid UTF-8 in model names and source,
configuration source, and all effective configuration strings. Invalid UTF-8
is rejected with a diagnostic; stored state strings still preserve their bytes.
The diagnostic identifies the timestamped directory beneath `-metadir` (or
`states` beside the specification). Resume with `-recover directory` and the
same model, configuration, and semantic options. Missing, damaged, incompatible,
or unsupported checkpoints fail rather than starting over. Output is streamed
in order during each run; recovery does not replay output from before the saved
checkpoint. Output emitted after the last checkpoint can be repeated after a
restart, and full Java output equivalence is not established.

Checksums detect accidental corruption, not deliberate modification. Structural
and identity checks reject inconsistent snapshots, but cannot prove that a
coherently forged search history is authentic; only recover files from trusted
sources. The 64 MiB limit applies to the serialized payload, not peak memory
used while encoding or decoding it.

For deterministic debugging, `TLA_GO_DEBUG_CHECKPOINT_AFTER=N` saves after N
state expansions and stops with a nonzero status. It is not a production
scheduling option.

## Documentation

- [Compatibility and limitations](docs/compatibility.md): supported model constructs and known differences from Java TLC.
- [Development and testing](docs/development.md): building from source, quality checks, Java TLC comparisons, and fixture provenance.
- [Statechart compatibility plan](docs/statechart-compatibility.md): the CLI, graph, and metadata interfaces needed by statechart.
- [Security](SECURITY.md): how to report a vulnerability.

Java TLC is the compatibility target. Java TLC 1.7.4 fixtures are traceable to TLA+ Tools commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`. Some regression fixtures also come from [tla-ts](https://github.com/aidenybai/tla-ts), pinned at `19c763f07fda4b098e8a24405301628a424547c0`; see [fixture provenance](docs/development.md#compare-with-java-tlc) and [UPSTREAM-LICENSE](UPSTREAM-LICENSE).
