# Generate-scoped process lifetime and schema changes on resume

## Finding and revised recommendation

A fresh Claude process can resume the same conversation with a different native
output schema, or with no schema. The schema is not an immutable attribute of
the persisted conversation in the tested path. Earlier experiments established
only that reinitializing an already-running process did not replace its schema.

Prefer investigating the user's smaller lifetime: keep one process and reader
alive for the entire logical Generate call, including intermediate waiting
results and automatic continuations, then close it after the assignment's
terminal outcome. The next Generate resumes the same session ID in a fresh
process with its own schema. This preserves shared conversation context between
differently typed call sites and avoids keeping processes alive merely because
the Session object still exists.

This revises the earlier recommendation to own the process for the entire
Session lifetime. Gimbal already starts/resumes and tears down a process per
RunTurn; the principal missing behavior is keeping that invocation pending
through a waiting result and correctly attributing the eventual terminal result.
Generate's validation/retry handling may invoke multiple adapter turns; the
precise ownership boundary still needs implementation review.

## Live test

Observed 2026-09-20 with Claude Code 2.1.270 and Claude Haiku
(claude-haiku-4-5-20251001). One newly created conversation was used throughout.
Each invocation had a different process ID. Native schema configuration used
--json-schema at process launch; continuation used --resume with the same ID.

| Process | Native schema | Observed output and shutdown |
| --- | --- | --- |
| A | phase: waiting/completed, a: string | Launched a finite background shell. First returned waiting/PENDING, then the same stream automatically produced completed/FINITE_DONE with task-notification origin. Closed stdin only after completion; process exited 0. |
| B | b: string | Resumed the same conversation and returned the remembered context token under b. Sent SIGTERM after its result; process exited 143. |
| C | c: integer, remembered: string | Resumed after B's SIGTERM and returned c=17 plus the original context token. Closed stdin; process exited 0. |
| D | No schema | Resumed and returned only the original context token as plain text, with no structured_output. Closed stdin; process exited 0. |

The later prompts did not repeat the token. Exact output values, result session
IDs, shapes, and first waiting/final completion ordering were asserted by the
probe. All assertions passed. No forced SIGKILL was needed. The resumed calls
produced their expected answer without an intervening empty orphan result in
these captures, after A's finite work had completed.

Probe program and raw captures are outside the repository at
/private/tmp/gimbal-317-resume-schema/. This is direct native-CLI evidence, not
an implemented Gimbal fix or a claim about arbitrary crash durability.

## What this solves and what it does not

It solves the schema-lifetime objection to reusing the same conversation across
call sites. A call can use its own fixed native schema throughout waiting and
completion; the next call can use a different schema in a new process. An
optional waiting/completed/failed envelope can embed the caller's exact final
schema rather than switching to an unrestricted payload. That completion
contract remains a proposed design requiring a live implementation test.

It does not identify authoritative task completion automatically. Native result
success and schema validity still allow waiting. The reader must keep consuming
until an explicit associated terminal declaration, cancellation, or failure.
The declared completed value must describe the whole assignment, not just the
last task notification. Known workflow checks can verify model claims.

It also does not preserve intentionally live native services across Generate
calls. Process shutdown after an assignment returns may stop such work; neither
--resume nor this test guarantees otherwise. If workflows need a service to
outlive a call, its lifetime needs separate ownership or an explicit exception.
For finite background work that finishes within the logical Generate, the new
probe demonstrates the proposed boundary without that issue.

Prefer orderly close after terminal completion, with a termination fallback.
SIGTERM-after-result worked here, but killing during work or before persistence
is a different case. Official [session documentation](https://code.claude.com/docs/en/agent-sdk/sessions)
explains conversation resumption after process restart; schema replacement and
clearing on that path are established here by the live test.
