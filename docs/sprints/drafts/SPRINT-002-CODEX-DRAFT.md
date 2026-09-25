# Sprint 002: Claude Generate waits for the assignment

## Outcome and boundaries

Repair #317: one Claude `Generate` remains pending through an explicit waiting
answer and native background completion, returns the actual final typed result,
then permits a differently typed call on the same conversation. Use one native
process and reader through the logical call; close after terminal completion.
The next call starts a fresh process with its own schema and resumes that ID.

This is an implementation plan, not implementation or completed proof. Sprint
001 remains unchanged; no chapter is selected. The controlling sources are
[intent](SPRINT-002-INTENT.md),
[lifetime research](../../../ephemeral/research/claude-lifecycle/generate-lifetime.md),
[native semantics](../../../ephemeral/research/claude-lifecycle/sol-semantics.md),
[issue](../../../ephemeral/research/claude-lifecycle/issue-317.md), and
[definition of done](../../definition-of-done.md). The newer lifetime finding
supersedes the older session-wide-process recommendation.

Change Claude's adapter and focused tests; touch `session.go` only where logical
call ownership requires it. No public API names, session-wide persistence,
server, disabled background tasks, unrestricted payload, service-survival
policy, dashboard, retention feature, or unrelated provider changes.

## Proposed completion and attribution contract

`waitTurn` currently returns the first successful result; `RunTurn` then closes
its stream/client. Native success, `terminal_reason`, schema validity, task
counts, notification origin, and increasing result indexes cannot establish
assignment completion.

Propose a private Claude protocol envelope with an invocation token and three
states: `waiting`, `completed`, `failed`. The adapter supplies a fresh token in
its instruction and constrains it in the native schema. The completed branch
contains `value` under the caller's exact schema; waiting carries no final
value, and failed carries a reason. Preserve schema references correctly when
embedding. Native validation of T remains mandatory; the runtime still validates
and decodes the unwrapped value. This envelope is internal, not a workflow API.

The instruction defines completion as fulfillment of the entire assignment,
including required finite dependencies. Waiting means keep consuming the same
stream without sending another prompt. A matching completed declaration permits
return; matching failed, provider failure, cancellation, or stream exhaustion
without completion returns an error. Cancellation takes precedence over a
concurrent result. Shutdown is orderly with a bounded termination fallback.

Attribution uses the active invocation token plus the native conversation ID.
Origin, indexes, and task/tool IDs support diagnosis, not semantic authority.
An old-token result or empty orphan notification cannot satisfy the new call;
continue listening for its associated declaration. A matching malformed terminal
payload cannot become success. Missing observation events must not abort valid
execution merely because an assumed event ordering was violated.

The token establishes which assignment Claude is answering, not whether its
claim is true. The acceptance workflow independently checks task output and
ordering. This contract does not promise exactly-once recovery after crashes.

## Temporary acceptance workflow

The root agent owns the actual fixture and baseline. Keep its source, generated
types, project records, and raw captures under a fresh `/private/tmp` directory.
Use this checkout through the temporary module, real `gimbal.Run`, a Haiku
`ModelBinding` using `claude.New()`, exactly one `NewSession`, and sequential
`session.Generate[First]` and `session.Generate[Second]` calls. Types implement
the current `Output` contract and have incompatible required fields.

The first constant prompt asks Claude to launch a finite, sufficiently long
native Bash command with `run_in_background: true`, explicitly answer waiting,
then await its native notification. The command produces a fresh unpredictable
output only at completion. Its actual output and exit status are checked against
the returned typed value. No host reprompt, polling, blocking `TaskOutput`, or
direct-CLI replacement of Generate is permitted.

Give the first prompt a separate recall token via scope context. Exclude it
from the second call's rendered context with `WithScopeTemplate`; do not repeat
it in that prompt or an agent-visible file. Keep fixture source and verification
records outside the agent workdir, and inspect tool activity for reads that
could invalidate recall. The second type requires the recalled token and a
different result shape. Verify the same native conversation ID and fresh process
with schema B.

The validator must observe background launch, an actual intermediate waiting
result while Generate remains pending, native task completion and wakeup,
terminal typed output, then first-call return and successful second-call return.
The output must match the completed command. Elapsed time alone proves none of
this. Missing waiting or substituted blocking execution fails the fixture.

Use an explicit overall deadline, propagate every error through Run to nonzero
process exit, and bound cleanup of the Claude process and fixture-owned child
processes. Run the unchanged fixture before repair: record the actual failing
assertion and exit status. A timeout or failure to elicit waiting is not evidence
that the intended baseline defect was reproduced.

## Definition of done and remaining decisions

After repair, the same acceptance passes with Haiku. An independent validator
inspects the fixture and observations, confirms acceptance was not weakened,
and reports what happened in chat or the PR. Commit plan notes and implementation
tests, never temporary proof programs or captures.

Focused adapter tests cover repeated waiting, notification races, stale/empty
results before a current answer, invalid terminal payload, required-task failure,
provider failure, cancellation while waiting, premature EOF, schema-changing
resume, bounded cleanup, and usage across multiple native results. Run the
repository's applicable checks; green tests supplement the live demonstration.

Resolve before implementation:

- `generate` currently retries invalid values through separate `RunTurn` calls.
  Specify private ownership so validation retries cannot silently violate the
  one-process-per-logical-call boundary; preserve bounded validation semantics.
- `Text` currently sends no schema. Choose and demonstrate its explicit terminal
  protocol without pretending ordinary prose reliably signals completion.
- Verify the native schema dialect accepts the envelope and preserves caller
  references. Existing CLI probes establish schema replacement on resume, not
  this proposed envelope's correctness.
- Integrate the root agent's baseline result and exact temporary workflow path;
  neither is established by this draft.
