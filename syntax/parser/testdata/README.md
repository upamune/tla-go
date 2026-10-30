`upstream-examples.json` contains selected TLA+ modules and projections of
ASTs produced by tla-ts at commit `19c763f07fda4b098e8a24405301628a424547c0`.
The projection maps the supported subset to tla-go's AST while preserving
node kinds, operands, declarations, and source locations. Regenerate it with
`scripts/generate-parser-fixtures.ts` and a checkout of that upstream commit.

These examples exercise only the parser forms implemented so far; they are not
an assertion of full tla-ts parser or model-checker compatibility.

`upstream-corpus.jsonl.gz` contains projections of 132 of the 139 TLA+
fixtures at the same commit. The projection checks the fields represented by
our current AST; it does not include all upstream module units or proof data.
The seven files omitted during generation and their reasons are recorded in
`upstream-corpus-skipped.txt`: four are rejected by the upstream parser and
three use proof structures the projection cannot yet represent. Regenerate
both files with `scripts/generate-parser-corpus.ts` and inspect any changes to
the skipped list. The input files are covered by `UPSTREAM-LICENSE`.
