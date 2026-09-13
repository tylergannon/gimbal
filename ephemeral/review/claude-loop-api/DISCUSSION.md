# Loop review: Claude and Codex in conversation — 2026-09-12

Two independent reviews of the same target (merged PR 140, `2a52971`),
followed by rounds of exchange. Nobody edits the other's artifact; each round
is a new file. This log records what was said and what moved.

| Artifact | Author | Path |
| --- | --- | --- |
| Codex review | Codex (gpt-5.6 family, Codex Desktop session `01a09585-487d-7cf3-9983-88a1466c4b40`) | `/Users/tyler/.codex/worktrees/loop-api-review/gimble/ephemeral/review/loop-api/REVIEW.md` |
| Claude review | Claude (Fable 5.1) | `ephemeral/review/claude-loop-api/REVIEW.md` (this worktree) |
| Round files | each party | `ephemeral/review/claude-loop-api/rounds/NN-<party>.md` |
| Recommendations | joint, written by Claude, signed off by both | `ephemeral/review/claude-loop-api/RECOMMENDATIONS.md` |

Transport: `tractor run-prompt --session <id>` resumes Codex's own reviewer
session, so Codex argues with the context of its morning review intact.
Claude writes its side directly.

## Protocol

1. Claude sends Codex its review and asks for a point-by-point response:
   agree, disagree with evidence, or refine. Codex writes `rounds/01-codex.md`.
2. Claude answers in `rounds/02-claude.md`, conceding what is shown and
   holding what is not; Codex replies in `rounds/02-codex.md`.
3. One more round if anything material is still open; otherwise draft
   `RECOMMENDATIONS.md` and ask Codex to sign or dissent per item.
4. Disagreements that survive three exchanges are recorded as dissent for
   Tyler, not resolved by fiat.

## Log

### 11:23–11:32 Claude live run
Semverbump fixture, Luna planner/worker, Haiku reviewer in a Group. One task, 28/28 first try, planner ended dispatch, final gate passed. No adaptation exercised; reported as such.

### 11:45 Round 1 request → Codex
Sent to Codex session `01a09585…` via `tractor run-prompt --session`: read Claude's REVIEW.md and evidence; respond point by point to Claude's three fixes, to Claude's responses to Codex's three fixes, to anything overstated, and give a re-ranked top three. Output: `rounds/01-codex.md`. Prompt text is in `rounds/01-codex.log` header context (not repeated here).

### 11:43 Round 1 ← Codex (`rounds/01-codex.md`)
Agrees: single-channel planner protocol (refines: goal outside model output, `Next` indexes the returned snapshot, runtime still validates cross-field conditions); required idempotent `Close` (refines: logically closed first, cleanup ctx independent of cancellation, failure is a lifecycle failure not a recording error). Disagrees: panicking `Set` (NaN is a real marshal error; no panic boundary in the runtime; `Group.Go` has no recover; a local `mustSet` is ordinary Go). Withdraws `Get` as a top-three item but says Claude's own run shows selective-projection pressure (double `role`, 6.5→11.9 KB). Holds observation correctness as #3, split from UI richness. Corrects two Claude overstatements ("every input is a workflow line"; "identity, data, lifetime agree") and the classification of Close failures as recording errors.

### 11:50 Round 2 → Codex (`rounds/02-claude.md`)
Claude withdraws the panic recommendation; concedes both overstatements and the Close-failure domain. Holds: Close failure must not enter the *scope* error because scope errors drive control flow (Group cancel, Loop end, commit gate); proposes `SessionClosed{Error}` plus a distinct value joined into Run's result. Holds: the double-role pressure points at a runtime change (Loop's feedback projection skips shadowing keys), not a new name. Leaves invalid-plan handling open (end Loop vs one repair turn). Drafts eight joint recommendations and asks Codex to mark each agree/dissent.

### 12:00 Round 2 ← Codex (`rounds/02-codex.md`)
Agrees Close failures stay out of the scope error; refines to an aggregate typed cleanup error on Run and a truthful `RunEnded`. Dissents on skipping shadowing keys: key collision cannot distinguish a worker-only `role` from an intentional `status` revision; fold into the two-projection experiment. Picks "end Loop on invalid plan, no repair turn". Marks the eight items: agree 1, 2, 3, 5, 7; dissent 4 (remove), 6 (Loop itself adds planner text, so reword), 8 (Set ceremony is 9% and 22%, not 17–30%).

### 12:05 Resolution
Claude accepts all three dissents. `RECOMMENDATIONS.md` written with Codex's amendments: invalid plan ends Loop; aggregate typed cleanup error; item 4 removed and folded into deferred item 7; Godoc wording per Codex; corrected percentages. Sent to Codex for final sign-off (round 3, sign-off only).

### 12:10 Round 3 ← Codex (`rounds/03-codex.md`)
SIGNED. "This accurately states the shared verdict, resolved design decisions, priority order, and intentionally deferred questions." Consensus complete; nothing left for Tyler to adjudicate.
