# FastMutex `UNCHANGED` allocation investigation

On macOS arm64 (Apple M3, Go 1.27.3), a FastMutex CPU and allocation
profile at `44d536c` showed repeated allocation in `UNCHANGED vars` during
liveness checking. The allocation profile attributed 92.5 MB cumulatively to
evaluating those expressions in one run. This is a sampled allocation profile,
not a measure of retained memory.

The evaluator now compares a small tuple of unshadowed state variables without
materializing two temporary functions. Evaluation with an observer, an
applicable ENABLED substitution, incomplete states, shadowed identifiers, or
more complex expressions uses the existing evaluation path. So does a tuple
containing strings when a value ordering is configured: an application's
`Intern` callback can mutate state during evaluation. An alias is used only
when it is a nonrecursive nullary operator resolving directly to such a tuple.

For comparison, separate checker test binaries were built from `44d536c` and
from the change, then run alternately four times each with `GOMAXPROCS=1`,
`-test.run '^$'`, `-test.bench '^BenchmarkUpstreamFastMutex$'`,
`-test.benchtime=1x`, and `-test.benchmem`. Both versions check the same
42,277 generated and 15,900 distinct states. The baseline allocated about
450.7 MB in 3,939,535–3,939,539 allocations per run; the change allocated
about 374.9 MB in 3,418,096–3,418,104 allocations. Elapsed times ranged from
2.47 to 3.19 seconds for the baseline and 2.27 to 4.61 seconds for the change
while other jobs were running. The time measurements are too noisy to establish
a speedup; the allocation reduction is specific to this benchmark. These runs
preceded the final scalar-operand and observable-string checks; they do not
measure the effect of those checks.
