# Durable Interactive spec and delivery plan: Opus review, round 05

Outcome: **material findings remain**

## Target

- `specs/durable-interactive.md` at `afd5958a` (673 lines).
- `ephemeral/research/durable-interactive/DELIVERY-PLAN.md` at `afd5958a`
  (152 lines).
- Both were read through `git show afd5958a:<path>`. Source was read from a
  snapshot extracted from the same commit, not from the working tree, which
  implementation is changing concurrently.

No caller narrowing was given, so none was ignored. Human decisions recorded
in the spec, the plan and `REVIEW.md` are treated as settled and are not
re-raised. Those decisions are:

- no version policy;
- no native resource policy;
- SERVICE restoration and rechecks are excluded;
- no abandon outcome or eventual-settlement promise;
- authored defer/recover and timing observations are rejected;
- every occurrence starts human-first;
- `EndedBy` is the ending provenance;
- RunCommand returns complete output;
- questions are asked in a batch.

## Evidence inspected

- Repository rules: `AGENTS.md`, `docs/definition-of-done.md`,
  `ephemeral/research/durable-interactive/SOURCE-MAP.md`, the prior round-04
  review, and its adjudication in `REVIEW.md` and the worklog.
- The diff `3270c21e..afd5958a` for both target files, which is the round-04
  response.
- Remote placement:
  - `internal/runner/daemon.go`: the WorkflowDaemon, its run index, admission
    and control.
  - `cmd/gimbal/daemon.go`.
  - `internal/taskdaemon/{client,daemon,retention}.go`.
  - `internal/invocation/invocation.go` (`PrepareContext` task routing).
  - `internal/host/routes.go`: route sidecars, `routedController` and
    `RestoreProjects`.
- Local hosting:
  - `internal/host/host.go`: `Project.Run` and `Project.Start`.
  - `internal/runner/{runner,launch}.go`: lock, controller, recover and
    Setsid spawn.
  - `registry.go`: `LoadPlugins` and `RegisterWorkflow`.
- Execution:
  - `internal/agentexecutor/agent.go`: `RunTurn`, `Steer` and deadline
    copying.
  - `internal/executor/files.go`: Vacuum and Claim.
  - `group.go`, `session.go` dispatch and `interview.go`.
- Admission: `internal/gimballint/rules.md`, the analyzer and its testdata,
  and the Justfile `vet` recipe.
- Conversation product:
  - `internal/conversation/*`: ConfigureModel, CacheFile, SetupEnvironment,
    remote, Associate, Worktree and Rename.
  - `web/src/routes/conversation.remote.go`.
  - `web/src/routes/projects/[project]/conversations/**/page.server.go`.
  - `web/src/lib/run/RunQuestions.svelte` and the StopTurnButton/SteerBox
    usage.
  - `cmd/examples/interview/main.go`.
  - `internal/workflows/{interview,implementinterview}`.
- Cell baselines: every "Current lines" value in the cell table was
  re-measured over the stated ownership sets at `afd5958a`. All six match: J
  913, E 7,428, I 102, H 5,329, P 13,787, A 11,526.

## Round-04 dispositions

| Round-04 finding | Disposition at `afd5958a` |
| --- | --- |
| F1 native continuity path | Resolved. The spec now uses the executor checkpoint/reattach path (spec 396-406). E's set includes `internal/{agentexecutor,executor,…}` and E's maximum is 9,000. The `codex/` files left H. |
| F2 shared-host failure containment | Resolved within its decided scope. The per-run panic boundary is stated (spec 477-480) and claimed in C9. Service restoration is excluded, as decided. |
| F3 Run semantics during dormancy | Resolved. Run stays pending, same-ID re-entry reconstructs the run, and the deadline is recorded once (spec 408-426). |
| F4 admission precision | Resolved as language: an allowlist, rejection of unknown opaque calls, and the map-range forms (spec 534-547). The enforcement point is still open; see F4 below. |
| F5 dormant cancellation and reconciliation | Resolved. Existing cancellation applies, reconciliation can be retried, and no abandon outcome is added (spec 508-519, C10). |

## Pass summary

| Pass | Result |
| --- | --- |
| Consistency | Mostly consistent. The ownership-set rule in spec 167-172 is stricter than plan 64-65 (F5). The inbox kinds in spec 252-259 have no state for an operator stopping a turn, which the current product offers (F2). |
| Claim traceability | One requirement has no claim: non-local placement durability (spec 488-492) is a stated requirement, but no C- or L-claim exercises remote placement (spec 595-612). See F1. |
| Seam audit | One seam fails. The remote environment (WorkflowDaemon/taskdaemon over HTTP) is an external seam with no integration claim. That violates the spec-writing rule that every external seam carries one. See F1. |
| Claim classification | Correct for what is claimed. Cell-local claims stay within one cell, and L1-L4 name every cell they cross. |
| Derivability | Gaps: the product features lost when the conversation manager is replaced (F3), and where admission is enforced (F4). Two implementers would make different choices on each. |
| Pump hygiene | Clean. The round-04 adjudications are recorded in `REVIEW.md` and the worklog and are reflected in both documents. The status lines (spec 3-4, plan 3-4) are honest that nothing is implemented or proved. |

