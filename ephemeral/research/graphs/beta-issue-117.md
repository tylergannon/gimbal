# #117: Neither harness adapter emits delta, harness-error, approval-request, or nested-transcript events

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/117. Read beta-implementation-handoff.md and beta-bug-triage.md for final session decisions and coordination notes.

Sprint 2's event taxonomy (`SPRINTS.md`, `API.md` Observability) lists, on the agent side: user, assistant, thinking, tool call, tool result, a delta naming what it extends, usage, a harness error or retry, an approval request, and a nested transcript for a subagent inside a tool call.

`claude/events.go` and `codex/events.go` only ever emit `user`, `assistant`, `thinking`, `tool_call`, `tool_result`, and `usage`. There is no code path in either adapter that produces a `delta`, a harness-error/retry event, an `approval_request`, or a `nested_transcript` event, even though the `Event` struct has fields for them (`Delta`, `CallID`, etc.) and the persistence layer is generic over `Kind`.

This doesn't block Sprint 2 (the log format itself is proven against a fake adapter that can emit any kind), but the real harnesses can't yet surface these situations to the log or the future graph page. Wire them up from each SDK's native streaming/error/approval callbacks.
