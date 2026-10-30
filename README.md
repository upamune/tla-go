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

The module name must match the `.tla` filename. External modules referenced by `EXTENDS` or `INSTANCE` are loaded from the root module's directory. A `.cfg` path can also be passed directly.

| Option | Purpose |
| --- | --- |
| `-config FILE` | Use a different TLC configuration file. |
| `-deadlock` | Disable deadlock reporting. |
| `-seed N` | Set the random seed; the default is 0. |
| `-debug-liveness` | Write liveness debugging events as JSON lines to standard error. |
| `-modern-random-subsets` | Use the newer TLC random-subset algorithm. |
| `-version`, `-help` | Show the version or usage. |

The exit code is 0 on success, 11 for deadlock, 12 for invariant or property violations, and 13 for liveness violations. Other errors can have different codes; exact Java TLC output and exit-code compatibility is still in progress.

## Documentation

- [Compatibility and limitations](docs/compatibility.md): supported model constructs and known differences from Java TLC.
- [Development and testing](docs/development.md): building from source, quality checks, Java TLC comparisons, and fixture provenance.
- [Statechart compatibility plan](docs/statechart-compatibility.md): the CLI, graph, and metadata interfaces needed by statechart.
- [Security](SECURITY.md): how to report a vulnerability.

Java TLC is the compatibility target. Some regression fixtures also come from [tla-ts](https://github.com/aidenybai/tla-ts), pinned at `19c763f07fda4b098e8a24405301628a424547c0`; see [UPSTREAM-LICENSE](UPSTREAM-LICENSE).
