## Problem

A live `gimble run-prompt` review using Claude Fable 5.1 successfully wrote and checked its complete review artifact, then terminated with only `claude: assistant error: invalid_request`. The command returned exit 1 and empty stdout after 10m51s. The artifact survived, but the provider error lacks an actionable explanation and normal final-result delivery did not complete.

This is one observed failure, not yet a deterministic minimal reproduction. Root cause is unconfirmed: the saved evidence does not establish whether the invalid request originated in Gimble, the SDK, Claude Code, or the upstream provider.

## Environment and invocation

- Gimble branch `codex/issue-201-graph-plan`; failure evidence committed at `e1edfab`.
- Go `go1.27.1 darwin/arm64`.
- Claude Code `2.1.270` (reported by the installed CLI immediately after the incident).
- Requested and recorded model: `claude-fable-5-1`.
- Build: `go build -o bin/gimble ./cmd` (exit 0).
- Invocation: `bin/gimble run-prompt --model claude-fable-5-1 --workdir <worktree> --logs <new-empty-project-dir> <prompt>`.
- Exact prompt, paths, run/session IDs, exit status, and artifact hash: [invocation record](https://github.com/tylergannon/gimble/blob/e1edfab/ephemeral/issue-201/fable-review-invocation.json), [prompt](https://github.com/tylergannon/gimble/blob/e1edfab/ephemeral/issue-201/fable-review-prompt.txt).

The prompt requests a read-only review of local handoff/issue files, with one review artifact as the only write. To repeat, use a fresh log directory and a fresh review output path; adjust the absolute paths to the checkout containing the committed inputs. Do not overwrite the captured run or original review.

## Observed sequence

Run `01M2HGR9MV1E7YHGX9E0X7NGNQ.run-prompt`; native Claude session `a4c7091c-e94c-437f-9740-e598de944ce0`.

1. The model read the handoff, cached issue/comments, later user decisions, surrounding source, and available proof.
2. Native tool success at normalized session event 1315 confirms the report was written and checked: `8670 ephemeral/reviews/20260914211117-issue-201-fable-5-1-round-01.md`.
3. Session event 1318 records `session.execution.failed` with `claude: assistant error: invalid_request`.
4. `turn_ended` records the same error and an empty result; `run_ended` is failed. The run's final `complete` record has an empty recording error, so durable recording itself completed.
5. CLI stdout is empty. Stderr reports the run failure but does not print the usual final `Logs:` line.

[Complete written review](https://github.com/tylergannon/gimble/blob/e1edfab/ephemeral/reviews/20260914211117-issue-201-fable-5-1-round-01.md), [stderr](https://github.com/tylergannon/gimble/blob/e1edfab/ephemeral/issue-201/fable-review-stderr.txt), [full saved run](https://github.com/tylergannon/gimble/tree/e1edfab/ephemeral/issue-201/review-runs/20260914211117), [result summary](https://github.com/tylergannon/gimble/blob/e1edfab/ephemeral/issue-201/fable-review-result.md).

## Investigation pointers and expected behavior

`claude/claude.go:278` returns immediately on `assistantError`; `assistantError` at lines 308–319 reduces the assistant error to its enum code. `cmd/run_prompt.go` returns the run error before printing its final `Logs:` location. These are relevant code paths, not an established cause of the provider rejection.

Determine why the final request was rejected and correct any Gimble/SDK integration defect demonstrated by reproduction. Preserve useful provider diagnostic detail and make the run log location available on failures. A valid completed turn should deliver its final response; a real provider failure must remain a failure, with usable diagnostics. The existence of a written artifact alone must not be used to turn an unsuccessful turn into a successful CLI exit.
