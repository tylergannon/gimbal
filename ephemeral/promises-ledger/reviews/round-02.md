> Recovered from the session log; the original `ephemeral/reviews/202610130730-promises-ledger-plan-round-02.md` was git-excluded and was deleted with the worktree.

# Adversarial review: promises-ledger design plan, round 02

## Review target

`ephemeral/promises-ledger/plan.md` at commit `9b208a4b`, re-reviewed in full
against its section 0 authoritative request, the repository instructions, and
the same Gimbal and Clarity sources used in round 01.

## Evidence inspected

- The complete current plan, its diff from `c5e365cc`, and
  `ephemeral/reviews/202610130700-promises-ledger-plan-round-01.md`.
- `AGENTS.md`, `skills/gimbal/SKILL.md`, `skills/gimbal-release/SKILL.md`,
  `docs/definition-of-done.md`, `docs/web-app.md`,
  `ephemeral/research/svelte-idioms/dos-and-donts.md`, `go doc -all .`, and the
  API design/sprint records named by `AGENTS.md`.
- Current Gimbal code cited by the plan, including `harness.go`,
  `internal/{binding,host,modelalias,transcript}`, `claude`, the conversation
  manager, workspace remotes/components, and the native recording call path.
- `/Users/tyler/src/promises-research/notes/{clarity-claims-ledger,gimbal-integration-points,clarity-export-and-live}.md`
  and the referenced Clarity snapshot at
  `/Users/tyler/src/promises-research/token-cache/clarity`, including the
  review validation/application code and conversational fixtures 03, 04, and
  07.

The revised plan does address round-01 findings 1–4: it specifies enforced
tool denial, copy/save/publish with disk reconciliation, transcript-grounded
evidence plus nonempty required fields, and a Claude-only lifecycle whose
`Close` has no native process left to reconcile. The findings below are from a
fresh review of the whole revised design.

## Findings

### 1. Critical — a successful batch can permanently acknowledge requirements it was incapable of returning or applying

The worker sends as many as 40 new chunks in one batch and advances the cursor
to the end of that entire range after applying the answer
(`plan.md:187-195`). The response schema, however, permits at most 24
operations (`plan.md:296-306`). There is no completion/coverage field and no
follow-up pass over the same range. A stopped recording containing 25 chunks,
each with one independent promise, is therefore a direct reproduction: a
schema-valid answer can represent at most 24 adds, yet the cursor advances past
all 25 and the omitted promise is never shown again.

The same false acknowledgement occurs for operations the model did return.
`apply` silently skips an add with a missing required field, an update to an
unknown id, or a withdraw of an unknown/already-withdrawn id, and the plan says
these skips are not errors (`plan.md:312-326`). The batch transaction still
advances `Cursors[id]` (`plan.md:191-195`). This is weaker than the cited
Clarity boundary, which rejects semantically invalid review operations before
application (`ResponsesClaimReviewProvider.swift:100-182`), and it contradicts
the core requested outcome that streamed requirements accumulate in the
ledger (`plan.md:5-20,35-47`). Lean validation does not require Clarity's full
machinery, but it cannot treat a known partial or invalid result as complete.

Impact: ordinary bounded input can be reported as successfully consumed while
one or more promises are permanently lost. The cursor may advance only through
input the response had capacity to account for, and an explicitly malformed
operation must not be silently converted into successful consumption.

### 2. Issue — the exact demo's withdrawal is unrepresentable when the promises and retraction settle in one batch

The done-criteria ask the user to speak three new promises and then “withdraw
the second one,” with entry #2 moving to Withdrawn (`plan.md:35-43`). A batch
request contains only promises that existed before the call, while every add
must use id `0` and every update/withdraw must target an exact supplied id;
inventing an id is forbidden (`plan.md:253-269,292-306`). Stable ids are not
allocated until `apply` processes the answer (`plan.md:318-325`).

Consequently, with an empty ledger and all four utterances in one settled
batch, the model cannot both add the three promises and withdraw the newly
allocated #2. If it omits the retracted promise, the third promise becomes #2
and there is no withdrawn entry; if it emits all three adds, no legal withdraw
operation can target the second add. The cursor then advances, so a later pass
cannot repair the result. Fixture 07 avoids this case by putting the two adds
and the withdrawal in separate turns, and the proposed live tests merely inline
that fixture (`plan.md:379-381`; Clarity fixture
`07-withdrawal-second.json:5-8`).

Impact: the primary real-microphone demonstration depends on pauses and worker
timing that the plan does not state or enforce. The operation language needs a
way to express same-batch retraction (or the worker must establish a batch
boundary before the retraction), and the live proof must exercise the combined
batch case.

### 3. Issue — the in-memory live status has no state owner, and success never clears `Working`

The concrete `Store` has only `mu`, `dir`, and `ledger`; `Ledger` has only
entries, cursors, and an updated timestamp (`plan.md:66-75`). Yet `Snapshot`
must produce a `View` containing `Working`, `Error`, and `Version`, all described
as in-memory state (`plan.md:77-83,112-116,197-198`). The plan never adds fields
that can hold those values or defines their initialization/version increments.

The success transaction also sets `Working=true` before the call, clears only
`Error` after it, and never sets `Working=false` on success
(`plan.md:191-195`). Only the failure path clears it (`plan.md:215-217`). The
Promises tab is explicitly supposed to render these values and suppress
unchanged live-query yields using `Version` (`plan.md:334-355`).

Impact: implementing the written data model literally is impossible; filling
in the obvious missing fields still leaves the page displaying “working”
forever after the first successful batch unless the implementer invents an
unstated transition. The plan needs one explicit owner and transition table for
the three live-only fields, including success, failure, save reconciliation,
UI withdrawal/restore, and initial subscription.

### 4. Issue — one recording-specific failure can starve every other requirements recording in the project

`retryAt` and the exponential backoff are defined once for the worker, not per
recording (`plan.md:174-175,215-217`). Each step scans the recording history in
its existing newest-first order, selects only one batch, and stops after that
call (`plan.md:176-189`; `internal/transcript/durable.go:239-271`). After a
newest marked recording repeatedly returns invalid JSON, times out, or has a
prompt too large for the model, every tick first waits on the project-wide
backoff and then selects that same still-unconsumed recording again. Older
marked recordings never reach the LLM.

The proposed failure test covers one recording failing and later succeeding,
but no test places an independently processable recording behind a persistently
failing one (`plan.md:366-378`).

Impact: a single bad requirements discussion can stop promises from
accumulating from all other recordings in that project indefinitely. Failure
state and scheduling need enough recording identity to let unrelated eligible
recordings continue, with a test that proves the second recording progresses
while the first remains in backoff.

## Outcome

**material findings remain**
