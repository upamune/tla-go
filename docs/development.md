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

## Compare with Java TLC

CI downloads the official Java TLC 1.7.4 JAR, verifies its SHA-256, and runs selected comparisons. To run them locally, install Java, point `TLC_JAR` to an absolute path to the JAR, and run:

```sh
TLC_JAR=/absolute/path/to/tla2tools.jar mise exec -- go test ./checker ./cmd/tla-go -run '^TestJavaTLC' -count=1
```

Without `TLC_JAR`, the tests still run tla-go assertions but do not execute the direct Java comparisons. These selected tests cover only part of TLC. Java TLC versions can also differ; selected newer random-subset tests use `TLC_MODERN_JAR` when supplied.

Fixtures from Java TLC 1.7.4 are traceable to TLA+ Tools commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`. Fixture-specific provenance is recorded alongside the test data. The `OneBitMutexNoSymmetryMC.trace.json` fixture records the counterexample asserted by that version's Java test.

The upstream `test-model/symmetry` directory includes general temporal properties with distinct concrete successors that share a symmetry-reduced state. In Java TLC's `LiveCheck.TableauLiveChecker`, successors are checked for tableau consistency before transitions are deduplicated by reduced fingerprint and tableau node. Deduplicating all transitions by reduced state alone misses violations in `ChooseTableauSymmetryMC`; keeping all concrete transitions reports false violations in `April21MC` and `MC`. These models now have regression tests for verdicts and state counts; other tableau forms and exact counterexample traces remain incomplete. Use those models together with the existing symmetry matrices when changing the product construction.

For supported reduced tableau checks, `-debug-liveness` emits `tableau-node` events with product-node, reduced-state and tableau indices, plus their outgoing edges. It also emits `tableau-regenerated` when a node's successors are regenerated. These events inspect the constructed graph without re-evaluating model expressions.

Additional lexer, parser, evaluator, and checker fixtures come from tla-ts commit `19c763f07fda4b098e8a24405301628a424547c0`. They provide regression coverage but do not establish Java TLC compatibility. See the fixture READMEs and [UPSTREAM-LICENSE](../UPSTREAM-LICENSE).

The auxiliary TypeScript generators call tla-ts directly and are not used by the normal Go build or CI. See [scripts/README.md](../scripts/README.md) for their purpose and usage.

## Releases

The release workflow builds CGO-free binaries for Linux, macOS, and Windows on amd64 and arm64 when a `v*` tag is pushed. It publishes archives and SHA-256 checksums to a GitHub release after the checks pass.
