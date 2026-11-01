# Java TLC weak-fairness effects investigation

Oracle: tla2tools v1.7.4, upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`, JAR SHA-256 `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`. `TestJavaTLCFairnessEffectsLifecycle` exercises the isolated models; set `TLC_JAR` to the pinned JAR to run the Java comparisons. Each Java invocation uses one worker, fingerprint polynomial 0, and its own temporary model and metadata directory. Plain and `-tool` were compared. The original was also run three times with a fresh metadir per run and one worker: Java printed 10 times and Go 7 times on all three runs, with 3 generated and 2 distinct states and a liveness violation. This is not a determinism claim for multiple workers, arbitrary fingerprint polynomials, periodic checks, or other models.

In additional single-worker runs with fingerprint polynomial indices 0 through 5, the labeled model produced the same ten values in the same order. This sample does not establish a general ordering guarantee.

For `Next == /\ x' = (x+1)%2 /\ PrintT(<<x,x'>>)` and `WF_x(Next)`, an isolated copy of the pinned Java `tlc2.module.TLC` class was instrumented to print the Java stack at `PrintT`. With one worker and `-fp 0`, the ten calls were:

| Position | Value | Java caller / purpose |
| --- | --- | --- |
| 1 | `<<0, 1>>` | `Worker.run`: generate successors |
| 2 | `<<0, 1>>` | `LNStateEnabled.eval`: fairness check for the real successor |
| 3 | `<<0, 1>>` | `LNAction.eval`: fairness action for the real successor |
| 4 | `<<0, 1>>` | `LNStateEnabled.eval`: fairness check for the stuttering successor |
| 5 | `<<1, 0>>` | `Worker.run`: generate successors |
| 6 | `<<1, 0>>` | `LNStateEnabled.eval`: fairness check for the real successor |
| 7 | `<<1, 0>>` | `LNAction.eval`: fairness action for the real successor |
| 8 | `<<1, 0>>` | `LNStateEnabled.eval`: fairness check for the stuttering successor |
| 9 | `<<0, 1>>` | `LiveWorker.printTrace`: recover the next trace state |
| 10 | `<<1, 0>>` | `LiveWorker.printTrace`: recover the closing action |

The instrumented class was compiled from the pinned source against the pinned JAR and placed before it on the classpath; the JAR itself was not modified. `Worker.doNextCheckLiveness` adds stuttering to the successor set, and `LiveCheck.addNextState` evaluates each solution's action checks on every concrete successor before state checks. In this example the check includes `~ENABLED <<Next>>_x \/ <<Next>>_x`; both successor checks evaluate `ENABLED`, while only the real transition evaluates the angle action. Set iteration, other graphs, and short-circuiting can change the order and number of evaluations. Trace reconstruction can run concurrently with prefix reconstruction in Java; the position table is evidence for this model and run, not a universal schedule.

Go's `evaluateIncrementalAtom` caches state atoms by source state across successor choices. That suppresses one effectful `ENABLED` evaluation per source state in this example. On this violation its `replayLassoPath` also does not evaluate the closing transition for the printed loop label. A separate control with fairness action independent of `Next` still shows the missing closing transition; removing fairness entirely selects a stuttering counterexample with no closing `Next` replay. A disabled fairness action and the satisfied property show why neither the existence of fairness nor a violation alone predicts replay. State-valued `PrintT(x)` and `Print` reproduce the same lifecycle. State and action constraints rejecting the only non-stuttering successor both produce two prints and success in both implementations. A `TLCSet`/`TLCGet` counter reaches 8 before Java's trace reconstruction; replay of `Init` resets it and the two trace evaluations print 1 and 2. The Go counter reaches 6 before reconstruction. Thus these are observable runtime changes, not just extra output.

No production fix is included. The next step is to model Java's per-solution, per-concrete-successor check execution, including short-circuiting and the ordering of effectful state subexpressions, while retaining the actual check results for SCC analysis. Separately, model trace recovery of the closing action using Java's actual successor generation and failure behavior, including nondeterministic candidates and reductions. First add differential cases where register writes change fairness or successor selection, failed recovery, multiple successors and constraints, and error/termination behavior. Avoid simply invalidating all caches or adding unconditional replay: either can change reachability, verdicts, registers, errors, or termination. Existing tests deliberately assert Java's oracle and matching controls but log the known Go output divergence; they do not claim that the bug is fixed.
