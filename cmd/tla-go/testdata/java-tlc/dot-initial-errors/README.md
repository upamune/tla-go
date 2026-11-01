The `etest4`, `etest10`, `etest11`, and `Continue` models are copied without
changes from `tlatools/org.lamport.tlatools/test-model` at upstream commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba` (Java TLC 1.7.4).

`Continue-java.dot` is the complete output of the 1.7.4 release jar (SHA-256
`936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`),
using `-deadlock -fp 0 -coverage 1 -workers 1 -checkpoint 0 -dump dot
Continue-java.dot -continue Continue`. The upstream ContinueTest uses three
workers; this file is a deterministic single-worker control, not an assertion
about the order of the original parallel execution. The upstream JUnit harness
also sets a liveness threshold and places the dump under its metadata directory;
the control does not claim to reproduce that lifecycle.
