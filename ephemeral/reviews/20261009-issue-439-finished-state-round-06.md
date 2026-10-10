# Adversarial review: issue 439 proposal, finished application state, round 06

Reviewer: Claude Fable 5.1, same session as rounds 01 to 05, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `ff26f254` (`docs: align proposal with named execution
environments`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)
- the unmerged runtime backend the revision now adopts, read at the commit
  the proposal cites

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 05 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 05: `git diff dd4d21a0..ff26f254` touches only
`ephemeral/`: `proposal.md` (TL;DR placement paragraph), `topics/
backend-contract.md` (named environment as placement unit; container-hosted
providers; Postgres worker events not yet the authoritative history),
`topics/delivery-and-review.md` (per-provider provisioning and OpenCode
stop reconciliation; align with the unmerged runtime backend),
`topics/execution-placement.md` (section renamed "Named environments place
work; roles choose the agent", five rewritten paragraphs, handoff and
demonstration paragraphs), `topics/interchange.md`, `topics/ownership.md`
(Codex turn path must become attach-only; new "Service lifecycle authority"
paragraph), `topics/storage-and-remote.md`, `topics/workflow-discovery.md`,
and the worklog (two entries). No Go, Node, `justfile`, or `plugins/`
change: `git diff dd4d21a0 --stat -- . ':!ephemeral'` is empty. This branch
has no `execution.go` and no `internal/execution/`.

