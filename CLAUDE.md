# tla-go

Go implementation of TLA+ and Java TLC, with tla-ts as an additional reference.

Use mise for tools. Run `mise run format`, `mise run lint`, `mise run test`, or `mise run ci`. Pin GitHub Actions with pinact. Keep compatibility tests traceable to the upstream commit listed in README. Measure performance changes using benchmarks. Java TLC is the compatibility target.

- `config/`: TLC configuration parser.
- `syntax/`: TLA+ lexer, partial parser, and upstream fixtures.
- `value/` and `evaluator/`: partial TLA+ runtime.
- `checker/`: partial finite-state safety checker.
- `cmd/tla-go/`: experimental CLI.
- `.github/workflows/`: CI and tagged builds.
