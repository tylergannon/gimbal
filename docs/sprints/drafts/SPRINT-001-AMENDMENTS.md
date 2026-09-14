# Sprint 001 amendments under review

Proposed changes to `docs/sprints/SPRINT-001.md` (issue #157), not yet
applied. They fold in the resolutions for #155, #152, #151, #147, #13 and
the execution constraints Tyler set on 2026-09-13.

## Execution constraints (fixed)

- Subagents do the implementation; a separate validator subagent that never
  sees the implementer's notes checks each phase against the Definition of
  Done and reports pass/fail with evidence.
- Unit tests gate a phase; they are not the proof. Proof is one live run on
  the cheap tier plus a validator's independent arithmetic from `run.jsonl`
  plus screenshots.
- No proof infrastructure. `web/scripts/reconcile-usage.ts` is dropped and
  filed as a separate issue; the validator does the arithmetic by hand with
  throwaway commands. The shared fixture under `internal/observation/testdata/`
  is plain test data: Go and TS tests each replay the same copied logs and
  assert the same literal per-scope numbers. No cross-language harness.

## Per-issue resolutions to add

1. **#155 (reducer session info never filled).** The observation store
   builds each session's projection at `internal/observation/store.go:230`
   with `sessionstate.New(sessionstate.NewProjectionState())`. Seed it
   instead, at `SessionCreated`, with the session's info record: id, name as
   title, parent, model, adapter as agent. That is upstream's own path (the
   session record arrives before any step event), done from outside the
   port; the ported reducers are not edited. Then `session.usage.updated`
   lands in `info` as designed. Delete `RunInfo.Usage` and the store's fold
   of `session.usage.updated`; the session card reads reducer info. One
   source per level: session totals from reducer info, turn totals from the
   snapshot's `turns`, scope totals by summing turns. The browser needs no
   seed: it restores the projection from the snapshot it is sent.
2. **#152 (cost never on a message row).** Rule: a message row's cost cell
   shows what the harness stated for that call, which today is zero
   everywhere; cost is real at turn level and above (Claude's turn report).
   No proportional attribution.
3. **#151 (Codex cache write inside inputTokens).** Phase 1 line item:
   `codex/events.go` computes `input = max(0, inputTokens − cachedInputTokens
   − cacheWriteInputTokens)`, one unit test. "Not changed: adapters" narrows
   to "except this line". No live effect on today's models.
4. **#147 (task shown twice).** Phase 3 acceptance line: the tree renders a
   task scope's task once.
5. **#13 (usage and quota telemetry, pre-restart).** Close as superseded by
   #156; quota windows stay unbuilt.

## Named risk

Deleting the store's usage map touches the `scopeUsage` live query and the
session card that #156 validated. The proof run's validator must re-check
session totals against reducer info, not only scope sums.