## Findings

### F1 (critical): non-local durability has no owner, budget or integration claim

- **Pass:** claim traceability and seam audit.
- **Spec requirement:** spec 488-492 requires durable behaviour in "local and
  non-local workflow placement in the selected execution environment". Remote
  input is "routed to that environment's durable owner".
- **Source at `afd5958a`:** no durable owner exists in a non-local environment.
  - `internal/runner/daemon.go:27` keeps the run index as
    `projects map[string]string` in memory. After a daemon restart, control
    returns "workflow not found" (`daemon.go:37-39`).
  - `/workflows/control` calls
    `Connect(filepath.Join(project, ".gimbal"), v.Run)` (`daemon.go:156`).
    That is a live per-run unix socket. A dormant run, whose Go activation
    has unwound by design, has no socket and cannot accept a message or
    archive.
  - `internal/taskdaemon/daemon.go:57,103-117,177-179` keeps sessions in an
    in-memory map. A restarted daemon answers "session not found" for any
    session it held before.
  - `internal/taskdaemon/client.go:45` refuses `steer` on messaging routes.
    C7's active-input steering therefore cannot work for a remotely placed
    agent.
  - `cmd/gimbal/daemon.go:42` documents "task-daemon crash recovery is not
    supported."
  - `internal/invocation/invocation.go` (PrepareContext) replaces every
    role's adapter with `taskdaemon.Harness` when a task endpoint is
    configured. Non-local agent turns therefore bypass the direct
    agentexecutor continuity that spec 396-406 relies on.
- **Ownership gap:** `cmd/gimbal/daemon.go` and all 1,409 lines of
  `internal/taskdaemon/` are outside every ownership set (spec 145-165). The
  cell budgets cannot absorb a durable remote owner.
- **Missing claim:** none of L1-L4 places a run remotely or restarts a remote
  daemon. The plan's single sentence "Remote placement carries input through
  existing ControlWorkflow transport" (plan ~137) describes a transport. It
  does not describe an owner that survives dormancy.
- **Impact:** an implementer meets this either by silently building a durable
  remote daemon (unbudgeted, unowned and unproved) or by shipping remote runs
  that lose their input once they go dormant. Either outcome breaks a written
  requirement.
- **Proposed amendment (HITL), choose one:**
  - (a) Scope it: add `internal/taskdaemon/**` and `cmd/gimbal/daemon.go` to
    H with a raised maximum. Require persisted daemon run and session
    indexes, durable-inbox control that does not depend on a live run
    socket, and steering on messaging routes. Add an L-claim: start remotely,
    go dormant, restart the remote daemon, send, and see the same run and
    native session continue.
  - (b) Exclude it: replace "This applies to local and non-local workflow
    placement" with a statement that this delivery makes local placement
    durable. Non-local runs keep today's live-activation behaviour, and a
    dormant remote run reports unresolved rather than accepting input it
    cannot deliver. Record the exclusion in the exclusions list.

### F2 (issue): stopping a turn without finishing the conversation is unspecified

- **Pass:** consistency and derivability.
- **Spec:** inbox kinds are only `MESSAGE | FINISH` (spec 252-259). The
  conversation loop returns on any Interactive error (spec 214-220). The
  only interruption the spec defines is the one archive causes (spec 47-50,
  C8).
- **Source:** the product offers Stop today without ending the chat.
  - `web/src/routes/conversation.remote.go:90` submits a `Kind: "interrupt"`
    request, and the conversation continues afterwards.
  - StopTurnButton appears in the run view and in SteerBox.
  - In a Group, a Killed child does not cancel its siblings, but other errors
    do (`group.go`).
- **Impact:** once the conversation manager becomes an ordinary workflow, the
  spec does not say what an operator's turn stop produces:
  - a return to the durable wait;
  - an Interactive error, which under the conversation loop ends the
    conversation with an error;
  - or whether the Stop control is removed.

  Each implementer will pick differently, and the obvious one (the turn
  context is cancelled, so Interactive returns an error) quietly turns
  today's Stop into the end of the conversation.
- **Proposed amendment (HITL):** state the outcome. For example: "An operator
  turn stop interrupts the active turn through the adapter's interruption
  contract; Interactive records that turn's partial or absent reply and
  returns to the durable wait; it is not FINISH and not an error." Name the
  control that triggers it, add it to C7/C8 or a new I claim, and say whether
  non-conversation runs keep StopTurnButton with the same meaning.

### F3 (issue): no inventory of conversation features kept or dropped in the replacement

- **Pass:** derivability.
- **Spec:** spec 481-484 says to "Replace separate conversation execution
  ownership with ordinary workflow admission, operations, controls and
  records", and that chat naming and workspace metadata "may remain product
  data".
