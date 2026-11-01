# Operator coverage fixtures

These small models exercise the same operator and quantified-expression cost
model as the upstream `C.tla` fixture. The complete `M.coverage` reports were
captured with Java TLC 1.7.4 (`tla2tools-v1.7.4.jar`), with one worker and
coverage interval zero. See the adjacent `upstream-coverage/README.md` for the
upstream revision. The repeated-call report is retained although Go rejects
that model. Java emits no coverage report for the initial invariant violation
in `failure`; the test checks that behavior directly when `TLC_JAR` is set.
The `nested_short_circuit` report also exercises nested, partially evaluated
property subtrees in both normal and `-tool` output.
