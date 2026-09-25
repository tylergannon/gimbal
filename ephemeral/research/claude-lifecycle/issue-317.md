# Issue 317: [P1] Claude background-task wait ends evaluator early; same-session follow-up returns empty success

## Problem and impact

A real `validate-product` run advanced past its evaluator while the evaluator was waiting for a background shell command. Claude's tool explicitly promised a completion notification. Opus returned a waiting message expecting to continue; Gimbal's `Generate` returned that message as a successful completed turn. The immediately following same-session debrief returned an empty string with no error and no recorded assistant/tool activity. The workflow closed the browser, performed synthesis, and exited 0 while the delegated target workflow was still running.

This interrupted a real implementation-to-PR user journey: no evaluator acceptance exercise, no PR, and no UX/UI debrief. Treat this as a significant provider/session lifecycle issue to investigate, not just an inadequate prompt. The precise fault boundary (Claude CLI/SDK, adapter stream handling, or workflow expectations) is not yet established.

## Exact observed sequence

2026-09-20, times below in America/Costa_Rica (UTC-6):

1. **09:14:26** — Opus starts `gimbal run implement` through Claude's shell tool with `run_in_background: true` and explores the run's live web UI.
2. The shell tool returns a background task ID and output-file path, followed by:
   > You will be notified when it completes. To check interim output, use Read on that file path.
3. **09:19:23** — Opus starts another background command waiting for coding completion. Its tool response makes the same notification promise.
4. **09:20:00** — Opus says:
   > I'll pause polling and let the background waiter notify me when the coding turn ends.
5. **09:20:07** — First `Generate` finishes without error. Its ENTIRE returned text is:
   > Waiting for the coding turn to finish — I'll pick back up when the notification arrives.
6. The workflow immediately calls `Generate` on the same session for the UI/UX debrief.
7. **09:20:13** — Second `Generate` finishes without error, returning the empty string. The normalized transcript records inbox enqueue/delivery, execution start, a usage update, and execution success; no assistant text, tool activity, or model-step events are recorded for this turn. Determine whether this is a wrongly associated native completion event, lost turn, or another cause; none is confirmed yet.
8. The harness stops recording/closes the browser and starts screenshot review.
9. **09:20:17** — Target coding actually finishes. Checks pass and independent QA starts at **09:20:20**.
10. **09:22:48** — Evaluation harness exits 0. Its final synthesis correctly reports the assignment incomplete, but the saved task report contains only the waiting sentence and no debrief.
11. **09:27:50** — Target implementation independently completes successfully; its validator reports browser checks and 18 passing tests. This does not repair the missing evaluator journey.

No outer-agent steering, rescue, or restart occurred.

## Environment and evidence

- Gimbal build: `a41c8b7e6e2d4289dfcdf6c15c1fb99a75a22ee7`, unmodified installed build copied for the trial.
- Evaluator: `claude-opus-5:high`; all four target implementation roles: `claude-sonnet-5:high`.
- Screenshot reviewer: `gemini-3.8-flash-medium`; triage: `gpt-6-astra:high`.
- Harness run: `01M2ZP2XFPPJKZT5B09RPHB0F8.validate-product`.
- Target run: `01M2ZP40HF2KA2A89563C0KET0.implement`.
- Claude native evaluator session: `deee5fb6-61a3-4c41-b827-d5033fa663ae`.
- Task duration recorded by harness: 371.24 seconds. Target duration: 803.87 seconds.

Local-only evidence on the reporter's machine (not hosted attachments):

- Suite and assignment: `/private/tmp/gimbal-drag-drop-eval/suite.json`, `/private/tmp/gimbal-drag-drop-eval/assignment.md`.
- Harness store: `/private/tmp/gimbal-drag-drop-eval/observer/.gimbal/runs/01M2ZP2XFPPJKZT5B09RPHB0F8.validate-product/` — `turns.json` contains both return values; `sessions/user-testing.1/tester1.1/product-operation.1.jsonl` contains the tool notification promises and normalized events.
- Reports, 13 captioned screenshots and finalized video: `/private/tmp/gimbal-drag-drop-eval/results/user-testing-2103216338/`.
- Target store: `/private/tmp/scrabbler-drag-drop-eval/.gimbal/runs/01M2ZP40HF2KA2A89563C0KET0.implement/`.

## Investigation and expected behavior

Establish the native provider contract before prescribing a repair:

- Does Claude intentionally end a foreground turn while background tasks remain, and how are subsequent task notifications delivered and generation resumed in SDK/headless use?
- Does Gimbal consume, discard, or misassociate those notifications and result events? Explain the empty successful second `Generate`, including which native result belongs to which submitted prompt.
- Compare Codex using the same minimal workload. In the reporter's current Codex desktop tool surface, a yielded `exec_command` returns a `session_id`; the agent retrieves completion via `write_stdin`. A fresh two-second shell probe confirmed that shape and returned no prose promise of automatic notification. This does **not** establish every Codex adapter behavior or prove that Codex lacks asynchronous events. Its app-server documents output notifications: https://learn.chatgpt.com/docs/app-server#command-execution . Transport notifications and automatic model resumption are separate questions.
- Verify behavior for intentional long-lived service commands too; do not solve this by blindly waiting for every background process to exit.

Proof of the eventual fix should demonstrate a shell task outliving the initial yield, correct continuation through the intended task result, and a nonempty same-session follow-up response associated with the correct prompt. Test provider-specific semantics for Claude and Codex. The evaluation must not silently advance and tear down observation while the user assignment is still pending.

Explicit tool-based waiting within the evaluator's active turn is a possible workaround, not a tested repair or established root cause. No new orchestration framework is requested.

Historical related topic: #75 discussed waking idle host sessions, but concerns an older integration and does not establish the behavior of this current `Generate`/background-tool path.