- **Source:** `internal/conversation` (in H) and
  `web/src/routes/conversation.remote.go` (in P) provide more than naming:
  - ConfigureModel changes the model mid-conversation, exposed through the
    `configureConversationModel` command.
  - CacheFile caches a project file for discussion, used by the
    project-file discussion route.
  - SetupEnvironment and `remote.go` handle conversation-specific remote
    environment selection.
  - Associate links runs to a conversation, and the conversation has its own
    worktree.
  - Rename, cleanup and setup queries.
- **Impact:** "may remain" covers naming and metadata only. A model change in
  the middle of an Interactive, or a file-discussion entry point that has to
  seed the human-first message, are behaviours that need a workflow-level
  answer. Without a keep-or-drop list, implementers either drop them
  silently, which is a regression the human never chose, or rebuild them as
  new mechanisms, which breaks "build only what was asked".
- **Proposed amendment (HITL):** add a short table listing each existing
  conversation feature (model change, file discussion, remote environment,
  run association/worktree, rename, cleanup) with *kept as product data*,
  *expressed in the conversation workflow as X*, or *dropped in this
  delivery*. Where a feature is kept, add it to C11 or L-claim coverage.

### F4 (issue): where admission is enforced is not named

- **Pass:** derivability and claim classification (C12).
- **Spec:** "Diagnostics reject prohibited authored code" (spec 532-533).
  Built-ins "admit them through the same checks as project workflows" (spec
  548-550). C12 claims that prohibited code "is rejected".
- **Source:**
  - Today's checks are the advisory `gimbal lint` / `go vet -vettool`
    analyzer (`internal/gimballint/rules.md`), run by `just vet`.
  - `RegisterWorkflow` (`registry.go`) only requires a generated Graph.
  - `LoadPlugins` opens compiled `.so` files, which have no source to
    inspect.
  - Nothing at generation, registration or hosted admission refuses a
    workflow the analyzer would flag.
- **Impact:** "rejected" can mean a lint finding the author may ignore, or a
  hard gate that stops the workflow from being generated or registered. The
  durability guarantees in E depend on admission actually holding. A
  lint-only reading lets an unchecked project plugin dormant-unwind a stack
  with authored `defer`s or `select`, which is exactly the failure the rules
  exist to prevent. The two readings differ in which cell owns the gate and
  in what C12 must show.
- **Proposed amendment (HITL):** name the gate. For example: "`go generate`
  (gimbalgen) runs the admission analyzer and refuses to emit a workflow's
  generated registration when it reports a violation; a workflow without
  generated registration cannot be registered or hosted." Alternatively,
  state that admission is lint-time only and that hosting trusts it. C12 then
  proves the named gate.

### F5 (nitpick): ownership sets miss known touch points, and the plan's rule is narrower than the spec's

- **Pass:** consistency.
- **Spec and plan:** spec 167-172 says "necessary changes outside these sets
  must be assigned and budgeted before implementation". Plan 64-65 only asks
  owners to report "before adding production files outside the spec's
  measured ownership sets". That rule covers new files, not edits to
  existing unowned ones.
- **Known touch points outside every set:**
  - `web/src/routes/projects/[project]/conversations/page.server.go` and
    `[conversationID]/page.server.go`. Both import `internal/conversation`,
    and P names only `+page.server.ts`.
  - The conversation `+page.svelte` routes.
  - `cmd/gimbal/daemon.go`, unless F1(b) is chosen.
  - The header-less copies under `internal/skgo/links/`.
- **Related small gaps, each suitable for an issue rather than a spec
  rewrite:**
  - `internal/host/host.go:325` (`Project.Run`) mints
    `runstore.NewRunID()` on every call. The maintained interview example
    reaches Interactive through this path, so after restart it cannot
    re-enter "with the same existing run ID" (spec 411-412) unless the ID is
    persisted.
  - "Installed ingress" (spec 414) is not defined for a direct `gimbal.Run`
    caller.
  - An ordinary, non-conversation workflow's Interactive has no stated human
    finish affordance. Archive is a conversation-tab action, and
    `RunQuestions.svelte:24` makes the answer field `required`.
  - The executor dedups terminal operations for seven days
    (`internal/executor/files.go:383`), but dormancy is unbounded. A run
    reconstructed after that would see accepted turns as new submissions.
    That is a blind redispatch edge, outside the "without blind native
    redispatch" promise in spec 424.
- **Proposed amendment (HITL):**
  - Add the listed paths to P/H (or to H under F1(a)).
  - Change plan 64 to "before adding or changing production files outside…".
  - Record the four small gaps as issues, or as one sentence each in the
    spec.

## Delivery plan notes

- The plan follows the spec's cells, keeps proof as observed runs rather than
  committed artefacts, and uses the cheapest unversioned aliases.
- Its work items 3-4 inherit F1 and F3 directly. Item 3's "existing project
  and placement paths" includes the remote path whose durable owner does not
  exist.
- The two owners plus a manager can parallelize safely once F1 and F4 are
  settled. Both decide which cell owns a gate, so leaving them open invites
  cross-owner conflict mid-slice.
- No proof program, run log or results dump is planned. That complies with
  `AGENTS.md`.

## Outcome

material findings remain
