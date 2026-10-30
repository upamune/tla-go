`upstream-expressions.jsonl` contains selected expression results produced by
`tla-ts` at commit `19c763f07fda4b098e8a24405301628a424547c0`. Each expression
is evaluated with `x = 2` and next-state `x = 3`. Regenerate it with
`scripts/generate-evaluator-fixtures.ts` and a checkout of that commit.

These examples cover only the implemented evaluator subset. Passing them does
not establish full interpreter or model-checker compatibility. The upstream
source and derived test cases are covered by `UPSTREAM-LICENSE`.
