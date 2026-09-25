# Sprint 002: Claude Generate Waits Through Background Completion (Issue 317)

## Pyramid Index

- **L0**: Resolve premature `Generate` completion during background bash execution; keep Claude's native process alive through intermediate waiting turns until explicit completion, followed by same-session resumption with an incompatible schema.
- **L1**:
  - *Process Lifetime*: Scope native process lifetime to the logical `Generate` turn across intermediate waiting results and automatic wakeups; terminate cleanly upon completion.
  - *Completion Contract*: Distinguish non-terminal waiting from terminal completion using an explicit typed completion contract, as native fields (`subtype: success`, `terminal_reason: completed`) appear on both.
  - *Schema-Changing Resumption*: Reconnect via `--resume <sessionID>` on subsequent `Generate` calls, asserting context preservation and distinct output schemas without process leaks.
- **L2**: Implementation in [`claude.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/claude/claude.go) and [`session.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/session.go); temporary acceptance workflow in `/private/tmp/gimbal-317-acceptance/`; research in [`issue-317.md`](file:///Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/issue-317.md), [`generate-lifetime.md`](file:///Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/generate-lifetime.md), and [`sol-semantics.md`](file:///Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/sol-semantics.md).

---

## 1. Outcome & Scope

Fix [Issue 317](file:///Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/issue-317.md): Gimbal's Claude adapter terminates turns prematurely when Claude yields an intermediate waiting response while background commands execute. Closing the process tears down in-flight background tasks; subsequent turns resuming the session encounter empty orphan results or dead context.

The repair ensures:
1. `Generate` blocks through background task execution. Intermediate `waiting` results do not return to the caller or kill the client stream.
2. Native task completion notifications trigger Claude's automatic wakeup generation on the existing stream without host reprompts.
3. Once the task genuinely finishes and Claude signals terminal completion, `Generate` returns the validated typed output and cleanly exits the native process.
4. Subsequent calls on the same `*Session` resume the conversation (`--resume`) with a different structured output schema, retaining previous context.

---

## 2. Implementation Boundaries

### Within Scope

- **Adapter Wait Loop ([`claude.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/claude/claude.go))**:
  - Refactor `waitTurn` to loop across incoming `claudeagent.Message` items until encountering a terminal completion message, an unrecoverable failure, or context cancellation.
  - When a `ResultMessage` arrives with a status indicating `waiting` (non-terminal state), record step usage and intermediate events via `projector`, but keep `stream` and `client` open.
  - Ingest `task_updated` and `task_notification` events. Consume the subsequent generation resulting from native wakeup (`origin.kind == "task-notification"`).
  - Return only when a `ResultMessage` satisfies the terminal completion contract.
- **Explicit Terminal Completion Contract**:
  - Native provider fields do not differentiate intermediate waiting from final completion (`subtype: success` and `terminal_reason: "completed"` appear on both).
  - Implement a concrete contract: structured generation prompts define or wrap the schema with execution state (`state: "waiting" | "completed"`). Intermediate yields with `state == "waiting"` keep the stream listener active; only `state == "completed"` satisfies `waitTurn`.
- **Orderly Teardown & Resumption ([`claude.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/claude/claude.go), [`session.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/session.go))**:
  - Once terminal completion occurs, `RunTurn` cleanly shuts down stdin and closes `stream`/`client`.
  - Ensure subsequent turns on the same `*Session` pass `--resume <sessionID>` and pass the new turn's `--json-schema` cleanly.
- **Unit & Adapter Regression Tests**:
  - Mock and stream-level unit tests in [`claude_test.go`](file:///Users/tyler/.codex/worktrees/286c/gimbal/claude/claude_test.go) asserting waiting loops, notification arrival, error aborts, and clean exits.

### Out of Scope

- No public API changes to `gimbal.Session` or `gimbal.Generate`.
- No session-wide background daemon, persistent server process, or background service survival across sessions.
- No general framework for arbitrary multi-agent coordination or custom polling loops.
- No modifications to other provider adapters (e.g. Codex).

---

## 3. Temporary Acceptance Workflow

Per [`SPRINT-002-INTENT.md`](file:///Users/tyler/.codex/worktrees/286c/gimbal/docs/sprints/drafts/SPRINT-002-INTENT.md), an isolated acceptance script located at `/private/tmp/gimbal-317-acceptance/main.go` verifies the fix against live Claude Haiku (`claude-haiku-4-5-20251001`):

1. **Setup**: Run via real `gimbal.Run`. Create one `*Session` using `NewSession`.
2. **Turn 1 (Background Wait & Wakeup)**:
   - Schema $T_1$: `{ "status": "waiting" | "completed", "computed_token": string }`.
   - Prompt provides canary $K$ (`CANARY-WAIT-9021`) and commands Haiku to launch a background bash job (`run_in_background: true`) sleeping 6 seconds before writing a computed hash $H$ to a temporary file.
   - Haiku outputs an intermediate `waiting` status.
   - *Verification Assertions*:
     - `Generate[T1]` does **not** return on intermediate wait.
     - Event stream receives `task_notification` when the bash command finishes.
     - Claude wakes automatically and emits a terminal result with `status == "completed"` and `computed_token == H`.
     - Return time strictly succeeds task completion timestamp (verifying genuine wait, not premature exit).
3. **Turn 2 (Schema Mutation & Context Recall)**:
   - Schema $T_2$: `{ "recalled_token": string, "step_count": int }` (incompatible with $T_1$).
   - Prompt asks to recall canary $K$ without repeating $K$ or reading disk.
   - *Verification Assertions*:
     - Same session ID preserved across turns.
     - `Generate[T2]` succeeds without orphan result errors and accurately extracts $K$.
4. **Execution Safeguards**: Bounded timeout (90s); clean process termination; non-zero exit on assertion failure.

---

## 4. Definition of Done

Following [`definition-of-done.md`](file:///Users/tyler/.codex/worktrees/286c/gimbal/docs/definition-of-done.md):

| Requirement | Verification Evidence |
| :--- | :--- |
| **Genuinely Exercised Wait** | Acceptance logs prove intermediate waiting result received, listener remained active, and return occurred only after task completion event. |
| **Native Task Wakeup** | Native notification awoke Claude without host reprompting, external polling, or `TaskOutput` blocking. |
| **Output Integrity** | Turn 1 typed value contains background-computed hash $H$. |
| **Context Retention Across Schemas** | Turn 2 succeeds with incompatible type $T_2$, recalling canary $K$ on the same conversation ID. |
| **Bounded Failure Handling** | Background failure, cancellation, and timeouts abort cleanly with non-zero exit; no orphan processes. |
| **Zero Regression** | Existing tests pass (`go test ./...`); current codebase fails the acceptance script prior to the fix. |

---

## 5. Failure Modes & Edge Cases

1. **Background Task Failure**: If background bash exits non-zero, the adapter must not synthesize success. The failure notification must propagate to Claude; if unhandled, the turn must fail.
2. **Context Cancellation / Timeout**: If `ctx.Done()` fires while waiting, `waitTurn` triggers `stream.InterruptWithReceipt`, cancels the process context, and returns `ctx.Err()`.
3. **Premature Stream Termination**: If `stream.Messages()` closes before terminal completion, `waitTurn` must return an error (`"stream closed without terminal result"`), never defaulting to the intermediate waiting result.
4. **Orphan Notification Race on Resume**: If an old task completes during new process startup, the adapter must ensure that notifications are matched to active turns, avoiding empty orphan success values.

---

## 6. Unresolved Decisions & Assumptions

- **Terminal Contract Implementation**:
  - *Option A (Structured Schema Envelope)*: Adapter wraps user schema $T$ in `{ state: "waiting" | "completed", data: T }`. Reliable across models, but alters raw schema.
  - *Option B (Contract Prompting)*: Prompt mandates `state: completed` in user-defined schema $T$. Retains exact $T$, but relies on prompt compliance.
  - *Recommendation*: Validate Option B in the live acceptance trial; fall back to Option A if Haiku omits state.
- **Intentional Long-Lived Services**:
  - Because process teardown occurs upon logical turn completion, persistent background services (e.g. web servers) started in Turn 1 may be killed. It is assumed that multi-turn background services require a distinct lifecycle policy outside Sprint 002.
