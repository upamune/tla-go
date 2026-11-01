These models and configurations are from `test-model/coverage` in TLA+ Tools commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` (Java TLC 1.7.4). See
[UPSTREAM-LICENSE](../../../../../UPSTREAM-LICENSE).

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

`three-independent.coverage` and `three-dependent.coverage` are Java 1.7.4
reports for the small three-variable models constructed in the tests.

`exists-*.coverage` reports come from the 16 generated models in
`java_tlc_coverage_quantified_test.go`, using the pinned Java 1.7.4 jar with
`-workers 1 -deadlock -coverage 0`. They vary the bounded existential's
domain and preceding guard, including repeated states and an invariant.
