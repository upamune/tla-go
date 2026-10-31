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
