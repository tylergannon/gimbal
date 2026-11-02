# Adversarial review: process-leak diagnostics, round 05

## Target

Source revision 64ca739d (HEAD; working tree clean apart from review artifacts). Runtime source is d7ce4722; 64ca739d adds notes only. The built `bin/gimbal` reports `vcs.revision=64ca739deb00…`, `vcs.modified=false`, so the binary matches HEAD. Changes since round 04: `11e9eee8` (shared reap and end reporting), `d7ce4722` (frozen shared cohort), `64ca739d` and `c2356e1c` (docs). They also include the Darwin NUL-terminated executable fix. The review is against `ephemeral/plans/process-leak-diagnostics.md` (which gained a "closed cohort" bullet), `AGENTS.md`, and `/private/tmp/gimbal-process-debug-proof-c403fdfd/observations.md`.

The launch prompt imposed only operating constraints (no repository edits, luna only, no shared service stop or restart, no unrelated transcripts). I took no narrowing of scope from it.

## Evidence inspected and independently produced

- The full diff since a251b230, with the whole current `observe` function and the new tests read line by line.
- The three named parent and worker logs in `closed-log-paths.json`, parsed line by line.
- My own run on the live proof central (PID 45116, `closed-instance`): `gimbal run review … --code-review luna`, run `01M6D9MVGJR7M8R87Z50A18RRS`. It ended ok, and I waited through its 60s grace. It used gpt-6-luna and ran no command, by instruction, so its session file has 0 `commandExecution` entries.
- Tests, run by me:
  - `GIMBAL_PROCESSDEBUG_LONG_TEST=1 go test -race -run 'TestLongServiceGrace|TestSharedDaemonStandin|TestSurvivingOwnedChild' -count=1 -v ./internal/processdebug`: **exit 0**. `TestSharedDaemonStandin` takes 15s with the long-mode post-end child check, and `TestLongServiceGrace` takes 61s.
  - `go test -race -count=2 ./internal/processdebug`: exit 0.
  - `go vet` on darwin and `GOOS=linux`: clean.
- Output went to `/private/tmp/r5-*.log`. The tests isolate `HOME`, so they added no files to the user cache.
- I did not stop or restart any service, and I did not touch the shared daemon (PID 48413).

## Observed claims

| Claim | Result |
| --- | --- |
| No new log contains `reaped` with the daemon PID 48413 | Confirmed across the three named logs and my run. Parent `reaped` events carry worker PIDs only, and shared ends are `observation_end` with no PID. Round 04 finding 1 is **fixed**. |
| No shared candidate is added after its `observation_end` | Confirmed by script over every named log, including mine. There were no late `candidate` events. |
| The shared cohort is frozen at end request or result publication | Confirmed by code (`canDiscover`, `endRequested`, `resultPublished`) and by the real shared-parent regression. A child born after the end stays alive past the 10s grace, and `candidate_count` stays 0 with no stderr warning. |
| Candidate counts settle to 0 at grace; early counts can be nonzero during native teardown | Confirmed. In the 04:49:57 turn, the parent logged six `candidate_survivor` events and `candidate_count=6` immediately. All six were gone at 10s and 60s (count 0). My own turn showed 0 at 0s, 10s and 60s. |
| Baseline processes are never candidates | Confirmed (`baseline_alive_count=7` throughout; none of the 7 appear as survivors). |
| The Darwin executable NUL fix | Confirmed. Logs written by the post-fix binary (`…2999156969`, `…3448185719`, `…2077874308`, `…2522170834`) contain 0 control-character executable names. The older pre-fix logs contain them and are not evidence against the current code. |
| An end before the first census is reported as unavailable | Confirmed by the passing early-end regression. |
| Correction: the original run did execute tool commands | **Accepted.** My round 04 statement that no tool turn existed was wrong. The original session file has 3 `commandExecution` entries, and run `…D9EZ9…` has 4. I withdraw the "no tool-using turn" gap from round 04. |
| No arguments, environment or prompt fields in any log | Confirmed by a script scan of all current files. |

## Findings

### 1. nitpick: freezing at the end request drops the last sampling interval of the pre-end cohort

- `End()` sets `endRequested` before the observer's final census, and `canDiscover` is false from then on. The census taken at end is therefore not used for discovery. A daemon child born between the last 500 ms tick and the end request, for example a tool process started in the final half second of a turn, never enters the cohort and is not reported even if it survives 60s.
- The plan allows this ("Sampling is not a complete OS process-creation audit"), and the `candidate_count` detail says sampling may miss ancestry. But the final census is the best evidence available, and the birth identity already recorded would let it be bounded by the end time.
- Impact: a possible false negative in exactly the window where a leaked tool process is most likely. It is worth one sentence in the PR body, not a blocker.

### 2. nitpick: the immediate shared sample is mostly native teardown noise, and the worker-local view disagrees with the parent

- Immediately after observation end, the parent logged six `candidate_survivor` events in the 04:49:57 turn. The worker-local observer logged one (PID 46291) at the same moment, because it sampled earlier in teardown and then exited. All six disappear by 10s.
- This is correctly labelled as uncertain, and warnings are raised only at the 10s and 60s samples. A reader of the raw log should still read 10s and 60s counts for trends.
- The worker-local Codex observer remains a duplicate and partial view. The proof discloses it, and I accept the stated reason for keeping it.

### 3. nitpick: standing items from earlier rounds

- `dropped_owner` is replayed per observer.
- Diagnostic plumbing still spreads across `Launch`, `commandexecutor.Request.DiagnosticRun` and `taskdaemon`.
- Agent turns have no run correlation (`run` is `unavailable`), which the plan permits.
- The `//nolint:errcheck` annotation and the De Morgan rewrites remain. The proof explains them as lint-driven and semantics-preserving, and I accept that.

### 4. proof gaps to state in the PR body

- Built-app runs show zero survivors at grace. The positive cases (a deliberate surviving child, PID reuse, a long service, a post-end late child that must not be attributed) are established by package tests, which I re-ran green.
- The flag-off built-app case rests on the recorded cache comparison. I did not re-run it.
- Linux enumeration is compile and vet checked only.
- Historical leak causation is not claimed, and the diagnostics make no such claim.

## Not found

- No defect in the corrected lifetime reporting. A shared owner never claims the daemon was reaped, and its end is explicit and PID-free.
- No attribution after a shared observation has ended. Known-owned descendants are still tracked after owner end, as the plan requires, and are kept distinct from shared candidates.
- Flag-off behavior, secrecy of log fields, bounded state and non-intervention (nothing is killed or retried, and results are unchanged) all hold. My own run ended `ok` with no recording error.

## Outcome

only nitpicks remain
