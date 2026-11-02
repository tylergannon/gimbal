# Native-control gate proof: adversarial review, round 02

Reviewer: Sonnet 5.5, read-only. Only this file was written. No native calls,
tasks, service or configuration changes, or experiments. Remote inspection was
read-only, with no raw environments, credentials or unrelated sessions. The
launch prompt narrows nothing and predicts no verdict. Scope was derived from
the current spec, plan, approval brief, recipe, repository instructions and the
evidence index.

## Review target

The complete current gate work: `ephemeral/native-control/{spec,approval,plan,operating-recipe,proof-adjudication}.md`,
the worklog, the round-01 review, the updated evidence index, and the original
runtime and media evidence. Several of these are uncommitted working-tree
changes (see nitpick 3).

## Evidence inspected (this round)

- **Round-01 items re-verified.** I did not rely on round 01's conclusions.
  - The original JPEGs re-hash to the indexed SHA-256 values (`da31aecc…`, `7b89034f…`). In round 01 I confirmed they equal the base64 payloads in the child rollouts.
  - Both images show Finder with the Go to Folder sheet and the exact marker.
  - The native tool sequences, the worker ledger (one owner, five `cancelled` requests), the installed bytes equal to the repo with scoped `env`, and the loginwindow ALLOW/unlock/relock windows all match the earlier review.
- **Current doc diff** (`git diff`, uncommitted)
  - `spec.md` gets a dated HITL amendment and the DoD now says "dismiss the sheet with Escape".
  - `approval.md` and `operating-recipe.md` are updated to match.
  - The worklog adds two entries.
  - `proof-adjudication.md` is new.
- **New retained records** (`~/.codex/outputs/native-control-feasibility/2026-11-01/`), each carrying parent rollout source path, source line and timestamp.
  - `original-lock-tool-records.jsonl`: I parsed all seven records. The lock readings are:
    - 02:45:58 (before admission): CG session locked true, IOConsoleLocked true, one online display.
    - 02:55:07 (before the first attempt): locked true, display 2 inactive and asleep.
    - 03:05:18 (before the retry): same.
    - 03:08:52 (after the retry): CG locked true.
    - 03:12:14 (before trial 2): locked true, same display state.
    - 03:14:25 (after trial 2): locked true.
    - A 03:01:54 reading is inspector-chat context, not a trial reading.
  - `original-model-observation-records.jsonl`: record at 02:43:34, source line 889.
  - `modelalias-original-preparation-extract.txt`: labelled by its author as a transcription.

## Disposition of round-01 findings

1. **Literal Cancel click (was issue).** Resolved. The spec and approval brief record a dated amendment, and the recipe states the as-run variation honestly. The amendment's authority rests on the caller's restatement that Tyler accepted it in the requesting chat. I cannot see that conversation, and I treat conversation-only requirements restated by the caller as authoritative. The as-run inputs keep their earlier wording, which is the correct handling.
2. **Lock evidence (was issue).** Substantially resolved. Original tool records now exist, with source lines, for pre and post CG lock readings around both successful trials. Per the spec, "unknown cannot pass" is satisfied. Residual gaps are minor and listed below. The ALLOW/unlock/relock correlation is unchanged and still temporal, not task-tagged. The index and adjudication say so.
3. **Model resolution and comparison (was issue).** Partly resolved. See nitpick 1.
4. **LLM conversation agent as the caller (was nitpick).** The adjudication states plainly that the gate used the hosted conversation workflow and did not establish the maintained caller or C1–C4. That is sufficient. The recipe says the same.
5. **No manual unlock or config change, asserted only (was nitpick).** Unchanged and still a reported assertion. The index says it is not an exhaustive audit. I found nothing contradicting it.

## Findings

No critical findings and no issues remain. I found no defect in the proof, the implementation (unchanged worker plus scoped configuration), or the documentation that should block the gate's stated conclusion. The conclusion is that the Gimbal-initiated, Desktop-owned worker route produced native Finder control under Locked Use in two fresh child tasks. It does not cover unattended or maintained-caller operation. Nitpicks:

1. **Model-selection provenance is weak, though the outcome is confirmed (nitpick).**
   - The resolver extract is a self-described transcription with no verifiable timestamp or tool-record URI. The adjudication admits this, and it is not proof of chronology.
   - The "Desktop-advertised model choices" record (`original-model-observation-records.jsonl`, line 889) is the controlling client's inspector chat reporting its own `create_thread` tool definition. It was captured at 02:43:34, before the worker was bound. That record is a secondhand agent summary ("The create_thread model choices do include gpt-6-luna…"), not a query of the worker's genuine context.
   - `start.json` carries `"gpt-6-luna"`, which was typed in by hand rather than produced by a Resolve call in the caller.
   - Real evidence that the model was accepted is the child and Gimbal model metadata. That shows `gpt-6-luna` worked, not that the spec's resolve-then-compare steps ran in order. The adjudication correctly frames the requirement as selecting a supported model, so this is a provenance nit.
2. **Lock readings are not immediately adjacent to each trial, and the admission re-check is thin (nitpick).**
   - The pre-reads precede dispatch by about 53 s (retry, 03:05:18 versus child start 03:06:11) and 30 s (trial 2, 03:12:14 versus 03:12:44). The post-reads follow the relock logs by about 2 min (03:08:52 versus 03:06:44) and about 1 min (03:14:25 versus 03:13:17).
   - The only admission-phase reading is 02:45:58, before the admission children at 02:47 and 02:49. Nothing shows the lock state at the moment of those admission start/read/cancel cycles. The spec asks for admission "repeated while locked".
   - The display-wrangler power property was unavailable, which the index records honestly.
   - Given the ALLOW/relock bracket, these gaps do not undermine the claim.
3. **Doc hygiene (nitpick)**
   - The amendment and proof-adjudication changes are uncommitted. Repository policy says to commit and push when something interesting happens.
   - `plan.md:4` still says "Remote execution awaits A's host window".
   - `spec.md:266` still says saved Finder approval "is not yet confirmed" although the gate confirmed it. The recipe, not the spec, now holds the confirmed state.

## Outcome

only nitpicks remain
