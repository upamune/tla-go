`Github179a`, `Github179b`, and `Github179c` TLA and CFG files are copied from the TLA+ Tools test
models at Java TLC v1.7.4, commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`.
They exercise `tlc2.tool.Github179cTest` and are covered by
`TestJavaTLCGithub179c`. The upstream project is distributed under the MIT
license; see `LICENSE` in this directory.

`Github798I.tla` and `Github798N.tla` are unchanged test-model files from the
same commit. Their tests use `-config Github798I.tla` and `-config Github798N.tla`,
which resolve to missing `.tla.cfg` files. `etest6-java.dot` is the complete
output of the pinned 1.7.4 jar with `-tool -deadlock -fp 0 -coverage 1 -workers 1
-checkpoint 0 -dump dot`, generated from the unchanged suite model and config.

`test17`, `test20`, and `test21` are unchanged models from the same release's
`test-model/suite` directory. They exercise priming and ENABLED with operator
arguments.

`test55` and `test55a` are unchanged models from the same suite. They exercise
`ENABLED` with substituted module variables.

`test57` and `test57a` are unchanged models from the same suite. They exercise
`ENABLED` through operator arguments and substituted module variables.

`test61` and `test61a` come from the same suite; `test61.tla` has its trailing
blank line removed. Java TLC 1.7.4
rejects its parameterized instance, while 1.8.0 and tla-go explore five
distinct states. `TestJavaTLCParameterizedInstanceVersion` records both results.

`test30` and `test999` are unchanged models from the same suite. They exercise
configuration substitutions of standard and infix operators.

`test204` and `test205` are unchanged models from the same suite. They exercise
symbolic operator calls, including calls through module instances.

`test45` is an unchanged model from the same suite. It exercises recursive
functions returning functions.

`test51`, `test51a`, `test51b`, and `test52` are unchanged models from the same
suite. They exercise symbolic operators passed as higher-order arguments.

`test203` and `test203a` are unchanged models from the same suite. They test a
recursive operator passed into a parameterized module instance.

`test209` is an unchanged model from the same suite. It exercises positional
subexpression references and their lexical scopes.

`test206` and `test206a` are unchanged models from the same suite. They test
labels, selected subexpressions, and instance operator substitutions.

`test207` is from the same suite, with trailing whitespace normalized. It covers
hierarchical proof steps and the scope of `NEW`, `SUFFICES`, and `DEFINE`.

`test219` and `test219a` are unchanged models from the same suite. They test
parameterized instances, theorem labels, and subexpression selections.

`etest16` is an unchanged model from the same suite. It checks that duplicate
record and record-set fields are rejected during semantic analysis.

`etest1`, `etest2`, and `etest5` are unchanged models from the same suite.
They test wrong operator argument counts and an undefined qualified name.

`etest12` through `etest15` are unchanged models from the same suite. They
exercise errors while evaluating assumptions, including cardinality overflow,
an unsatisfied CHOOSE, and comparison of incompatible function-domain values.

The files in `simulation-liveness` are unchanged copies of upstream test
models from the same release. `simulation` holds `simulation/Example1` and
`Example2`; `test-model` holds `Test2`, `Test2a`, and `TLCGetLevel` from the
test-model root; `CodePlexBug08` holds that directory's `MC` and
`CodeplexBug8`; `NQSpec` holds `simulation/NQSpec`; and `symmetry` holds
`Unsymmetric`, `UnsymmetricMCA`, and `UnsymmetricMCB`.
`TestJavaTLCSimulationUpstreamLiveness` runs them in simulation mode, and
`expected.json` records the corresponding Java TLC 1.7.4 messages.

`etest6` is unchanged from the same suite. It tests a next-state action that
reads an unassigned primed variable through an operator argument.

`ViewMap` is an unchanged model from the upstream `test-model` directory. Its
upstream `ViewMapTest` runs it with `-view` and checks the printed trace.

`TraceWithLargeSetOfInitialStatesTest` is an unchanged model from the upstream
`test-model` directory. Its upstream test runs it with `-maxSetSize 10`, which
is smaller than its number of initial states.

`etest4` and `etest8` are unchanged models from the same suite. They test the
error stack for applying a model value as a function in an initial predicate,
and the behavior reported for a failed `Assert` during simulation.

`etest7` and `test201a` are unchanged models from the same suite. They test
configuration substitution of a nonconstant expression and an unassigned
operator constant.

`testa` and `test` are unchanged models from the same suite. They test the
error reported when an imported module declares a different name.

`test215` is an unchanged model from the same suite. It exercises SANY's level
checks for temporal operators and bounded temporal formulas.

`test216`, `test216a`, `test216b`, and `test216c` come from the same suite,
with trailing whitespace normalized. They cover anonymous theorem selections and
theorem references through module instances.

`test217` and `test217a` are unchanged models from the same suite. They check
level constraints for arguments to parameterized module instances.

`StandardModules` is an unchanged model from the upstream `test-model`
directory. Its initial predicate yields no states, so TLC does not evaluate the
temporal next-state expression.

`Github432` is a model and configuration from the same directory, with trailing
blank lines normalized. Its configuration has a malformed set value and
exercises TLC's config error. `TestJavaTLCSymmetrySetWarning` substitutes set
elements into it as the upstream `Github432Test` does.

`continue/Continue` and `continue/DoInitFunctorInvariantContinue` are unchanged
models and configurations from `test-model/Continue` and
`test-model/DoInitFunctor` at the pinned commit. They are used by the upstream
`ContinueTest` and `DoInitFunctorInvariantContinueTest`, respectively.

`upstream-coverage/subset-*.coverage` records the complete human-readable
coverage reports from Java TLC 1.7.4 for the generated models in
`TestJavaTLCCoverageSubsetInvariant`. These exercise empty and small interval
powersets, repeated invariant evaluation, and existential short-circuiting.
The test also compares the complete coverage message sequence in `-tool` mode.
