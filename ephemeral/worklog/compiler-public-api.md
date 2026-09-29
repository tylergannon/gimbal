# Compiler public API worklog

decision: Stage 3 follows the accepted writeup; a separate consumer and maintenance edit are the outcome.
correction: Preserve the user stopping rule of 90–95% after the main outcome works; avoid repeated broad review without new material failure.
friction: unified command execution failed with OS error 24 before edits; node_repl child_process is working as the local runner.
friction: Merging main during the Plan schema case-only rename failed and restored lowercase files on macOS, producing an embedded-schema startup panic. Finish case renames and generation before Git integration; then rebuild before live proof.
decision: The Temporal emitter now lives only in the separate consumer module; root paired/source-edit regressions invoke that CLI. Public Gimbal owns admission and independent graph extraction.
friction: The independent maintenance run found that changing the deliverable also requires updating its independent expected-output assertion. The guide now includes that test-contract edit rather than deriving expectations from implementation.
correction: The initial live Claude prompt invited unnecessary extra workflow execution. Tighten the example to a one-sentence summary with no tools; shell creation/check remains authored workflow behavior.
decision: Integrated origin/main before final validation. Full hook run passed Go source-mutation suite, consumer tests and 113 frontend tests; the only failure was errcheck on a test file close, repaired and verified by full golangci-lint plus affected public-host test. Do not repeat the 285-second Go suite solely to create the checkpoint.
