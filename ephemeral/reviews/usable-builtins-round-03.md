# Adversarial review: usable Gimble builtins, round 03

Outcome: **no findings** in this narrow follow-up.

## Target and evidence

This round checks only the round-02 planning-check handoff finding and its focused tests. The CLI now passes `in.Checks` to `planning.Input` (`cmd/workflows.go:337`); planning keeps the commands in root scoped data (`internal/workflows/planning/planning.go:28,82`), includes each exact command in the common brief (`:135-154`), and tells draft, critique, and synthesis turns to preserve them (`:190,227,243`). The injected three-lane test supplies two distinct commands and asserts that every draft, critique, and synthesis prompt contains both (`internal/workflows/planning/planning_test.go:70-102`). `go test ./internal/workflows/planning ./cmd` and the focused test passed. No paid model was called in this follow-up.

The round-02 finding is closed at the input and prompt boundary. These deterministic tests prove handoff, not that a future model will obey the prompt in its saved prose; native planning with the changed code was not rerun. The earlier absolute-path startup failure cited in round 02 is preserved at `ephemeral/attest/builtins/sprint-path-failure.txt`; `ephemeral/attest/builtins/sprint-output.txt` now contains the successful Sprint run with the generated plan, final validation, and run completion. Round 02 remains unchanged as the historical finding.
