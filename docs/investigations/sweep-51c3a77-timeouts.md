# Follow-up on 51c3a77 diagnostic changes

The 51c3a77 sweep recorded `pcal.SimpleMultiProcTest` as unavailable after
its Go process hit the 30-second limit, and
`tlc2.tool.liveness.CodePlexBug08EWD840FL2Test` as unavailable after its Java
process hit the same limit. The previous 9f581626 snapshot had completed
selected comparisons for both. Neither snapshot is a JUnit pass. The sweep's
raw log has SHA-256
`606f46f5a51d30125f9b980a8eafccc03c40b219faff0194933eca28ac135598`.

In the original 51 run, Go's `SimpleMultiProc` had already reported 16 initial
states, 369680 generated and 103944 distinct states at depth 17, and started
the final temporal check before the deadline. In the CodePlex run, Java had
reported 1024 initial states and a progress message with 1092 generated and
1032 distinct states; the diagnostic does not launch Go when Java times out.
The logs do not establish why either process was slow.

On 2026-11-01 we ran two samples of each side for `SimpleMultiProc` in fixed
order, each followed by a Java sample for CodePlex. The second round also
verified the reproduction driver below. Inputs came from the clean pinned
upstream commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`, the verified
Java TLC 1.7.4 jar with SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`, and
clean detached Go worktrees at `9f5816269083eb7c9d44e5477695e95e6bf68059`
and `51c3a77c6c4cbc02c6f388b0c90047cc895f9869`. The machine was macOS arm64
with Go 1.27.3 and Temurin Java 21.0.11. Each process had a private directory
and DOT file. The 75-second limit for `SimpleMultiProc` and 45-second limit
for CodePlex were longer than the sweep's 30 seconds.

| Case | Side | First | Second | Exit | DOT SHA-256 |
| --- | --- | ---: | ---: | ---: | --- |
| SimpleMultiProc | Go 9f | 28.403 s | 30.483 s | 0 | `8ca3a600419f6eb8216e07ab9562544288f0a45a3bfa86f991e0a91704fbb43b` |
| SimpleMultiProc | Go 51 | 26.316 s | 26.874 s | 0 | `8ca3a600419f6eb8216e07ab9562544288f0a45a3bfa86f991e0a91704fbb43b` |
| SimpleMultiProc | Java | 18.896 s | 29.246 s | 0 | `8ca3a600419f6eb8216e07ab9562544288f0a45a3bfa86f991e0a91704fbb43b` |
| EWD840MC2 | Java | 20.761 s | 35.348 s | 13 | `a60c5ef6136d42ae401a29ea6d87f64f043446b91e10f85c34446b9523ba082c` |

The [first](testdata/sweep-51c3a77/first-run.txt) and
[second](testdata/sweep-51c3a77/second-run.txt) summaries retain the measured
durations, exit statuses, and raw DOT hashes.

For `SimpleMultiProc`, all three translations had the same TLA SHA-256
`b0795054543fbc7459d16da46c48628c818b03474c8c34c3418c4f9e77d92b62` and
configuration SHA-256
`0bbee4d0737023e5c7da5da7e1a276524449cebacac430d540f5a95f4f3f2da0`.
The second round reused copies of these verified translated inputs.
Each model-checking log reported 16 initial states, 369680 generated, 103944
distinct, no remaining states, depth 17, and the final liveness check. The
two Java CodePlex runs reported 1024 initial states, 15986 generated,
1566 distinct, no remaining states, and a temporal violation. Its exit 13 is
expected for that violation. It does not establish Go behavior in the new
snapshot, where Go was not launched.

Two samples per side cannot establish a speed change. Java had coverage enabled
and Go did not, as in the diagnostic harness. Both Go measurements were close
to 30 seconds; one Go 9f sample and one Java CodePlex sample exceeded it.
The limit and execution conditions can therefore affect availability;
the original scheduling and load were not reproduced. The selected messages
and DOT comparison do not prove JUnit lifecycle, coverage, translator mapping
or liveness-strategy equivalence. See the [earlier SimpleMultiProc study](simple-multiproc.md)
for additional bounded samples.

## Boolean coverage case

The two snapshots also differ for `tlc2.tool.coverage.BCoverageTest`. A
one-worker rerun of the pinned upstream `coverage/B.tla` and `B.cfg` used the
diagnostic flags, including `-deadlock -fp 0 -checkpoint 0 -dump dot` and
`-coverage 9999`. Java additionally received the harness's earlier
`-coverage 1`; the final explicit interval is 9999. Go 9f exited 255 with
`-coverage does not yet support this model's action or expression forms`.
Go 51 and Java both exited 0, reported 5 generated and 2 distinct states at
depth 2, and wrote byte-identical DOT, SHA-256
`791cb00bd31c906660b9d8e9a121281badefc9bd811a0de47d993b71aa44d5f3`.
Their coverage message records 2773, 2772 and 2221 were also byte-identical,
SHA-256 `fdd9342ada0a34d91db4f08113726aa159253d129ef80eec8f709b3738b678bd`.
This is a reproduced improvement for this particular model and selected
output. `TestJavaTLCUpstreamBooleanCoverage/upstream` also compares its Go
and Java coverage; it passed against the pinned jar. The Java JUnit case is
run separately; these checks do not establish a Go JUnit lifecycle pass or
general coverage parity.

