These models exercise `-generateSpecTE`. They were written for tla-go; the
`java-SpecTE.tla` and `java-SpecTE.cfg` files were produced from them by Java
TLC 1.7.4 (TLA+ Tools commit `5a47802b5c391f59ecdd44117981f4ff8c0656ba`) with
`-generateSpecTE -workers 1 M`, plus `nomonolith` for the `nomonolith` case.

The files are normalized: the tool output that Java TLC writes at the start of
`SpecTE.tla` is removed through the final `2186` message, the time-based
`def_ov_` identifier is replaced by `def_ov_ID`, and the generation date by
`DATE`. Java TLC writes SpecTE files while a second thread is still parsing
its own output, so it can omit trailing states; the `liveness` output was kept
from a run that recorded the whole counterexample. The cases cover an
invariant, a liveness loop, stuttering, deadlock, an action property, a failed
`Assert`, model-value constants, an extended user module with and without
`nomonolith`, and, in `instance`, an instantiated module next to nested
extended modules, which SANY parses after every `EXTENDS`, and, in `long`,
values wider than 80 columns, which Java TLC lays out over several lines.

The `instance/java-partial-SpecTE.tla` fixture records a two-state Java output
from [CI job 124854171910](https://github.com/upamune/tla-go/actions/runs/41308589763/job/124854171910).
The model's full trace has three states, but the Java SpecTE writer can race the
tool-output consumer and capture only the first two. The partial trace suffix
was also reproduced with the pinned JAR's `TraceExplorer` using the complete
tool output with the last state removed. The live Java comparison tries three
times for the full fixture; if every produced file is the exact partial
fixture, it reports that the live comparison is partial and does not establish
exact compatibility. This exception requires the pinned JAR's SHA-256. A
timed-out Java process or an unexpected exit is retried without accepting its
generated file; if no attempt completes with the expected exit code and a
matching file, the test fails. Other differences still fail. tla-go's generated
output is always compared against the complete three-state fixture.
