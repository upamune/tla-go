# SimpleMultiProc diagnostic sweep follow-up

## Question and scope

The e81 sweep recorded `pcal.SimpleMultiProcTest` as unavailable because the
Go process hit its 30-second timeout. The 9f sweep recorded no differences in
its selected messages and exits. Neither is a JUnit pass. This investigates
whether the changed record establishes an improvement or a compatibility bug.
For the later 51c3a77 timeout and a CodePlex Java timeout, see the
[bounded follow-up](sweep-51c3a77-timeouts.md).

The pinned upstream checkout is `5a47802b5c391f59ecdd44117981f4ff8c0656ba`;
the oracle jar SHA-256 is
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`.
See upstream [SimpleMultiProcTest.java](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/test/pcal/SimpleMultiProcTest.java),
[model](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/test-model/pcal/SimpleMultiProc.tla),
[PlusCal setup](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/test/pcal/PCalModelCheckerTestCase.java),
and [TLC setup](https://github.com/tlaplus/tlaplus/blob/5a47802b5c391f59ecdd44117981f4ff8c0656ba/tlatools/org.lamport.tlatools/test/tlc2/tool/liveness/ModelCheckerTestCase.java).

The constructor selects `SimpleMultiProc`, `pcal`, and translator options
`-wf -termination`. JUnit's `@Before` translates with `-unixEOL`, checks
translation success, mapping and message count, then runs TLC with no deadlock
checking, fingerprint 0, coverage, one worker, no checkpoints and a DOT dump.
It sets `livenessThreshold` to `Double.MAX_VALUE`, vetoes cleanup, installs a
recorder and runs TLC before the test method. The method checks 16 initial
states, complete-space temporal check with 103944 distinct states, finished,
no general error, statistics 369680/103944/0 and depth 17. `@After` checks
successful exit. The generated configuration specifies `Spec` and property
`Termination`.

The diagnostic wrapper sets the Java liveness threshold, translates copies and
compares selected messages, exit and dump bytes, but does not run this JUnit
lifecycle, verify its translation mapping/messages or Java coverage assertions,
or enable Go coverage. Java uses coverage and Go does not. The Go liveness
strategy has not been established equivalent in general. The DOT path/cleanup
also differs. These are limits on inference, not demonstrated model defects.

The raw e81 log contains both outputs: Java reported 21,714 ms, and Go reported
16 initial states, then 369680/103944/0 at depth 17 and began the complete
liveness check just before timeout. The 9f log reports Java/Go exit 0, no
selected differences and no extracted assertion failures; it does not retain
successful full output or timings. Raw log SHA-256s are
`65e957a50c1a58779f5982458ca42adb3d4ed16e0884229dd7bbe752aa298765`
and `e548e0fba387f59c1278069d2bb0bcd7196475e36a6049003336d2616bb13000`
respectively.

## Reproduction

The following commands use the pinned upstream checkout and jar described in
`internal/upstreamjunit/README.md`. Run them from this repository in a POSIX
shell with Go managed by mise and a JDK on PATH. They create a new private
`mktemp` directory and do not write to the checkout or reuse earlier results.
Set the two input paths if necessary. Verify the printed commit and jar hash
against the values above before continuing. `git show` reads the model at the
pinned commit even if the upstream working tree has local edits.

```sh
TLA_ROOT=${TLA_GO_JUNIT_ROOT:-/tmp/tlaplus-java-v1.7.4}
TLA_JAR=${TLA_GO_JUNIT_JAR:-/tmp/tla2tools-v1.7.4.jar}
TLA_ROOT=$(cd "$TLA_ROOT" && pwd -P) || exit 1
TLA_JAR=$(cd "$(dirname "$TLA_JAR")" && pwd -P)/$(basename "$TLA_JAR")
RUN_DIR=$(mktemp -d "${TMPDIR:-/tmp}/simplemultiproc.XXXXXXXX") || exit 1
RUN_DIR=$(cd "$RUN_DIR" && pwd -P) || exit 1
printf 'Results: %s\n' "$RUN_DIR"
git -C "$TLA_ROOT" rev-parse HEAD
shasum -a 256 "$TLA_JAR"
```

Build each Go revision from its immutable Git object, independently of the
current working tree. The wrapper sets Java's final-only liveness threshold.

```sh
for entry in e81:e81f221f7ba59b833244e7613bd149f836532c86 9f:9f5816269083eb7c9d44e5477695e95e6bf68059 current:e168ed355a379ca0497a09cf376ab73a2c58b43d; do
  name=${entry%%:*}; revision=${entry#*:}
  git cat-file -e "$revision^{commit}" || exit 1
  mkdir "$RUN_DIR/src-$name" || exit 1
  git archive "$revision" | tar -x -C "$RUN_DIR/src-$name" || exit 1
  mise exec -- sh -c 'cd "$1" && go build -buildvcs=false -o "$2" ./cmd/tla-go' sh "$RUN_DIR/src-$name" "$RUN_DIR/tla-$name" || exit 1
done
cat > "$RUN_DIR/JUnitTLC.java" <<'JAVA'
class JUnitTLC { public static void main(String[] args) throws Exception { tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE; tlc2.TLC.main(args); } }
JAVA
shasum -a 256 "$RUN_DIR"/tla-*
```

Translate separate copies, then run three samples in the historical order.
Set `SAMPLES=1` before the last block for a smaller smoke run. Each sample gets
a new directory and log; `mkdir` refuses to reuse an existing one. The shell
prints each exit status, and `/usr/bin/time -p` appends wall time to the log.
Inspect every status and log: a nonzero exit, interruption, or incomplete output
is not a successful comparison. These commands have no automatic timeout;
interrupt a stuck run manually. Wall time includes startup and output and is
not a microbenchmark.

```sh
for name in e81 9f current java; do
  mkdir "$RUN_DIR/$name" || exit 1
  git -C "$TLA_ROOT" show 5a47802b5c391f59ecdd44117981f4ff8c0656ba:tlatools/org.lamport.tlatools/test-model/pcal/SimpleMultiProc.tla > "$RUN_DIR/$name/SimpleMultiProc.tla" || exit 1
  if [ "$name" = java ]; then
    (cd "$RUN_DIR/$name" && java -cp "$TLA_JAR" pcal.trans -unixEOL -wf -termination SimpleMultiProc.tla > translate.log 2>&1) || exit 1
  else
    (cd "$RUN_DIR/$name" && "$RUN_DIR/tla-$name" -translate -unixEOL -wf -termination SimpleMultiProc.tla > translate.log 2>&1) || exit 1
  fi
done
SAMPLES=${SAMPLES:-3}
sample=1
while [ "$sample" -le "$SAMPLES" ]; do
  for name in e81 9f current java; do
    dir="$RUN_DIR/$name/run$sample"
    mkdir "$dir" || exit 1
    cp "$RUN_DIR/$name/SimpleMultiProc.tla" "$RUN_DIR/$name/SimpleMultiProc.cfg" "$dir/" || exit 1
    if [ "$name" = java ]; then
      (cd "$dir" && /usr/bin/time -p java -cp "$TLA_JAR" "$RUN_DIR/JUnitTLC.java" -tool -deadlock -fp 0 -coverage 1 -workers 1 -checkpoint 0 -dump dot "$dir/run.dot" SimpleMultiProc > run.log 2>&1)
    else
      (cd "$dir" && /usr/bin/time -p "$RUN_DIR/tla-$name" -tool -deadlock -fp 0 -workers 1 -checkpoint 0 -dump dot "$dir/run.dot" SimpleMultiProc > run.log 2>&1)
    fi
    status=$?
    printf '%s sample %s exit %s\n' "$name" "$sample" "$status"
    [ "$status" -eq 0 ] || exit "$status"
  done
  sample=$((sample + 1))
done
```

Each side translates its own model; Java alone receives `-coverage 1`. Compare
the translation hashes, DOT hashes, timings and relevant messages manually.
The command output is evidence to inspect, not an automated pass verdict.

```sh
shasum -a 256 "$RUN_DIR"/*/SimpleMultiProc.tla "$RUN_DIR"/*/SimpleMultiProc.cfg
shasum -a 256 "$RUN_DIR"/*/run*/run.dot
rg 'Finished computing initial states|Checking temporal properties for the complete|states generated, 103944 distinct|depth of the complete|Finished in|GENERAL|^real ' "$RUN_DIR"/*/run*/run.log
```

## Results (2026-11-01)

These historical results were produced with an ephemeral Python runner; the
commands above are a manual reproduction and have not remeasured the table.
The original `results.jsonl`, per-run logs, and DOT files were saved under
`/tmp/simplemultiproc-investigation`; that temporary directory may disappear.
The current revision was `e168ed355a379ca0497a09cf376ab73a2c58b43d`.
Runs used Go 1.27.3, Temurin Java 21.0.11, macOS 26.6 on Apple M3,
with the same jar and upstream commit as the sweeps. All processes completed
within 90 seconds with exit 0. Times are seconds, in execution order:

| Implementation | Sample 1 | Sample 2 | Sample 3 |
| --- | ---: | ---: | ---: |
| e81f221f | 22.469 | 21.680 | 26.600 |
| 9f581626 | 21.625 | 20.860 | 26.120 |
| e168ed3 | 21.218 | 21.925 | 25.431 |
| Java 1.7.4 | 16.296 | 19.310 | 22.893 |

All translations produced identical bytes: TLA SHA-256
`b0795054543fbc7459d16da46c48628c818b03474c8c34c3418c4f9e77d92b62`,
configuration SHA-256
`0bbee4d0737023e5c7da5da7e1a276524449cebacac430d540f5a95f4f3f2da0`.
Each of the 12 tool logs reported 16 initial states, the complete temporal
check on 103944 states, statistics 369680/103944/0, depth 17, finished, and no
`GENERAL` message. All 12 DOT files had SHA-256
`8ca3a600419f6eb8216e07ab9562544288f0a45a3bfa86f991e0a91704fbb43b`.
The e81 timeout is consistent with a 30-second boundary affected by runtime
variation: it reached the final temporal check before the deadline in the
original log and completed in all three reruns. In these samples 9f was
0.48–0.84 seconds faster than e81 in corresponding rounds, while e81's own
range was 4.92 seconds and all implementations slowed in round three. This
small, fixed-order sample cannot establish whether an actual speed change
exists or identify a cause. It also cannot reproduce the original machine
load. Java's timing is not a controlled speed comparison with Go because
coverage and other implementation details differ.

No concrete semantic defect is isolated by these observations. Identical
selected output and dumps for this model do not prove full semantic or JUnit
lifecycle equivalence, especially for coverage, translator mapping, and
liveness scheduling. No production change or new oracle test is justified by
the timeout alone. For a performance attribution, use a larger randomized,
controlled benchmark and profile the relevant revisions, with a timeout that
does not censor completion.