The fourth changed record, `ContinueTest`, is discussed in the separate
[DOT investigation](continue-dot-sweep.md#follow-up-at-51c3a77). Its sampled
raw output ordering varies on Java, while the captured labeled graphs match.

## Reproduce

This follow-up uses the same `TLA_SWEEP51_DIR` setup and
`JUnitTLC.java` wrapper as the [Continue investigation](continue-dot-sweep.md#follow-up-at-51c3a77).
Start with a fresh path and build the two pinned Go binaries as described there.
Verify the checkout revision and jar hash before running. The following prepares
fresh directories, translates each model with its corresponding implementation,
and preserves the per-run logs and DOT files. The Go driver refuses to overwrite
an existing `run.log`, `graph.dot`, or `graph_liveness.dot` and enforces the
limits shown above. Java can also create large `graph_liveness.dot` and state
metadata files; remove the named generated files after preserving any needed
evidence.

```sh
D=${TLA_SWEEP51_DIR:?run the setup from the Continue investigation first}
T="$TLA_GO_JUNIT_ROOT/tlatools/org.lamport.tlatools/test-model"
mkdir "$D/simple" "$D/codeplex" || exit 1
mkdir "$D/simple/9f" "$D/simple/51" "$D/simple/java" || exit 1
for side in 9f 51 java; do
  cp "$T/pcal/SimpleMultiProc.tla" "$D/simple/$side/" || exit 1
  if [ "$side" = java ]; then
    (cd "$D/simple/$side" && java -cp "$TLA_GO_JUNIT_JAR" pcal.trans -unixEOL -wf -termination SimpleMultiProc.tla > translate.log 2>&1) || exit 1
  else
    (cd "$D/simple/$side" && "$D/tla-$side" -translate -unixEOL -wf -termination SimpleMultiProc.tla > translate.log 2>&1) || exit 1
  fi
done
cp "$T/CodePlexBug08/EWD840.tla" "$T/CodePlexBug08/EWD840MC2.tla" "$D/codeplex/" || exit 1
cp "$T/CodePlexBug08/EWD840MC2.cfg" "$D/codeplex/" || exit 1
MISE_AUTO_INSTALL=false mise exec -- go run docs/investigations/testdata/sweep-51c3a77/reproduce.go > "$D/long-summary.log" || exit 1
[ "$(wc -l < "$D/long-summary.log")" -eq 4 ] || exit 1
shasum -a 256 "$D"/simple/*/SimpleMultiProc.tla "$D"/simple/*/SimpleMultiProc.cfg || exit 1
```

The driver is equivalent to the temporary Go harness used for the first round;
the second round used the checked-in driver after preparation. Timings include
process startup and output. The per-run logs remain in ephemeral directories
and may disappear. The large SimpleMultiProc DOT files were removed after
hashing to reclaim space; the hashes are retained in the summaries above.
Never infer completion from a partially written DOT or log: inspect the process
status and the timeout as well.

To repeat the bounded Boolean coverage comparison in fresh directories:

```sh
mkdir -p "$D/coverage"
for side in 9f 51 java; do
  dir="$D/coverage/$side"
  mkdir "$dir" || exit 1
  cp "$T/coverage/B.tla" "$T/coverage/B.cfg" "$dir/" || exit 1
  if [ "$side" = java ]; then
    (cd "$dir" && java -cp "$TLA_GO_JUNIT_JAR" "$D/JUnitTLC.java" -tool -deadlock -fp 0 -coverage 1 -workers 1 -checkpoint 0 -dump dot "$dir/graph.dot" -coverage 9999 B > run.log 2>&1)
  else
    (cd "$dir" && "$D/tla-$side" -tool -deadlock -fp 0 -workers 1 -checkpoint 0 -dump dot "$dir/graph.dot" -coverage 9999 B > run.log 2>&1)
  fi
  task_exit=$?
  printf '%s exit %s\n' "$side" "$task_exit"
  case "$side" in 9f) expected=255 ;; *) expected=0 ;; esac
  [ "$task_exit" -eq "$expected" ] || exit 1
done
shasum -a 256 "$D"/coverage/*/graph.dot || exit 1
for side in 51 java; do
  awk '/STARTMSG (2773|2772|2221):/{p=1} p{print} p && /ENDMSG/{p=0}' "$D/coverage/$side/run.log" > "$D/coverage/$side/coverage.txt" || exit 1
done
cmp "$D/coverage/51/coverage.txt" "$D/coverage/java/coverage.txt" || exit 1
```

The commands print the exit statuses; Go 9f is expected to exit 255, while Go
51 and Java exit 0. The output filter compares the selected coverage records,
not the entire tool protocol or timestamps.
