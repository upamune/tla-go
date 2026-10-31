`Detlefs.tla` is unchanged from `test-model/pcal` in TLA+ Tools commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` (Java TLC 1.7.4); it already
contains its PlusCal translation. `Detlefs.cfg` is the upstream configuration
with `Address = {a1, a2}` instead of four addresses, and with
`CHECK_DEADLOCK FALSE` in place of the `-deadlock` option used by
`pcal.DetlefsTest`. Java TLC 1.7.4 reports 23,821 states generated, 10,149
distinct states and depth 42 for it. See
[UPSTREAM-LICENSE](../../../UPSTREAM-LICENSE).
