# Adversarial review: process-leak diagnostics, round 04

## Target

Source revision a251b230 (code identical to 0d5cdac5; a251b230 adds notes only). Working tree clean except the untracked round-03 review. The built `bin/gimbal` reports `vcs.revision=a251b230570d…`, `vcs.modified=false`. The review is against `ephemeral/plans/process-leak-diagnostics.md`, `AGENTS.md`, and the proof in `/private/tmp/gimbal-process-debug-proof-c403fdfd/observations.md`.

The launch prompt imposed only operating constraints (no edits, no shared service lifecycle changes, luna only, no unrelated transcripts). I took no narrowing of scope from it.

## Evidence inspected and independently produced

- I parsed the three named diagnostic logs line by line (`process-4123829119`, `-4189735501`, `-765276347`) rather than reading the summary.
- I ran `GIMBAL_PROCESSDEBUG_LONG_TEST=1 go test -race -run 'TestLongServiceGrace|TestSharedDaemonStandin|TestSurvivingOwnedChild' -count=1 -v ./internal/processdebug`: **exit 0**, all three pass, including the real 10s/60s service grace (61s). Output is in `/private/tmp/r4-longtest.log`, outside the repository. The user cache directory still held exactly the four proof logs afterwards (the tests isolate `HOME`).
- I started my own flag-on workflow against the live proof central (PID 9225, disposable state): `gimbal run review … --code-review luna` produced run `01M6D8FYKMW2YWF7QNHV2XS8R1`, which ended `ok` in 8s. The model was gpt-6-luna, selected by `luna`. I then waited through the 60s grace and read the new records in the existing parent log.
- I did not stop or restart any service. The proof central (PID 9225) and the shared Codex daemon (PID 48413) are untouched. I did not inspect any transcript.

## Observed claims: checked against the logs and my own run

| Claim in observations.md | Result |
| --- | --- |
| Parent log has start, owned, reaped and owner_end; first-census delays 169 and 84 ms; baseline 7 | Confirmed. |
| Seven baseline children are labelled baseline, not candidates or owned | Confirmed (`baseline`, `baseline_alive_count=7`). |
| Survivor and candidate counts are 0 at immediate, 10s and 60s | Confirmed for both original turns. |
| Worker-local shared log: delay 132 ms, baseline 14, two candidate survivors at immediate owner end only | Confirmed. |
| Service cancellation log shows stop_requested, stop_observation, reaped, owner_end, with no suspect_survivor | Confirmed. |
| Flag off creates no diagnostic log | Consistent. The cache directory holds no file written in the off-run window, and the single pre-existing file is unchanged. I did not re-run flag-off against a fresh central. |
| All three logs are mode 0600 with the stated sizes | Confirmed. |

My own run reproduced the same shape:
- Baseline 7 (first-census delay 62 ms).
- Candidates appeared only after the baseline; they were the Codex tool processes `node`, `node_repl`, `vp`, `uv` and `Python`.
- At 0s, 10s and 60s: `survivor_count=0`, `candidate_count=0`, `baseline_alive_count=7`.
- Both the agent worker and the close worker were reaped before owner end.
- `run` is `unavailable` and `session` is present in every owner, as documented.

## Findings

### 1. issue: `reaped` for a shared-daemon owner reports the daemon's PID when the reaped process is the Gimbal worker

- Where: `internal/processdebug/process.go`, the `case <-reap` branch writes `event{Event: "reaped", PID: pid}`. For shared owners `pid` was reassigned to `daemon.PID` after discovery, and `executor.Launch` calls `candidates.Reaped()` when the worker is reaped (`internal/executor/files.go`, wait goroutine).
- Reproduction (both the original proof logs and my run): in `process-4123829119.jsonl`, each turn has two `reaped` lines at the same millisecond, one with `pid` 48413 (the shared daemon) and one with the worker PID.
  - Original turn: `04:24:02.668`, 48413 and 10346. Close worker: `04:24:02.797`, 48413 and 11333.
  - My run: `04:32:53.369`, 48413 and 22857.
- The daemon (48413) stayed alive throughout every one of those runs: it appears as `shared_daemon` in each later census and is still the process serving the next turn.
- Impact: the diagnostic states that the shared daemon was reaped, which is false and is the opposite of the plan's rule that borrowed daemons have their own lifetime. A reader auditing the log for exit and reap observations would be misled. `owner_end` with the daemon PID has the same ambiguity but is less wrong, since that is the owner's own end.
- Fix direction: do not log a `reaped` event for shared owners, or log it with the worker's PID and a kind that says the reaped process is the launching worker, not the sampled daemon.

### 2. nitpick: the worker-local shared observer produces misleading, low-value evidence

- In the worker-local log, the first census (132 ms) already includes this turn's own MCP and tool processes, which the parent log saw as `candidate` at 04:23:53.94. So the 14 baseline entries include 7 processes created by the turn itself (PIDs 10456 to 10503).
- The worker exits at owner end, so it records only the immediate sample, where daemon teardown is in progress. It reports `candidate_survivor` for PID 10709 `node` (which the parent saw gone 300 ms later) and for 11330 `SkyComputerUseClient`, an unrelated Desktop helper that appeared in the same second.
- These are correctly labelled candidates, with the uncertainty text, and the parent-side observer supersedes them. But the plan says not to conflate unrelated Desktop clients with Gimbal work, and the worker-local log is the one place it happens in the proof. The worker-local observer duplicates the parent-side observer and could be dropped. This is the only place the proof shows a candidate that was plausibly not Gimbal's.

### 3. nitpick: proof gaps to state, not defects

- **No positive survivor on the built binary.**
  - Every built-app run shows zero survivors. The deliberate surviving child, PID reuse, missing identity, stale result, capacity, and service grace at 10s/60s are demonstrated only by package tests.
  - That matches the plan ("real subprocess tests demonstrate"), and I re-ran those tests green. It should be said plainly in the PR body.
- **No tool-using provider turn in the built-app proof.**
  - The tool processes seen are Codex's own MCP processes, not a turn-issued command. My run executed no command by design.
- **Run correlation is absent for agent turns.** `run` is `unavailable` for all Codex owners, which the plan allows.
- **Linux enumeration is compile-checked only**, as the proof states.
- **The flag-off built-app case rests on the recorded cache comparison.** The user-cache listing is consistent with it, but I did not re-run it.

### 4. nitpick: standing items from earlier rounds, still true

- `dropped_owner` is replayed per observer.
- Diagnostic plumbing is spread across `Launch`, `commandexecutor.Request.DiagnosticRun` and `taskdaemon`.
- The unrelated De Morgan rewrites and the `//nolint:errcheck` comment on a channel receive remain.

## Not found

- Flag off creates no logs, goroutines or subprocesses. The constructors return nil, and the cache directory is unchanged by the off-run.
- No process was killed, retried or gated. Nothing changed any run result: my run ended `ok` with no recording error.
- Baseline, candidate, owned and survivor labels follow the plan, and the counts agree with the independent process census I could read (7 baseline on each turn).
- The tests that I re-ran are repeatable and stable under isolated subprocesses.

## Outcome

material findings remain

(Finding 1 is a small, verifiable misreport; everything else the plan requires is demonstrated on the built binary and in package tests.)
