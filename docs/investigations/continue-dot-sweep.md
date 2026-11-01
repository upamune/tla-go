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

## Follow-up at 51c3a77

The later `51c3a77` diagnostic snapshot again recorded `dump:content`, while
`9f581626` recorded no selected difference. On 2026-11-01, we repeated the
three-worker command above eight times each with pinned Java 1.7.4 and Go
binaries built from clean detached worktrees at `9f5816269083eb7c9d44e5477695e95e6bf68059`
and `51c3a77c6c4cbc02c6f388b0c90047cc895f9869`. The upstream checkout and
model hashes were as listed above; the runs used Go 1.27.3 and Temurin Java
21.0.11 on macOS arm64. Each process had a 30-second wall-clock limit and a
separate directory. All 24 exited 0 and reported 32 generated, 29 distinct,
and zero queued states.

The 16 Go DOT files were byte-identical across both revisions, SHA-256
`53e246d2dea76a98ad680174bb60e320ede54fc9f543dd28440208a834e1eed1`.
The eight Java files had six distinct raw hashes; one matched the Go bytes.
The [raw hash and exit summary](testdata/continue-51c3a77/raw-hashes.txt)
and [canonical hash summary](testdata/continue-51c3a77/canonical-hashes.txt)
record each run.
The saved raw files [Java run 1](testdata/continue-51c3a77/java-01.dot),
[Java run 2](testdata/continue-51c3a77/java-02.dot), and
[Go 51 run 1](testdata/continue-51c3a77/go-51.dot) preserve two Java orderings
and the Go ordering. In one Java/Go diff the duplicate edge from `(x=1,y=1)`
to `(x=2,y=1)` appears at different positions.

For this fixture, [the Go canonicalizer](continuecanon/main.go)
resolves fingerprint IDs to their complete state labels, preserves which
vertex is initial, and sorts vertex and directed-edge records. It preserves
duplicate edges and rejects unsupported DOT syntax or duplicate state labels.
It checks that rank declarations reference known vertices, then ignores their
layout effect. All 24 canonical files matched
[the captured graph](testdata/continue-51c3a77/graph.canonical) byte for byte:
29 vertices and 29 edge records, including one duplicate edge. Its SHA-256 is
`c06e8cc9942e06f822af25abb3d7ba6f7b127649c8aa83de472c66cd85a2bbb4`.
This demonstrates equality of the captured labeled graphs and edge-record
multisets, not full DOT layout or JUnit lifecycle compatibility. The original
sweep did not retain its two DOT files, so its exact difference cannot be
proven from these reruns. The sample supports output-order nondeterminism as
an explanation; it does not establish a graph regression between revisions.
All 24 raw DOTs, logs and summaries were also kept under the ephemeral
`/tmp/tla-sweep51-investigation/runs` directory.

To reproduce this bounded follow-up, run from the repository root. The script
refuses to overwrite an existing run directory. The checkout and JAR paths
default to the pinned ones used above; verify their revision and hash before
running. The exported directory variable is also used by the timeout follow-up.

```sh
D=$(mktemp -d "${TMPDIR:-/tmp}/tla-sweep51.XXXXXXXX") || exit 1
D=$(cd "$D" && pwd -P) || exit 1
export TLA_SWEEP51_DIR="$D"
printf 'Results: %s\n' "$D"
TLA_GO_JUNIT_ROOT=${TLA_GO_JUNIT_ROOT:-/tmp/tlaplus-java-v1.7.4}
TLA_GO_JUNIT_JAR=${TLA_GO_JUNIT_JAR:-/tmp/tla2tools-v1.7.4.jar}
TLA_GO_JUNIT_ROOT=$(cd "$TLA_GO_JUNIT_ROOT" && pwd -P) || exit 1
task_jar_dir=$(cd "$(dirname "$TLA_GO_JUNIT_JAR")" && pwd -P) || exit 1
task_jar_name=$(basename "$TLA_GO_JUNIT_JAR") || exit 1
TLA_GO_JUNIT_JAR="$task_jar_dir/$task_jar_name"
export TLA_GO_JUNIT_ROOT TLA_GO_JUNIT_JAR
git -C "$TLA_GO_JUNIT_ROOT" rev-parse HEAD > "$D/upstream-head.txt" || exit 1
[ "$(cat "$D/upstream-head.txt")" = 5a47802b5c391f59ecdd44117981f4ff8c0656ba ] || exit 1
git -C "$TLA_GO_JUNIT_ROOT" status --porcelain --untracked-files=no > "$D/upstream-status.txt" || exit 1
[ ! -s "$D/upstream-status.txt" ] || exit 1
shasum -a 256 "$TLA_GO_JUNIT_JAR" > "$D/jar-sha256.txt" || exit 1
[ "$(awk '{print $1}' "$D/jar-sha256.txt")" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 ] || exit 1
mkdir -p "$D/src-9f" "$D/src-51" "$D/runs" || exit 1
git archive 9f5816269083eb7c9d44e5477695e95e6bf68059 > "$D/src-9f.tar" || exit 1
git archive 51c3a77c6c4cbc02c6f388b0c90047cc895f9869 > "$D/src-51.tar" || exit 1
tar -xf "$D/src-9f.tar" -C "$D/src-9f" || exit 1
tar -xf "$D/src-51.tar" -C "$D/src-51" || exit 1
MISE_AUTO_INSTALL=false mise exec -- go build -C "$D/src-9f" -buildvcs=false -o "$D/tla-9f" ./cmd/tla-go || exit 1
MISE_AUTO_INSTALL=false mise exec -- go build -C "$D/src-51" -buildvcs=false -o "$D/tla-51" ./cmd/tla-go || exit 1
cat > "$D/JUnitTLC.java" <<'JAVA'
class JUnitTLC {
  public static void main(String[] args) throws Exception {
    tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE;
    tlc2.TLC.main(args);
  }
}
JAVA
MISE_AUTO_INSTALL=false mise exec -- go run docs/investigations/testdata/continue-51c3a77/reproduce.go > "$D/continue-summary.log" || exit 1
[ "$(wc -l < "$D/continue-summary.log")" -eq 24 ] || exit 1
set -- "$D"/runs/*/graph.dot
[ "$#" -eq 24 ] || exit 1
for task_file do
  MISE_AUTO_INSTALL=false mise exec -- go run ./docs/investigations/continuecanon "$task_file" > "$task_file.canonical" || exit 1
  cmp "$task_file.canonical" docs/investigations/testdata/continue-51c3a77/graph.canonical || exit 1
done
```

The re-run script implements the same flags and 30-second limit as the
experiment, but was prepared after the measured run. Binary hashes from that
run were `37f87e62b64cbf354ebabecf375f6aaf3d76be2d52a76da358ced71fc9d89f18`
for 9f and `12f7f5da76cfd2913c84dfb0e137bafed035af55184c6476f9ddc7c236c5e16e`
for 51; a different toolchain or build environment can produce different
binary hashes. Java uses coverage and Go does not, matching the diagnostic
harness, so the comparison remains partial.

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
