`Github179c.tla` and `Github179c.cfg` are copied from the TLA+ Tools test
models at Java TLC v1.7.4, commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`.
They exercise `tlc2.tool.Github179cTest` and are covered by
`TestJavaTLCGithub179c`. The upstream project is distributed under the MIT
license; see `LICENSE` in this directory.

`test17`, `test20`, and `test21` are unchanged models from the same release's
`test-model/suite` directory. They exercise priming and ENABLED with operator
arguments.

`test55` and `test55a` are unchanged models from the same suite. They exercise
`ENABLED` with substituted module variables.

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

`test219` and `test219a` are unchanged models from the same suite. They test
parameterized instances, theorem labels, and subexpression selections.

`etest16` is an unchanged model from the same suite. It checks that duplicate
record and record-set fields are rejected during semantic analysis.

`etest1`, `etest2`, and `etest5` are unchanged models from the same suite.
They test wrong operator argument counts and an undefined qualified name.

`etest12` through `etest15` are unchanged models from the same suite. They
exercise errors while evaluating assumptions, including cardinality overflow,
an unsatisfied CHOOSE, and comparison of incompatible function-domain values.

`etest6` is unchanged from the same suite. It tests a next-state action that
reads an unassigned primed variable through an operator argument.

`etest7` and `test201a` are unchanged models from the same suite. They test
configuration substitution of a nonconstant expression and an unassigned
operator constant.
