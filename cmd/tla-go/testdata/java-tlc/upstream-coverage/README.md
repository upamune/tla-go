These models and configurations are from `test-model/coverage` in TLA+ Tools commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` (Java TLC 1.7.4). See
[UPSTREAM-LICENSE](../../../../../UPSTREAM-LICENSE).

`A.tla`, `A.cfg`, and `A.coverage` preserve the original `ACoverageTest` model
and its Java TLC 1.7.4 final report. The `A-*.coverage` snapshots are generated
by the small models in `java_tlc_coverage_a_oracle_test.go`; they isolate
parameterized action calls, shared actions, reporting order, source locations,
guards, range assignments and unused parameters. All use `-deadlock -fp 0
-workers 1 -checkpoint 0 -coverage 9999`, selected from the JUnit arguments.
These CLI runs do not execute JUnit setup, assertions or teardown, and omit its
DOT dump. Go still rejects these models in coverage mode. See the
[cost-model investigation](../../../../../docs/investigations/coverage-cost-model-a.md).

`B.coverage` records the full final coverage report for upstream `B.tla` and
`B.cfg` from Java TLC 1.7.4 with `-workers 1 -coverage 9999` (the same
interval as `BCoverageTest`). Tests compare variants and tool coverage messages against
the Java oracle when `TLC_JAR` is set.

`CoverageStatistics.coverage` is the full final coverage report from Java TLC
1.7.4 with `-workers 1 -coverage 0`; its timestamp is omitted.
The `CoverageStatistics-guard.coverage` and
`CoverageStatistics-deterministic-init.coverage` reports use the same command,
with respectively the first action guard changed to `x < 2` and the initial
membership changed to `x = 1`. Other `CoverageStatistics-*.coverage` files
cover equality and ordering guards, independent and dependent initial ranges,
empty ranges, and next-state range enumeration. The tests construct these variants from
the upstream fixture and compare the complete report with these snapshots and,
when `TLC_JAR` is set, with Java directly. `C.coverage` records the same Java
report for the upstream quantified-action and invariant example; Go does not
yet support collecting that model's coverage.

`G.tla`, `G.cfg`, and `G.coverage` preserve the unmodified upstream G coverage
model and the final report from a Java 1.7.4 CLI run with selected JUnit
arguments `-deadlock -fp 0 -workers 1 -checkpoint 0 -coverage 9999`. It does
not execute the JUnit lifecycle or its DOT dump. Go still rejects G:
its alias, multiple UNCHANGED conjuncts and ENABLED invariant need additional
cost-model support. Smaller derived cases test a direct UNCHANGED next action
over all variables; state constraints and INSTANCE substitutions remain
outside that implementation's scope.

`three-independent.coverage` and `three-dependent.coverage` are Java 1.7.4
reports for the small three-variable models constructed in the tests.

`exists-*.coverage` reports come from the 16 generated models in
`java_tlc_coverage_quantified_test.go`, using the pinned Java 1.7.4 jar with
`-workers 1 -deadlock -coverage 0`. They vary the bounded existential's
domain and preceding guard, including repeated states and an invariant.
