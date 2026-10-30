`upstream-lexer.json.gz` contains the 139 `.tla` files in tla-ts
`packages/tla-ts/tests/fixtures` and the token sequences produced by its lexer at
commit `19c763f07fda4b098e8a24405301628a424547c0`. The source fixtures and
lexer are distributed under the MIT license in `UPSTREAM-LICENSE`.

To regenerate the data, check out that commit and follow the commands in
`scripts/generate-lexer-fixtures.ts`. This corpus checks tokenization only;
it does not assert that the Go parser or model checker implements the specs.
