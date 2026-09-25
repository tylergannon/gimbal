# Sprint 002 intent: Claude Generate waits through background completion

## Seed

Tyler requests a sprint plan and temporary definition-of-done workflow for bug
317. The workflow must synthesize Claude launching a long-running command and
yielding to wait. The first Generate must remain pending until the command
finishes and its native completion wakes Claude. Then a second query on the
same Session must use a different output format and succeed.

## Context and settled direction

Read these local sources:
- /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/generate-lifetime.md
- /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/sol-semantics.md
- /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/issue-317.md
- /Users/tyler/.codex/worktrees/286c/gimbal/docs/definition-of-done.md
- /Users/tyler/.codex/worktrees/286c/gimbal/claude/claude.go
- /Users/tyler/.codex/worktrees/286c/gimbal/session.go

A native success may mean waiting. Keep one native process alive through a
logical Generate's intermediate answers and automatic continuations; close it
after terminal completion. The next call resumes the same conversation in a
fresh process with that call's schema. Direct Haiku probes established schema
A -> EOF -> resume B -> SIGTERM -> resume C -> EOF -> resume unstructured text,
with the original conversation ID and earlier context preserved. Do not replace
this direction with session-wide persistence, a server, disabled background
tasks, or an unrestricted payload that drops native validation of T.

## Pyramid Index

- L0: Fix premature completion and prove it with two real sequential Generate calls.
- L1: Explicit waiting versus terminal semantics; one process per logical call;
  native background wakeup; schema-changing conversation reuse.
- L2: Settled direction above; acceptance below; research and code paths above.

## Prior art and sprint context

Only Sprint 001 exists (token usage by scope); it is unrelated and remains
unchanged. No chapter link selected. docs/SEMANTIC-INDEX.md identifies docs/ and
ephemeral/ as the local token cache, with no built index. Use the narrow local
research files above, not a broad scan. This task plans implementation, does
not implement or release the harness repair.

## Acceptance and constraints

- Use real gimbal.Run, one NewSession, and two sequential Generate calls with
  incompatible structured output types. Live model is Claude Haiku.
- First prompt causes native background Bash execution and an explicit waiting
  answer. Continue listening; native task completion wakes Claude without host
  reprompt, polling, TaskOutput blocking, or replacing Generate with a CLI call.
- Prove waiting was genuinely exercised, completion preceded return, and final
  typed value reflects actual task output. Time elapsed alone does not prove it.
- Second prompt uses a different result type, recalls a first-prompt token not
  repeated in its prompt or visible file, and preserves the conversation ID.
- Temporary workflow/probes/raw logs stay outside the repository in /private/tmp;
  commit plan notes only. User explicitly requested this temporary acceptance
  workflow, overriding the general prohibition on proof programs for this task.
- A bounded deadline, failure propagation, cleanup, and nonzero failure status
  are required. Fixture failure to elicit waiting is not a pass.
- Current behavior should fail this workflow. Do not fix it while planning.
- Native result attribution and explicit task terminal declaration need a
  concrete proposed contract. Do not mistake field parsing for semantic truth.
- Cancellation and provider failure must not turn an intermediate answer into
  success. Keep focused regression tests beside adapter code when implementing.
- No new public API names, general framework, service-survival policy, dashboard,
  raw-retention product feature, or unrelated provider changes.
- Spell out unresolved assumptions and what a validator must observe. A reviewer
  may not broaden scope or weaken this acceptance workflow to pass.

## Planning outputs

Draft a concise plan (roughly 1000 words maximum) covering outcome, implementation
boundaries, temporary acceptance workflow, definition of done, failure cases,
and unresolved decisions. Use the current API. You own only your assigned draft
or critique. Other agents are active; do not edit their files or product code,
run live probes, commit, or push. The root agent is writing the actual temporary
workflow and will fold its baseline result into the final plan.
