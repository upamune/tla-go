# ContinueTest DOT sweep investigation

2026-11-01, macOS arm64, Java Temurin 21.0.11, Go 1.27.0. This is a
bounded experiment, not a JUnit compatibility verdict.

The full diagnostic logs record `dump:content` for `tlc2.tool.ContinueTest`
at e81f221 and no selected difference at 9f58162. They retain neither raw
DOT nor the scheduler decisions, and both explicitly mark the comparison as
partial. The snapshot `internal/upstreamjunit/testdata/snapshots/sweep-e81f221f.json`
records the former result. The change does not by itself demonstrate an
improvement.

The checkout `/tmp/tlaplus-java-v1.7.4` was clean at upstream commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba`. The jar's SHA-256 was
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`.
The original `test-model/Continue.tla` and `.cfg` hashes were respectively
`6e71c2e2863cb059a8bbced9dd6bd114afbc42e48d4b240ffdf27cef034654fb`
and `73351a3785bb6d495a4a84143d91545658b3cd1475a03234816c2db7fdbf6e93`.
They specify two invariants, a constraint, and branching `Next`, including a
duplicate transition. Both fixed Go binaries were built from `git archive`
of e81f221f7ba59b833244e7613bd149f836532c86 and
9f5816269083eb7c9d44e5477695e95e6bf68059, not from the current worktree.

Each side ran in a fresh directory, with separate raw DOT and output logs.
The sweep's source-file Java launcher sets the final-only liveness threshold;
all sides received `-tool -deadlock -fp 0 -workers N -checkpoint 0 -dump dot
<absolute-path> -continue Continue`, with Java additionally receiving
`-coverage 1`. The two fixture files are sufficient for this model; unlike the
sweep, this experiment does not copy the entire tracked fixture tree.

| Workers | Runs per implementation | Distinct raw Java DOT hashes | Distinct raw Go hashes per revision | Paired raw matches per revision |
| --- | ---: | ---: | ---: | ---: |
| 1 | 12 | 1 | 1 | 12/12 |
| 3 | 12 | 5 | 1 | 0/12 |

All 72 processes exited 0. All 72 DOTs had the same multiset of complete
lines, including duplicate lines. For example, Java moved the duplicate edge
from the initial state to `x = 2, y = 1` later in the file. Sorting lines is
a diagnostic for this particular output, not a general DOT graph-equivalence
algorithm. The twelve 3-worker Go files at both revisions also matched every
1-worker file byte for byte. This sample does not establish Go's behavior for
all schedules, nor that other models have equivalent graphs. In particular,
no deterministic compatibility bug is established here and no production fix
is warranted on this evidence.

The real JUnit 4 `ContinueTest.testSpec` was also executed six times in fresh
JVMs and fresh fixture directories. All six passed; their six raw DOTs had five
hashes, one of which matched the Go files above. All six had the same line
multiset as the partial runs. The actual lifecycle uses `@Before` to install a
recorder, set `vetoCleanup`, set `ToolIO`'s user directory, set the liveness
threshold, configure coverage, and call `TLC.process`; `@After` asserts the
exit status. The test checks 32 generated, 29 distinct, depth 29, 32 recorded
trace states, and zero uncovered expressions. It does not compare DOT bytes.
The `-dump` path uses `${metadir}/tlc2.tool.ContinueTest.dot`; files remained
under each run's `states` directory. The class overrides the worker count to
3 and says concurrent BFS does not guarantee trace order. The diagnostic uses
`-tool`, a wrapper, a fixed external dump path and no Go coverage; it does not
run JUnit's recorder or setup/teardown on Go. Thus a selected assertion match
in the sweep is not a JUnit pass. Coverage and lifecycle remain unverified on
Go. Matching line multisets here also does not prove equal trace order or all
recorder observations.

## Reproduce

The original raw outputs from this experiment are separately retained under
`/tmp/tla-continue-investigation/runs/w{1,3}-{java,e81,9f}-NN/graph.dot` and
`output.log`; JUnit outputs and DOTs are under `junit-NN`. `/tmp` is ephemeral.
Run the following in this repository with the named pinned checkout and jar.
The build archive contains the tracked revision only; these commands do not
change the repository or commit anything. `D` must be a new path.

