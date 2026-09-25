---
name: gimbal-runs
description: >
  Use existing Gimbal workflows in a project: choose and invoke a command,
  understand goals and proof, observe runs, steer sessions or loop planners,
  and assess results. Includes promise, chapter, and sprint terminology.
---

# Use Gimbal

Use the installed command to run a workflow against a project. You do not need
to write Go or build Svelte to call an installed workflow. For a new workflow
use [Author workflows](../gimbal-workflows/SKILL.md); for building the application
use [Build and release](../gimbal-release/SKILL.md).

## Choose a workflow and understand its contract

```sh
gimbal --help
gimbal run --help
gimbal run review --help
```

Installed help tells you what is available and which inputs and model flags
it accepts. The presence of a `df-*` skill does not make it a Gimbal command.

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
gimbal run review --work-dir /abs/project --no-web --goal "Review the parser changes for correctness; report concrete findings with evidence."
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

OpenCode models are explicit: `opencode/<model-id>` selects OpenCode's
`opencode` provider, while `opencode/<provider>/<model-id>` selects another
provider configured in OpenCode. Both forms work on workflow role flags and on
`gimbal run-prompt --model`. The shared server starts on first use and survives
session close. Its state defaults to `~/.gimbal/opencode`, or
`GIMBAL_OPENCODE_DIR`; `gimbal opencode start|stop [--state-dir DIR]` manages
it directly, and stop interrupts active work. Raw request/result/SSE captures
are written to `<state-dir>/captures/`.

Another useful profile spends frontier capability on planning, then assigns
low-risk, reversible, independently checkable background work to Terra or
Sonnet. Prefer it when wall-clock time is cheap and the workflow has bounded
validation feedback that can reject a weak completed attempt and try a better
assignment. The plan is not proof, so keep independent validation appropriate
to the risk. Do not assume the workflow retries provider, harness, or execution
errors; only an explicit retry path does that.

The workflow command remains running until its work finishes. Keep that process
alive while watching or steering from another shell. `--no-web` disables the
browser listener while retaining local control and records. Omit it for the
browser experience; `--port 0` selects an available port. `--uds` serves the
web listener through a Unix socket when needed.

## Find the run and observe its work

```sh
gimbal runs --work-dir /abs/project
gimbal watch RUN_ID --work-dir /abs/project
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
gimbal steer RUN_ID --work-dir /abs/project --session SESSION_ID "Investigate the failing parser test before expanding the review."
gimbal steer RUN_ID --work-dir /abs/project --loop LOOP_SCOPE_ID "Prioritize the CLI work in the next assignment."
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

Saved records remain in `/abs/project/.gimbal/runs/RUN_ID/`. To inspect history
in the browser, run `gimbal --port 0` from the project directory. Live `watch`
needs a running owner; viewing saved history does not revive its sessions.
Gimbal has no generic resume command. Repository promise helpers may define
their own resumable state, schedules, and badges; those are separate
capabilities.

For direct inspection, start with stored run and turn results, then the
relevant session transcript or recorded command output. Follow the consumer
project's policy for retaining or sharing evidence.

## Run practical user testing

`gimbal run validate-product --suite-file /abs/suite.yaml --instance-dir /abs/instance --project /abs/project --follow`
runs one to three caller-assigned workloads with Opus 5.5, reviews their
screenshots with Gemini Flash, and synthesizes findings. The suite requires
`issue_repo` for the tested product. The final agent checks for duplicates,
uploads supporting screenshots
with `gimbal upload-artifact`, and publishes supported findings there. After each
workload, the same tester session
answers one follow-up: its three favorite and three least favorite aspects of UX
and UI separately, with concrete examples. The debrief is appended to the task
report; elapsed workload time excludes it. Supply the product, local assignment
files, isolated workspaces, startup/readiness commands or existing URLs, and an
output directory. Start the selected instance; the workflow command admits
`/abs/project` on its first start when it was not admitted at startup.
`--follow` waits for triage and issue publication. The command's help describes the
JSON/YAML input and model overrides. Publishing requires authenticated `gh`
and a configured public artifact destination.

Assign useful tasks rather than exhaustive feature checklists. Testers must never
inspect the tested product's source. They capture captioned screenshots and report
task success separately from usability; video is for optional human review.
Install FFmpeg with libx264 before running it. `reports.json` points to each
2.5× H.264 `video.mp4`, capped at 1280×720 and ready for `gimbal upload-artifact`;
the raw `video.webm` remains beside it.
When testing Gimbal by having it build another project, use the delegated run's
own web listener for live UI/API monitoring. A standalone history viewer can
retain stale snapshots of runs owned by another process.

## Work from a conversation

Run `gimbal --port 0` from a Git checkout and open **Conversations**. Choose
Codex, Claude, or agy/Gemini when creating a conversation; Gimbal creates its
worktree and branch. Ask the agent to start a review or implementation, then
open the linked run to monitor it in the same server. Implementation needs a
local definition-of-done file in the conversation's worktree. Keep the server
running while a turn or workflow is active. After a restart, saved Codex
conversations resume their native thread on the next message. Saved Claude and
agy/Gemini conversations remain readable history and cannot continue yet.

## Report what happened

Give the workflow, project, model, run ID, useful result, and observed evidence.
When steering, include the target, delivery result, and subsequent behavior.
State failed or unproved requirements separately from completed work. Keep
the report proportionate to the task.
