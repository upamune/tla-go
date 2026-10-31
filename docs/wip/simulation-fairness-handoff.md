# Simulation fairness handoff

This branch is unfinished. The goal remains compatibility with upstream TLA+ and Java TLC, including the upstream TLC tests. Read AGENTS.md and docs/compatibility.md before continuing. Use mise and keep compatibility tests traceable to the Java TLC commit pinned in README.md.

The branch experiments with one pure weak or strong fairness condition in single-worker simulation. It deliberately rejects multiple fairness conditions, multiple properties with fairness, and fairness with leads-to. Differential probes for the rejected combinations found differences in both verdicts and counterexamples; their Go test source is saved in `docs/wip/simulation-fairness-probes.txt` for investigation, not enabled as a passing test suite.

Locally, `mise run format:check`, `mise run lint`, and `mise run test` passed. The Java TLC 1.7.4 fairness suite had one failure because the Java output contained an extra progress message 2209 while other tests were running concurrently; the affected test passed in isolation. Rerun the differential suite with the pinned upstream JAR, diagnose or account for asynchronous progress reporting, and check that any changes remain compatible. Java TLC 1.8.0 may differ. Fairness-free simulation differential tests also passed locally.

The SECURITY.md badge table was merged directly to main in commit 2c578b7. The main worktree retains its uncommitted copy of the experimental changes. This branch is only a handoff snapshot; do not merge it without completing verification.
