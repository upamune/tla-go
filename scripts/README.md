# Development scripts

The TypeScript scripts here are auxiliary tools for generating regression data
from [tla-ts](https://github.com/aidenybai/tla-ts). They call its TypeScript
implementation directly, which is why they use Bun. The Go binary, normal
build, and CI tests do not run these scripts or require Bun; the generated
fixtures are checked into this repository.

Java TLC is the compatibility target. tla-ts fixtures provide additional
regression coverage, not proof of Java TLC compatibility. Some generators copy
and patch tla-ts source in a temporary directory before using it; inspect the
individual script when interpreting its output.

The tla-ts reference revision is
`19c763f07fda4b098e8a24405301628a424547c0`. To regenerate a fixture, check out
that revision in a separate tla-ts directory, install Bun, and follow the
command in the script's first lines. For example, from the repository root:

```sh
bun scripts/generate-any-fixtures.ts /path/to/tla-ts > /tmp/tla-go-any.jsonl
```

Inspect and compare the output before replacing checked-in data. Bun is not
pinned by this repository, so reproducing historical output may require the
runtime used when a fixture was generated.

`verify-parameterized-function-action.sh` is different: it runs selected
fixtures against a supplied Java TLC JAR. See [development and testing](../docs/development.md)
for the Java comparison tests used in CI.