Unmerged backend, read at `/Users/tyler/.codex/worktrees/gimbal-temporal-backend/gimbal`,
HEAD `013559e85701cb424696b41fd396391a05a223ec` as the proposal cites:
`execution.go` (whole file: `ExecutionBackend`, `ExecutionEnvironment`,
`ExecutionCommand`, `ExecutionProcess`, `InEnvironment`, `WithExecution`),
`session.go:42-63` (environment-bound sessions resolve the adapter on the
first turn), `internal/execution/temporal.go:1-10` (package doc: Linux
worker binary, consumer Docker image carrying Codex), `:154-245` (`New`:
table creation, `owner: uuid.NewString()` per backend instance),
`:281-345` (`Resolve`: insert bootstrap row, start one worker container per
environment), `:438-475` (`Close` removes containers from the in-memory
list only), `:477-538` (`Environment.remove`: "There is no restart"),
`:553-558` (`Harness` returns nil unless the role's binding is `codex`),
`:871-913` (`Stop`: cancel activity, remove the environment when
unconfirmed), `internal/execution/harness.go:1-30,103-200`,
`internal/execution/worker.go:48` (the only `SELECT` on
`gimbal_environments`, run by the worker for its own bootstrap), and
`ephemeral/worklog/20260927-browser-evaluator.md` (final section "Awaiting
worker credentials", recorded by the cited commit). The three cited source
files are committed at that HEAD; the dirty files in that checkout are
unrelated provider/browser docs and tests. The cited draft
`temporal-docker-interface-design-2026-10-09.md` exists (31,266 bytes,
"Status: draft design, under independent review. Analysis only").

Mainline anchors unchanged from round 05: `codex/codex.go:96-98,401,475`,
`codex/rpc.go:79-103`, `opencode/client.go:107`,
`opencode/server.go:64-123`, `cmd/gimbal/main.go:157-205`.

## Disposition of round 05 findings

1. **Gimbal-provisioned OpenCode stop kills attached work: resolved as a
   decision.** `ownership.md:51` separates service lifecycle authority from
   run authority, names `gimbal opencode stop` as a service-wide action,
   preserves its evidence against the recorded provider generation, and
   confirms descendant cessation only where containment or provider
   evidence supports it. `delivery-and-review.md:12` demonstrates the stop
   with a run attached. The worklog records the rejected alternatives
   (cross-instance force-flag registry; unconditional descendant-death
   inference). Consistent with the tree.
2. **Codex turn path not attach-only: resolved.** `ownership.md:49`.
3. **Codex provisioning and endpoint unnamed: resolved.**
   `delivery-and-review.md:12`.

## Findings

### 1. Issue (incomplete requirement): the adopted runtime backend has no path to find or stop a dead runner's environments, and the proposal does not add one

**Requirement.** `requirements.md:19`: "each participating execution site
must have run-owned subprocess lifetime management."
`backend-contract.md:13`: "The backend must recover the execution units
belonging to that identity after a controller restart."
`execution-placement.md:43`: "recovered backend control must identify and
stop remaining owned work rather than replay that body."

**What the adopted seam does.** `New` mints a fresh backend owner
(`temporal.go:245`, `uuid.NewString()`) per `Run`. `Resolve` inserts a
bootstrap row keyed by that owner and starts one worker container per
environment (`temporal.go:281-333`). The only removal paths are
`Backend.Close`, which iterates the in-memory container list of the running
runner process (`temporal.go:438-456`), and `Environment.remove`, which
the same process calls after an unconfirmed cancellation
(`temporal.go:485-514`). No code outside the worker's own bootstrap read
(`worker.go:48`) selects from `gimbal_environments`, and a new backend
cannot see rows of a previous owner.

**Concrete failure.** A hosted run resolves `"linux"`, dispatches a
long-running command and a Codex turn, and its runner is killed. The
worker container keeps running the command, the Codex session, and any
browser; the bootstrap row stays; the Temporal activities keep heartbeating
for up to the 24 h start-to-close timeout (`temporal.go:606`). The
replacement server knows the run from `owner.json`, but neither
`runner.json` nor the run record carries the backend owner ID, scoped
environment name, task queue, container name, or state volume, because the
proposal never asks for them. Cancel-all marks the run unreachable and has
nothing to signal. This is the same hole the local branch closes with
registered unit records (`ownership.md:39`), left open for the first remote
environment the proposal now builds on.

**Smallest response.** Treat a resolved environment as a registered
execution unit: before the first dispatch into it, the runner persists the
environment's bootstrap identity (owner, scoped name, queue, container name,
state volume, Docker host) into its run record alongside the local unit
records, and recovery removes containers by that recorded identity after
checking the container's owner label, writing the removal into the run
history. Add "kill the runner during a remote command and a remote turn,
restart the server, confirm the container is removed and the run
reconciled" to the demonstration list.

### 2. Issue (overstated evidence, misread requirement): "named Linux execution exists there" omits that remote harness turns are unauthenticated and that role mismatch is detected at the first turn, not at admission

**Evidence claim.** `execution-placement.md:41`: "Named Linux execution
exists there; a usable macOS native environment remains an integration
obligation." `execution-placement.md:45`: "match the selected
environment/role to the actual worker's OS, provider, required app
permissions, and capabilities before admission."

**What the cited commit records.** The very commit the proposal pins,
`013559e8`, adds the section "Awaiting worker credentials" to that branch's
worklog: "Authenticated supervisor concurrency/steering, agent access to
scoped inputs/browser, and the full Docker/Temporal evaluator with final
reports/media remain unproved. The worker API-key-file choice is still
pending; the goal is not complete." The remote `Harness` supports exactly
one adapter, `codex`, and returns a nil adapter for every other role
(`temporal.go:553-558`). A session bound to an environment learns this only
when `Generate` runs ("the backend supplies the role's adapter on the first
turn; operational resolution failures are returned by Generate",
`session.go:46-48`), after the workflow may already have done work in that
environment.

**Impact.** The TL;DR now rests the mixed-platform story on this seam
("The active runtime backend work selects a named execution environment;
roles choose the agent within it", `proposal.md:9`). As evidence, commands
in a Linux container are demonstrated; authenticated harness turns are not,
and non-Codex roles fail late. Under the repository's proof rule, the
proposal should say what was seen and no more.

**Smallest response.** In `execution-placement.md:41` replace "Named Linux
execution exists there" with "remote Linux commands are demonstrated;
remote Codex turns await worker credentials; the remote harness is
Codex-only", and in the demonstration list require that an environment
whose roles are not all bound to a supported adapter is rejected at
`Resolve` or `NewSession`, not at the first `Generate`.

### 3. Nitpick: a single unconfirmed stop removes the whole environment, which the proposal does not surface

`Environment.remove` is called by any `Process.Stop` whose activity
completion is unconfirmed (`temporal.go:883-900`) and "ends every turn,
command and browser in it. There is no restart" (`temporal.go:485-490`).
One service whose cancellation times out therefore terminates every other
live session and command of the same run in that environment. That is
inside the run boundary and is a defensible containment choice, but the
run history must then record the removal as the cause of the other units'
termination, and the proposal's "Start dispatches its process work"
(`execution-placement.md:39`) should name the blast radius so a workflow
author does not place an expendable service beside a long turn.

### 4. Nitpick: the proposal's authoritative source for the seam is a sibling worktree path with dirty state

`execution-placement.md:41` cites three absolute paths under
`/Users/tyler/.codex/worktrees/gimbal-temporal-backend/gimbal/` and "additional
uncommitted work in that checkout". The three files are committed at the
pinned HEAD, so the design is reproducible from the branch
(`codex/temporal-container-backend` per the cited draft, PR 398). Cite the
branch and commit rather than a worktree path, and drop the uncommitted-work
qualifier, so a reader after that worktree is gone can still find the
source. The round-05 observation that the proposal now has two placement
routes (`execution-placement.md:45`) is adequately handled by making
deployment configuration the authority for both.

## What the revision gets right

- The seam it adopts is real, small, and matches the rule "as simple as
  possible": ordinary Go stays in the runner, standalone activities run
  remotely, no workflow compilation is required, and the TL;DR says so.
- Dedicated run-owned containers may host their own provider and die with
  the container, which correctly separates them from externally shared
  daemons.
- The OpenCode service-stop case is now a named service-wide action with
  evidence recorded against provider generation, and the alternatives that
  would have added a registry or over-inferred death were rejected.
- Postgres worker events are explicitly not yet the authoritative history.

## Outcome

material findings remain
