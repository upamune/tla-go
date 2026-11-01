These models and configurations are from `test-model/coverage` and
`test-model/suite` in TLA+ Tools commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` (Java TLC 1.7.4). See
[UPSTREAM-LICENSE](../../../../../UPSTREAM-LICENSE).

`A.tla`, `A.cfg`, and `A.coverage` preserve the original `ACoverageTest` model
and its Java TLC 1.7.4 final report. The `A-*.coverage` snapshots are generated
by the small models in `java_tlc_coverage_a_oracle_test.go`; they isolate
parameterized action calls, shared actions, reporting order, source locations,
guards, range assignments and unused parameters. All use `-deadlock -fp 0
-workers 1 -checkpoint 0 -coverage 9999`, selected from the JUnit arguments.
These CLI runs do not execute JUnit setup, assertions or teardown, and omit its
DOT dump. Go supports the restricted direct-assignment and UNCHANGED variants;
the original A and guard variants remain unsupported. See the
[cost-model investigation](../../../../../docs/investigations/coverage-cost-model-a.md).

`etest3.tla` and `etest3.cfg` preserve the upstream ETest3 model. The boundary
test checks Java's complete coverage report and trace-replay Print effects;
the Go coverage implementation still rejects this model.

`guard-*.coverage` contains Java 1.7.4 CLI reports for the models in
`java_tlc_coverage_guard_oracle_test.go`. They cover false and mixed guards,
shared roots, repeated formal occurrences, multiline heads, a type error,
state constraints and substituted instances. Runs use `-deadlock -fp 0
-workers 1 -checkpoint 0 -coverage 9999` and separate metadata directories.
The Go checker rejects these coverage models; they are not JUnit executions.

`origin-distinct-instances.coverage` and `origin-identical-instances.coverage`
record Java 1.7.4 CLI output for the models constructed in
`java_tlc_coverage_origin_oracle_test.go`: two named substituted instances of
the same definition report separate action roots with both different and equal
substitution values, even though their expression source ranges are identical.
`origin-shared-argument.coverage`
records separate formal-parameter locations for an action whose two formal
uses are replaced by the same caller expression in Go's current expansion.
Go still rejects coverage for both models.

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
