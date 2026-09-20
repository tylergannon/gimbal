# Sprint 002: Claude Generate waits through native completion

**Status:** planned. **Issue:** 317. No chapter is selected.

## Pyramid Index

- **L0:** A Claude `Generate` waits through an explicit intermediate response
  and native background-task wakeup before returning a validated final value;
  a following call resumes the same Gimble Session under a different schema.
- **L1:** The native process/reader lives for one adapter attempt through its
  waiting continuations, not for the Session; a private completion envelope
  separates `waiting` and `completed`; schema composition and result
  attribution remain sound; focused regressions and one real Haiku run prove
  the repair.
- **L2:** Change only the private Claude adapter and focused tests. The
  temporary acceptance fixture stays at `/private/tmp/gimble-317-sprint/`.
  Evidence: [lifetime](../../ephemeral/research/claude-lifecycle/generate-lifetime.md),
  [native semantics](../../ephemeral/research/claude-lifecycle/sol-semantics.md).
  Rerun it with `python3 /private/tmp/gimble-317-sprint/run.py`.

## Desired behavior and boundary

`claude.waitTurn` currently returns the first successful native result, so
`RunTurn` closes the client even if it says it is waiting. The unchanged Haiku
fixture (Claude Code 2.1.270), run
`01M2ZX43FSX7CQSQ6E3ZA1HC9Z.claude-wait-schema`, demonstrated that defect:
after about 11 seconds the first call returned typed
`phase=waiting, output=WAITING_317`, before its 20-second command completed;
the workflow ended `EARLY_RETURN` and never reached call two. That is a red
baseline, not evidence of a fix.

Repair only this private harness behavior. One adapter attempt keeps its
native client, stream reader, and submitted prompt alive across intermediate
waiting results and automatic native continuations. On terminal completion it
closes orderly, with bounded termination fallback. The next `Generate` starts
a fresh native process with `--resume` and that call's schema. A validation
re-ask may create a fresh resumed adapter attempt: this plan does not promise
one process across all bounded `Generate` retries.

Do not add a public API, Session daemon, server, background-task disablement,
service-survival policy, generic framework, retention feature, or other-provider
changes. `session.go` changes only if private attempt ownership or validation
requires it; public `Generate`, `Text`, and caller types retain their contracts.

## Proposed private completion contract

Native `success`, `terminal_reason: completed`, valid structured output,
`origin`, task snapshots, and `result_index` each describe a generation or an
observation; none proves that the assignment is done. The adapter will use a
private native-schema envelope with distinct `waiting` and `completed` states.
`waiting` has a fixture-witness message and keeps the reader open without a
host reprompt. `completed` must contain a final payload; only its unwrapped
value reaches caller validation and decoding. Missing or malformed payload is
invalid, never an empty successful `Text` result, and uses bounded validation
retry. Cancellation, provider/result failure, or EOF while waiting are errors;
a command's nonzero exit is for Claude to report through the caller type unless
the workflow required that command's success.

The envelope is adapter-private, not a field on caller types. Its composer must
preserve caller-schema meaning: retain/hoist root `$defs` (and legacy
`definitions`) with collision handling, rewrite affected references, and reject
composition it cannot preserve. Naively nesting under `value` breaks
`#/$defs/...` and recursive references. Native validation still applies to the
final shape, proven by a focused referenced/recursive-schema regression. `Text`
needs equivalent waiting/completed behavior while returning the same public
prose result; do not accept an omitted payload.

An orphan native result seen on a resumed process before its newly submitted
prompt must not finish that prompt, even if it is structured. Native
`origin`, `result_index`, and conversation ID help diagnose ordering but do
not prove prompt receipt. A schema `const` token merely identifies the launched
process and is not evidence that Claude received the current prompt. The
adapter must choose and verify a bounded, private post-submit attribution rule
(including a stale structured completion regression) before relying on it; no
exactly-once or cross-crash guarantee is part of this sprint.

## Product completion evidence

Keep the temporary two-call workflow outside the checkout and run it with
Claude Haiku after the repair. It uses real `gimble.Run`, one `NewSession`, and
two sequential `Generate` calls. Call one launches exactly one finite native
background Bash command, yields the explicit waiting declaration, and neither
polls nor blocks on `TaskOutput`. The command mints its receipt only at
completion. The fixture must observe, in order: waiting while `Generate` is
still pending; a native completion notification; a final response originating
from that automatic wakeup; receipt equality in the typed final value; then
the first call's return. It captures submitted input too, proving no hidden
host reprompt caused continuation. Elapsed time alone is insufficient.

Call two uses an incompatible schema in a different PID and resumes the same
conversation ID. It recalls first-call context without repeating the token,
calling tools, or reading an agent-visible token. The fixture checks both schemas,
result conversation IDs, one user input per process, and return ordering.
Failure to elicit waiting, a timeout, or unrelated failure is not a pass or red
baseline reproduction. It has a bounded deadline, drains its observer, and
cleans its process group and owned child work on every path.

Focused adapter tests cover waiting-to-completed flow and usage/event handling;
malformed or absent final payload; `Text`; cancellation, provider failure, and
EOF while waiting; stale/orphan notification before a current answer; schema
references; schema-changing resume; and validation retries/continuations. They
must not grow into a blanket background-task registry or unrelated-provider
test suite.

## Definition of done and open decisions

The product sprint is complete only when focused checks pass and the changed
harness produces a green live Haiku run with these observations. An independent
validator reads fixture, raw capture, and source; verifies the red baseline was
not softened; and confirms both shapes, PIDs, session ID, wakeup, receipt, and
ordering. No green repair is claimed here.

Before coding, settle the exact reference-preserving envelope construction,
the private attribution rule for resumed orphan results, and the `Text`/
validation-retry interaction. These are bounded design questions; they do not
authorize a public contract or persistent-service design. Writing and merging
this planned document is a separate gate from implementing and proving the
repair.
