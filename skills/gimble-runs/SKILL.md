---
name: gimble-runs
description: >
  Use existing Gimble workflows in a project: choose and invoke a command,
  understand goals and proof, observe runs, steer sessions or loop planners,
  and assess results. Includes promise, chapter, and sprint terminology.
---

# Use Gimble

Use the installed command to run a workflow against a project. You do not need
to write Go or build Svelte to call an installed workflow. For a new workflow
use [Author workflows](../gimble-workflows/SKILL.md); for building the application
use [Build and release](../gimble-release/SKILL.md).

## Choose a workflow and understand its contract

```sh
gimble --help
gimble run --help
gimble run review --help
```

Installed help tells you what is available and which inputs and model flags
it accepts. The current built-in is `review`, a read-only code review. The
presence of a `df-*` skill does not make it a Gimble command.

Establish the goal, target project, permitted changes, and what would
demonstrate a useful result. A supplied chapter or sprint can provide context;
use it when relevant rather than manufacturing planning artifacts for every
invocation. See the [glossary and proof guidance](references/glossary-and-proof.md)
for the distinctions among promises, chapters, sprints, tasks, and runs.

Workflow help should explain purpose, inputs, outputs or changes, completion
expectations, and limits. If a material part is missing, resolve it from the
workflow's source or the user rather than guessing at its authority.

## Start a run

```sh
gimble run review --work-dir /abs/project --no-web --goal "Review the parser changes for correctness; report concrete findings with evidence."
```

Use an absolute project path. Do not pass role-model flags by default: the
workflow's displayed defaults are its intended role configuration, and a
supplied flag replaces one. Override a role when the user requests a particular
model or effort, when its default cannot run and the user approves the
replacement, or when the workflow's own help identifies the present work as a
clearly bounded case for a cheaper model. In that last case, prefer the
documented Sol or Opus selection for routine work, but retain the configured
frontier default when the help's ambiguity, risk, or difficulty criteria apply.
Do not turn general cost pressure into blanket overrides across unrelated
roles. Before an override, read every role line in `--help` and state the
resulting mapping. Record the models actually used in the result report.

The workflow command remains running until its work finishes. Keep that process
alive while watching or steering from another shell. `--no-web` disables the
browser listener while retaining local control and records. Omit it for the
browser experience; `--port 0` selects an available port. `--uds` serves the
web listener through a Unix socket when needed.

## Find the run and observe its work

```sh
gimble runs --work-dir /abs/project
gimble watch RUN_ID --work-dir /abs/project
```

Use the same project directory. `runs` returns JSON with active runs from
responding local runtimes. `watch` emits an initial snapshot followed by
newline-delimited JSON changes. Read run status, scopes, sessions, and turns
to understand what is happening.

A turn without an end time identifies an active session. Copy that session's
exact ID. Role names, session IDs, turn IDs, and loop scope IDs identify
different things. An empty discovery result may mean the runtime is absent
or unreachable; it says nothing conclusive about saved history.

## Steer a session or the next planning decision

```sh
gimble steer RUN_ID --work-dir /abs/project --session SESSION_ID "Investigate the failing parser test before expanding the review."
gimble steer RUN_ID --work-dir /abs/project --loop LOOP_SCOPE_ID "Prioritize the CLI work in the next assignment."
```

Choose exactly one target. A session steer goes to that session's active turn;
`landed: false` means it was dropped, not saved for a later turn. Loop steering
queues a message for the next planning decision of that dispatching loop.

Give an actionable correction and relevant evidence, then observe what the
agent does. `landed: true` establishes delivery; `queued: true` establishes
queueing. Neither establishes that the instruction was followed.

## Assess the result and its proof

Inspect the work or behavior relevant to the goal. A review finding should
point to a concrete defect; an implementation claim needs observation of the
changed behavior. Tests and command results are evidence for what they
actually exercise. Recorded failure cannot be overruled by prose saying it
passed.

A run can finish, and a planner can stop dispatching, while the requested
outcome remains unmet. For a promise, identify the claim, scope, evidence,
verifier, and inspected state. Use independent validation when the workflow
or acceptance contract calls for it; the validator inspects the result rather
than accepting the worker's account as proof.

Saved records remain in `/abs/project/.gimble/runs/RUN_ID/`. To inspect history
in the browser, run `gimble --port 0` from the project directory. Live `watch`
needs a running owner; viewing saved history does not revive its sessions.
Gimble has no generic resume command. Repository promise helpers may define
their own resumable state, schedules, and badges; those are separate
capabilities.

For direct inspection, start with stored run and turn results, then the
relevant session transcript or recorded command output. Follow the consumer
project's policy for retaining or sharing evidence.

## Work from a conversation

Run `gimble --port 0` from a Git checkout and open **Conversations**. Choose
Codex, Claude, or agy/Gemini when creating a conversation; Gimble creates its
worktree and branch. Ask the agent to start a review or implementation, then
open the linked run to monitor it in the same server. Implementation needs a
local definition-of-done file in the conversation's worktree. Keep the server
running: after a restart, saved conversations are readable history, but their
provider sessions cannot continue yet.

## Report what happened

Give the workflow, project, model, run ID, useful result, and observed evidence.
When steering, include the target, delivery result, and subsequent behavior.
State failed or unproved requirements separately from completed work. Keep
the report proportionate to the task.
