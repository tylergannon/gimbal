# project.jsonl sequence numbers collide across concurrent runs

URL: https://github.com/tylergannon/gimble/issues/118
State: closed
Updated: 2026-09-14T00:43:44Z

`Run` opens its own `eventWriter` for `<project>/project.jsonl` (`run.go`), and each `eventWriter` keeps its own in-memory `seq` counter starting at 0 (`event_persistence.go`). Two runs of different workflows (or the same one) active at once in one project each write `run_started`/`run_ended`/`run_cancelled` to the shared `project.jsonl` with their own independent, overlapping `seq` values (both start at 1, 2, ...).

`Event.Seq` is documented as the ordinal within a log, and `TestAttestEventFixture` checks strict `seq` monotonicity for `run.jsonl` (one writer, one run) but nothing exercises two concurrent runs writing `project.jsonl`. Whoever reads `project.jsonl` (Sprint 3's `watchRuns`) will see non-monotonic or duplicate `seq` values once more than one run is live at a time.

Give `project.jsonl` one shared writer/counter per project (mirroring "one writer per run" for the project-wide stream), or drop `Seq` from project-level events and rely on `Time` there.
