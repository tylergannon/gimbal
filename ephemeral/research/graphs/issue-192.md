# runlog.Read fails at once when the run directory exists but run.jsonl is not yet written

URL: https://github.com/tylergannon/gimble/issues/192
State: open
Milestone: None
Updated: 2026-09-14T01:14:11Z

Found in the loop practice runs (#178): shape 2 run `01M2EQC7S1CTSVRBX7KRR83SXN.validation-command` and shape 4 run `01M2EQFH6EPBFAB7KVE4RW6D4P.group-in-task`.

A program that watches a run the way the page does learns the run id from the directory `gimble.Run` makes under `runs/`, then follows the log with `runlog.Read[gimble.LifecycleRecord](ctx, dir, ...)`. `ephemeral/attest/registry/main.go` and the four programs under `ephemeral/attest/loop-practice/` all do this.

**Expected:** `runlog.Read` on a run directory that exists follows `run.jsonl` from its first record, as its doc says ("replays the run log at dir and follows it until a record whose kind is complete is observed").

**What happened:** `run.go` creates the run directory before the log's first record is written. `runlog.Read` opens `run.jsonl` once (`internal/runlog/reader.go:24`) and returns the `os.Open` error when the file is not there yet. A watcher that starts between the two returns at once with ENOENT and sees nothing. In shape 2's first run the watcher printed none of its `[log]` lines; in shape 4's first run the operator goroutine that was to kill attempt 2 by id never saw `turn_started` and no kill was sent, so the run ended clean with three finished attempts. The loop-practice programs now wait for `run.jsonl` to exist before calling Read, which is the workaround.

Either `Read` waits for the file while the ctx lives (the directory is the run's promise that the log is coming), or the run writes `run.jsonl`'s first record before the directory is visible under `runs/`.
