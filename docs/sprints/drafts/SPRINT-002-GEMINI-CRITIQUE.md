# Sprint 002 Critique: Claude Generate Waiting Lifecycle

Critique of [SPRINT-002-CLAUDE-DRAFT.md](file:///Users/tyler/.codex/worktrees/286c/gimble/docs/sprints/drafts/SPRINT-002-CLAUDE-DRAFT.md) and [SPRINT-002-CODEX-DRAFT.md](file:///Users/tyler/.codex/worktrees/286c/gimble/docs/sprints/drafts/SPRINT-002-CODEX-DRAFT.md) against [SPRINT-002-INTENT.md](file:///Users/tyler/.codex/worktrees/286c/gimble/docs/sprints/drafts/SPRINT-002-INTENT.md) and the temporary fixture in `/private/tmp/gimble-317-sprint/` ([`main.go`](file:///private/tmp/gimble-317-sprint/main.go), [`run.py`](file:///private/tmp/gimble-317-sprint/run.py), [`claude-tap.py`](file:///private/tmp/gimble-317-sprint/claude-tap.py)).

The baseline reproduction confirmed the defect: `worker.Generate[Completion]` returned early on the first native result (`phase: "waiting"` at ~10s) before the 20s `slow.py` finished. Process termination followed immediately, preventing task completion and aborting before the second call.

---

### 1. Witnessing Waiting and Intermediate Payloads

- **Fixture divergence (Claude):** Claude’s draft invents an alternate fixture (`JobReport`/`TOKEN_A`/`TOKEN_B`, 45s sleep) asserting against "the Session's own event stream." In Gimble, `Session.Generate` is synchronous and blocks; `Session` has no public event subscription API. The actual fixture captures events out-of-band via `claude-tap.py` and asserts `waiting['received'] < completion['completed'] <= returned['time']`.
- **Missing waiting payload (Codex & Claude):** `run.py` requires `'WAITING_317' in json.dumps(first[res[0]])`. Codex specifies that waiting carries no value; with `additionalProperties: false`, Claude cannot output `WAITING_317` without violating the schema. Claude leaves `value` optional but typed as `T`. If omitted, `WAITING_317` is absent and the assertion fails; if included, it must conform to `T`'s schema (problematic when `T` requires other fields). The envelope must specify an explicit intermediate payload field (e.g. `"message": {"type": "string"}`) on `state: "waiting"`.

### 2. Schema References (`$defs` / `$ref`)

- **Root-relative pointer breakage (Claude):** Claude embeds `<T's schema, verbatim>` under `properties.value` and assumes nested `$defs` validate. Standard JSON Schema resolves relative JSON Pointers (e.g., `"#/$defs/MyType"`) from document root. Nesting moves definitions to `#/properties/value/$defs/MyType`, immediately breaking root-relative `$ref`s for complex Go structs.
- **Unspecified hoisting (Codex):** Codex acknowledges reference preservation but provides no mechanism. The adapter must hoist `$defs`/`definitions` to the top-level envelope schema and adjust or prefix subschema identifiers during envelope generation.

### 3. Stale Notification Attribution

- **Resume cross-call leakage (Claude):** Claude relies solely on one prompt per process and discards `origin` and invocation tokens, assuming Rule 2 (ignoring results lacking `structured_output`) prevents leaks. If a resumed process receives a delayed task notification from a prior turn that emits structured output under the new schema, Claude's adapter will misattribute it as the response to the new prompt.
- **Intra-turn task misattribution (Codex):** Codex’s invocation token prevents cross-turn leakage across resumes. However, within a single turn launching multiple commands, an intermediate notification from an unrelated background task carries the valid turn token. Without verifying `origin` or correlating `task_id`, the adapter cannot determine whether a wakeup stems from the required dependency or a secondary task.

### 4. Failure Semantics and Scope

- **Omission of `failed` state (Claude):** Claude rejects a `failed` state, asserting `T` carries task failure. When `T` has strict required fields (e.g. `Recall`), Claude cannot report command failures without schema violations, risking hallucinated data or deadlock (hanging in `state: "waiting"` until context timeout). Codex's explicit `failed` state with a reason correctly terminates the turn without corrupting `T`.
- **Validation retry process boundary:** If `state: "completed"` returns a malformed `value`, `session.go` re-asks via `s.turn`, launching a new process with `--resume`. Because the original process and background command have already exited, the resumed process cannot re-observe the command. Neither draft specifies how the re-ask prompt injects prior execution context so Claude can re-encode `value` without trying to re-execute completed tasks.
