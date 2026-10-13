# Adversarial review: promises-ledger plan, round 06

> Recovered from the session log. The original `ephemeral/reviews/202610130930-promises-ledger-plan-round-06.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Issue — the final implementation direction still reinstates the rejected failed-save reload behavior

The transaction section now says that only a nil `WriteJSON` return
acknowledges a batch, that **any** save error leaves the old in-memory ledger
and cursor in place, and explicitly says “No reload, no post-hoc cursor
inspection” (`plan.md:241-249`). The decisions section nevertheless still
directs the implementer: “on any save error, reload from disk”
(`plan.md:470-474`). The `Store` declaration also claims its in-memory ledger
equals `ledger.json` after every save or reload (`plan.md:67-74`), even though a
post-rename error intentionally leaves those states different until a later
successful rewrite.

These are mutually exclusive algorithms at the exact boundary corrected after
round 05. `transcript.WriteJSON` returns errors both before rename and after
rename while opening or syncing the parent directory
(`internal/transcript/durable.go:95-125`). If an implementer follows the final
decisions summary and reloads/adopts a currently readable replacement after a
failed directory sync, the worker can advance its in-memory cursor and clear
the batch despite never receiving the writer's durability acknowledgment—the
same restart/reprocessing failure identified in round 05.

Impact: the plan still has two authoritative-looking answers for a core commit
failure, and one breaks the stated “restart unchanged, no recording
reprocessed” done criterion (`plan.md:38-47`). This needs no new machinery: the
decisions summary and `Store` invariant should agree with the already-specified
nil-only acknowledgment and old-memory-on-error behavior.

## Outcome

**material findings remain**
