# Loop reviewers can never advance: the exit edge asks for a verdict the engine owns

URL: https://github.com/tylergannon/gimble/issues/80
State: closed
Milestone: None
Updated: 2026-09-08T05:44:37Z

## The defect

Every built-in loop workflow routes its reviewer back to the loop on a condition the reviewer cannot honestly satisfy. From `sprint-execute.yaml`, and the same shape in `chapter-loop.yaml` and `delivery-loop.yaml`:

```yaml
edges:
  - to: sprints
    condition: The sprint is demonstrated and nothing material is missing.
  - to: implement
    condition: Work remains, or the demonstration does not convince you.
```

Whether the sprint is demonstrated is a verdict **the engine owns**. The engine runs the item's `command`, consults `infer`, and writes `done`. The reviewer has no access to that outcome and cannot produce it. So an honest reviewer that cannot confirm a demonstration takes the only remaining edge — back to `implement` — and does so every lap.

That is the treadmill observed in the run recorded on #73: 64 `implement` turns, 63 `review` turns, exactly one arrival at the loop node, nothing changed.

## Why this is separate from #70 and #76

#73 added `max_visits` and said it bounds the damage rather than fixing the cause, pointing at #70. That is only half the cause.

- #70 / #76: the reviewer had no running application to look at. Fixing that lets the reviewer gather evidence.
- **This issue:** even with a running application, the exit condition asks for a conclusion the reviewer is not positioned to reach. A reviewer that has looked at the software and is merely unsure still has no edge to take but backwards.

Both halves have to be fixed or the loop still cannot advance.

## The fix

Condition the exit on what the reviewer can actually know, and route uncertainty toward the engine rather than back to the coder. Draft:

```yaml
edges:
  - to: sprints
    condition: The work is ready for the engine to check, or you cannot tell
      without running the item's own check.
  - to: implement
    condition: You found something specific and material that must be fixed
      before the engine spends a check on it.
```

The asymmetry is deliberate. Going back to the coder should require the reviewer to name a specific defect; anything less — including plain uncertainty — advances to the engine, which is the party that can settle it. A wasted engine check is cheap; an unbounded treadmill is not.

## Verification

Not landable on a passing test suite. The failure was only visible by running the workflow and reading what the reviewer wrote, and a lint rule cannot judge whether a condition is answerable by the agent being asked. This needs a live run of each built-in against a scratch repository, watched to completion, with the reviewer's transcript read.

## Related

- #73 — bounded the cycle, named this as unfixed
- #70, #76 — the other half of the cause

