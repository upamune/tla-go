# DiameterTest: worker scheduling and the diagnostic comparison

At upstream revision `5a47802b5c391f59ecdd44117981f4ff8c0656ba`,
`tlc2.tool.DiameterTest` runs `DieHardTLA` with four workers. It asserts 97
states generated, 16 distinct states, no general error, finished, zero uncovered
expressions, and a reported search depth of **at least** 8. The test explicitly
says the depth can vary with multiple workers; 8 is the single-worker value.
The partial sweep snapshot at `51c3a77c6c4cbc02c6f388b0c90047cc895f9869`
records `message:2194` and `dump:content`. The snapshot does not preserve those
raw outputs, so the original difference cannot be reconstructed from it alone.
The sweep runs CLI invocations and selected comparisons, not the JUnit lifecycle
or all of its assertions, including the coverage assertion.

## Bounded comparison

On macOS arm64, Temurin 21.0.11, Go 1.27.3, with no other intentional load,
we ran the pinned model and Java TLC 1.7.4 against a binary built from clean
tracked source at `20dcf5d265d59cc4e4556ea0362b858df2929e73` before adding
this investigation. The Java jar SHA-256 is
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`.
The recorded Go binary SHA-256 is
`fd10bddebe07270e11f3780c13c792a12f8e6c4c25985d88fff536c2722bdc00`;
this build used the default Go build flags, so another build need not reproduce
the binary hash. The source and model hashes are given below. Each run completed
with exit 0 and 97 generated / 16 distinct / 0 queued states.

| Implementation | Workers | Runs | 2194 depth | Raw DOT |
| --- | ---: | ---: | --- | --- |
| Java | 1 | 3 | 8, 8, 8 | one SHA, equal to all Go samples |
| Go | 1 | 3 | 8, 8, 8 | one SHA |
| Java | 4 | 8 | 9 once, 8 seven times | eight distinct SHAs |
| Go | 4 | 8 | 8 each time | one SHA |

For this invocation, Go selects serial exploration: the DOT graph callback and
`SPECIFICATION` each exclude the current parallel path. Java's 4-worker output
interleaves graph writes and may give different DOT `rank` groupings. The
captured depth-9 DOT puts the final two vertices in separate ranks, while the
captured depth-8 and serial DOT place them in one rank. An exact 4-worker DOT
or depth comparison is therefore not a stable oracle. Do not mask those raw
sweep differences or claim this proves every run semantically equal.

All 22 captured DOTs canonicalized to the **same multiset** of 16 labelled
vertices (including the initial marker) and 96 directed edges including
repeats. The canonicalizer validates rank references but deliberately omits rank
from the output; equal canonical graphs do not imply equal layout, rank or
other unspecified DOT semantics. Its restricted grammar rejects other syntax.
The per-run data and hashes are in [samples.tsv](testdata/diameter-20dcf5d/samples.tsv).
The directory also preserves exact stdout/stderr combined and raw DOT for
Java single-worker, Java four-worker depths 8 and 9, and Go four-worker, plus
the canonical graph. All other runs' raw outputs remain local and are not
claimed as committed evidence. The captured outputs contain timing, process
and machine-specific metadata; those parts vary.

The Java result at depth 9 provides a concrete possible explanation for a
`message:2194` difference and the distinct raw DOTs provide a concrete possible
explanation for `dump:content`. They do not identify the actual original sweep
output, establish a distribution, prove all future runs equivalent, or show
that the upstream JUnit test passed. Go also still differs from Java in actual
parallel scheduling. No production behavior was changed to force a particular
four-worker output.

## Reproduction

This command builds the model checker and copies its model from an archive of
the pinned tla-go revision `20dcf5d`, regardless of the current checkout. It
builds the investigation's later canonicalizer from its single checked-in
source file and verifies that file and the expected graph by SHA-256. Untracked
files cannot change these builds or model inputs. It
uses a separately compiled copy of the sweep's `JUnitTLC` wrapper, which sets
`TLCGlobals.livenessThreshold` to `Double.MAX_VALUE`. The sweep launches the
same wrapper in Java source-file mode; neither this command nor the sweep runs
JUnit. Java alone receives `-coverage 1`, as in the sweep. Use the absolute path
to the pinned jar as `TLA_GO_JUNIT_JAR`.

```sh
(
set -eu
task_rev=20dcf5d265d59cc4e4556ea0362b858df2929e73
git cat-file -e "$task_rev^{commit}"
: "${TLA_GO_JUNIT_JAR:?set the absolute path to the pinned jar}"
case "$TLA_GO_JUNIT_JAR" in /*) ;; *) exit 1 ;; esac
test "$(shasum -a 256 "$TLA_GO_JUNIT_JAR" | cut -d ' ' -f 1)" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88
test "$(shasum -a 256 docs/investigations/continuecanon/main.go | cut -d ' ' -f 1)" = 08bfff5486f307108ab5c4d1213b7ad0c50782bd34a44e612b3ae2eebc2647ec
test "$(shasum -a 256 docs/investigations/testdata/diameter-20dcf5d/graph.canonical | cut -d ' ' -f 1)" = 7f0fba7372c5c286e21a3db0642f0b8c9bd249554d4bdaf16fdb394cebdabb95

task_repo="$PWD"
task_dir=$(mktemp -d "${TMPDIR:-/tmp}/tla-diameter.XXXXXXXX")
mkdir -p "$task_dir/classes" "$task_dir/model" "$task_dir/src"
git archive -o "$task_dir/source.tar" "$task_rev"
tar -xf "$task_dir/source.tar" -C "$task_dir/src"
test "$(shasum -a 256 "$task_dir/src/checker/testdata/java-tlc/DieHardTLA.tla" | cut -d ' ' -f 1)" = 562ee80ecd88901b916e3ba3b9a2d554143269036afad7888d55a97a9cc052bf
test "$(shasum -a 256 "$task_dir/src/checker/testdata/java-tlc/DieHardTLA.cfg" | cut -d ' ' -f 1)" = f7e0ca84f02c45751e75bde28431be5ff20d5d240b17100f5b6fd71e3387f9c8
cp "$task_dir/src/checker/testdata/java-tlc/DieHardTLA.tla" "$task_dir/model/"
cp "$task_dir/src/checker/testdata/java-tlc/DieHardTLA.cfg" "$task_dir/model/"
cat > "$task_dir/classes/JUnitTLC.java" <<'JAVA'
class JUnitTLC {
  public static void main(String[] args) throws Exception {
    tlc2.TLCGlobals.livenessThreshold = Double.MAX_VALUE;
    tlc2.TLC.main(args);
  }
}
JAVA
javac -cp "$TLA_GO_JUNIT_JAR" "$task_dir/classes/JUnitTLC.java"
(cd "$task_dir/src" && MISE_AUTO_INSTALL=false MISE_TRUSTED_CONFIG_PATHS="$task_dir/src" mise exec -- go build -o "$task_dir/tla-go" ./cmd/tla-go)
MISE_AUTO_INSTALL=false MISE_TRUSTED_CONFIG_PATHS="$task_repo" mise exec -- go build -o "$task_dir/canonical" ./docs/investigations/continuecanon/main.go
cd "$task_dir/model"
for task_workers in 1 4; do
  for task_impl in java go; do
    if [ "$task_workers" = 1 ]; then task_count=3; else task_count=8; fi
    for task_run in $(seq 1 "$task_count"); do
      task_prefix="$task_dir/$task_impl-$task_workers-$task_run"
      if [ "$task_impl" = java ]; then
        java -cp "$TLA_GO_JUNIT_JAR:$task_dir/classes" JUnitTLC -tool -deadlock -fp 0 -coverage 1 -workers "$task_workers" -checkpoint 0 -dump dot "$task_prefix.dot" DieHardTLA > "$task_prefix.out" 2>&1
      else
        "$task_dir/tla-go" -tool -deadlock -fp 0 -workers "$task_workers" -checkpoint 0 -dump dot "$task_prefix.dot" DieHardTLA > "$task_prefix.out" 2>&1
      fi
      "$task_dir/canonical" "$task_prefix.dot" > "$task_prefix.canonical"
      cmp "$task_prefix.canonical" "$task_repo/docs/investigations/testdata/diameter-20dcf5d/graph.canonical"
      grep -Fx '97 states generated, 16 distinct states found, 0 states left on queue.' "$task_prefix.out" > /dev/null
      task_depth=$(awk '/STARTMSG 2194/{getline; print}' "$task_prefix.out")
      case "$task_depth" in 'The depth of the complete state graph search is '*) ;; *) exit 1 ;; esac
      printf '%s %s run %s: %s\n' "$task_impl" "$task_workers" "$task_run" "$task_depth"
    done
  done
done
printf 'All 22 graphs matched the captured canonical graph. Outputs: %s\n' "$task_dir"
)
```

A rerun may observe only depth 8, or another valid multi-worker depth. Preserve
and inspect raw outputs when it does; raw DOT byte equality is not expected in
the Java four-worker runs. `TestJavaTLCDiameterSingleWorkerReference` checks
Go's one-worker depth and exact DOT against the pinned Java reference. For four
workers it accepts depth 8 or greater and compares vertices and edge multiset
after canonicalization, without requiring serial DOT order or rank. Its Java
single-worker comparison runs when `TLC_JAR` names the pinned jar. It does not
assert that Java four-worker depth or DOT bytes are deterministic.