```sh
D=/tmp/tla-continue-repro
mkdir -p "$D/src-e81" "$D/src-9f" "$D/runs" "$D/junit-classes"
git archive e81f221 | tar -x -C "$D/src-e81"
git archive 9f58162 | tar -x -C "$D/src-9f"
(cd "$D/src-e81" && GOWORK=off go build -o "$D/tla-e81" ./cmd/tla-go)
(cd "$D/src-9f" && GOWORK=off go build -o "$D/tla-9f" ./cmd/tla-go)
cat > "$D/JUnitTLC.java" <<'JAVA'
class JUnitTLC {
  public static void main(String[] args) throws Exception {
    tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE;
    tlc2.TLC.main(args);
  }
}
JAVA
for w in 1 3; do
  for i in $(seq -w 1 12); do
    for side in java e81 9f; do
      run="$D/runs/w$w-$side-$i"
      mkdir -p "$run"
      cp /tmp/tlaplus-java-v1.7.4/tlatools/org.lamport.tlatools/test-model/Continue.{tla,cfg} "$run/"
      if [ "$side" = java ]; then
        (cd "$run" && java -cp /tmp/tla2tools-v1.7.4.jar "$D/JUnitTLC.java" -tool -deadlock -fp 0 -coverage 1 -workers "$w" -checkpoint 0 -dump dot "$run/graph.dot" -continue Continue > output.log 2>&1)
      else
        (cd "$run" && "$D/tla-$side" -tool -deadlock -fp 0 -workers "$w" -checkpoint 0 -dump dot "$run/graph.dot" -continue Continue > output.log 2>&1)
      fi
      printf '%s %s %s exit=%s\n' "$w" "$side" "$i" "$?"
      shasum -a 256 "$run/graph.dot"
    done
  done
done
```

Inspect `diff -u` of individual raw DOT files. For this fixture only, compare
line multisets with `LC_ALL=C sort graph.dot | shasum -a 256`; sorting preserves
duplicate lines. Record the process exit codes rather than inferring success
from the existence of a file.

For the actual JUnit lifecycle, the following uses JUnit 4.12 and Hamcrest
Core 1.3 at `/tmp/junit-4.12.jar` and `/tmp/hamcrest-core-1.3.jar`, SHA-256
`59721f0805e223d84b90677887d9ff567dc534d7c502ca903c0c2b17f05c116a` and
`66fdef91e9739348df7a096aa384a5685f4e875584cce89386a7a47251c4d8e9`.
They can be obtained from Maven Central as described in the
[JUnit investigation](../../internal/upstreamjunit/README.md#executing-one-upstream-junit-case-opt-in-experiment).

```sh
T=/tmp/tlaplus-java-v1.7.4/tlatools/org.lamport.tlatools
CP="$D/junit-classes:/tmp/tla2tools-v1.7.4.jar:/tmp/junit-4.12.jar:/tmp/hamcrest-core-1.3.jar"
javac -cp "$CP" -sourcepath "$T/test" -d "$D/junit-classes" "$T/test/tlc2/tool/ContinueTest.java"
for i in 01 02 03 04 05 06; do
  run="$D/junit-$i"
  mkdir -p "$run/test-model"
  cp "$T"/test-model/Continue.{tla,cfg} "$run/test-model/"
  (cd "$run" && java -Dbasedir="$run/" -cp "$CP" org.junit.runner.JUnitCore tlc2.tool.ContinueTest > junit.log 2>&1)
  printf '%s exit=%s\n' "$i" "$?"
  tail -5 "$run/junit.log"
  find "$run/states" -name '*.dot' -exec shasum -a 256 {} \;
done
```

Results depend on scheduling, machine and tool versions; none of the small
samples proves a probability distribution. Re-run with fresh directories. The
logs and fixed binaries above were generated with Go 1.27.0; the repository's
mise configuration currently pins a different Go version, so record the
actual compiler when repeating.
