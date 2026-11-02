# Adversarial review: conversation environment declarations and preflight

Reviewer: Sonnet 5.5, independent session, round 01.
Outcome: **material findings remain**

## Target

- Checkpoint `9fe8e769` (`feat: declare logical task environments before execution`) in
  `/Users/tyler/.codex/worktrees/1430/gimbal`.
- Measured against `ephemeral/plans/conversation-declarations-task.md`, the parent
  `ephemeral/plans/distributed-execution-slice.md` (R1, R2, R4, D1, D2, C1) and the
  repository `AGENTS.md`.
- The checkpoint is immutable. I read it only through `git show 9fe8e769` and
  `git archive 9fe8e769`, extracted to `/tmp/cp9fe` (outside the repository). Every probe
  and test run below was executed in that copy. The working tree
  (`cmd/gimbal/*`, `plugins/messaging-river/*`, nine `*/workflow_gen.go` files) belongs to
  the next River task. I used it only to see what later edits changed, never as reviewed source.

No repository file other than this artifact was written.

## Operating-constraint note

The launch prompt only supplied the target, the artifact path and the read-only boundary.
It did not narrow the scope, so nothing was ignored. The scope was derived from the task
file, the parent plan and `AGENTS.md`.

## Evidence inspected

- The full 37-file diff of `9fe8e769` (production, generated, tests, web types, worklog).
- Surrounding code at the checkpoint:
  - `effects` (`Submit`, `Call`, `Route`, `Environment*`), `environment_messaging.go`
  - `resource_registration.go`, `internal/invocation/invocation.go`
  - `internal/runner/runner.go`, `internal/host/host.go`
  - `internal/conversation/{manager,remote,requests}.go`
  - `internal/environments/service.go`
  - `cmd/examples/interactive`, `cmd/gimbal/daemon_native_test.go`
- Test run at the checkpoint copy: `go test . ./internal/invocation ./internal/generate
  ./internal/host ./internal/conversation ./internal/effects ./internal/runner
  ./internal/taskdaemon`. All packages passed. `go build ./...` reports only the
  `main`-less plugin packages. These are c-shared plugins, so I did not treat that output as a
  checkpoint defect.
- Temporary probes added only to `/tmp/cp9fe` (a fixture package, a second fixture package,
  and two `_test.go` files). Their output is quoted below.
- The working-tree diff of the nine `workflow_gen.go` files (only to establish what the
  generator emits that the checkpoint does not contain).
- `ephemeral/worklog/202611012137-conversation-placement.md` and
  `ephemeral/worklog/distributed-execution-slice.md` at the checkpoint.

What the checkpoint does well (verified by tests and reading):

- Literal and direct-input extraction, with mutation and alias protection of the entry
  parameter.
- The fail-closed fallback for `WithEnvironment` calls inside syntax the walker skips.
- Pure preparation that rejects empty, missing, undeclared and physical-route selections
  before effects.
- Session creation, turns and close capture the creation-time name.
- The host passes the selected name and bus separately.

## Findings

### 1. Issue: the environment inventory is not complete, and unfollowed selections pass preflight

Requirement: R2 ("The registered definition/graph describes every referenced environment...
Arbitrary computed or indeterminate selectors are rejected"). Task: "Generated graph/catalog
metadata carries literal names and input-field references... rejects ... undeclared/computed
selections before workflow effects." D1 ("invalid or absent names reject before any
workflow/native effect").

`internal/generate/graph.go:432-450` backstops only `WithEnvironment` calls that appear as direct
`gimbal.WithEnvironment` calls in the entry function body. Two other routes bypass it, and
`workflow.Graph.Diagnostics` is not consulted by `invocation.PrepareContext` (the only gate is
`len(graph.Environments) > 0`, `internal/invocation/invocation.go:80`). I extracted fixtures into
the checkpoint copy and ran `generate.Extract`:

```
CrossPackage: envs=[] diags=[... a call into .../fixture/other, which imports Gimbal, is not followed]
FuncValue:    envs=[] diags=[... a call through a function value is not read]
```

`CrossPackage` calls `other.Pick(ctx)`, where `Pick` returns `gimbal.WithEnvironment(ctx, "sneaky")`.
`FuncValue` does `f := gimbal.WithEnvironment; ctx = f(ctx, "viafunc")`. In both cases the graph
records no environment, so `Prepare` runs no environment checks. The name then reaches execution
unvalidated. The only runtime backstop (`command.go:186`, `service.go:50`, `session.go:1021`) fires
when `effects.Messages(ctx) == nil`. In the new messaging composition `Messages` is non-nil, so an
undeclared name goes straight through to dispatch. At this checkpoint `effects.Submit` happens to
reject it because there is no orchestrator. That is an accident of the unfinished transport, not a
declaration check, and it disappears when named delivery lands.

Impact: the statement "preflight enumerated every selection" is false for workflows whose
selections sit behind a cross-package helper or a function value. The guarantee should fail closed:
treat an unread call that could carry a context (or any non-empty `Diagnostics` entry about
following a call) as an indeterminate environment declaration when the workflow can reach the new
composition, or reject the workflow. No test covers either shape; the added fixtures cover only
in-body computed and mutated cases.

### 2. Issue: nine registered workflow graphs were not regenerated; the committed tree disagrees with the generator

Requirement: task ("Integrate schema/generation changes through generators."), `AGENTS.md`
("`generated/` is written by `go generate` and never by hand"). The worklog the checkpoint itself
commits states "Regeneration must include workflow graphs as well as the application and TypeScript
schema."

The generator was changed to always emit `Environments: []workflow.EnvironmentDeclaration{...}`
(`internal/generate/source.go:321-326`). The commit regenerated only
`internal/workflows/conversation/workflow_gen.go` and `internal/invocation/conversation_gen.go`.
The `workflow_gen.go` files for implementation, implementinterview, pyramidsummary,
researchdocument, researcheval, review, servicecheck, sprintplan and validateproduct were left as
they were. The working tree shows exactly this drift: each of those nine files now gains an
`Environments: []workflow.EnvironmentDeclaration{},` line that the checkpoint lacks.

Impact:
- A fresh `go generate ./...` on the checkpoint leaves a dirty tree, so the checkpoint is not the
  generator's output.
- Those nine graphs carry a nil `Environments`, which serializes as JSON `null`. The checkpoint's
  new web type declares `environments: Array<EnvironmentDeclaration>`
  (`web/src/lib/workflow/types.ts`) and three web fixtures were updated to `[]`, so the production
  graphs and the typed fixtures disagree.
- The "Proof"/"affected checks" claim cannot include "generation" at this revision. All the tests I
  ran passed regardless, so nothing detects this.

### 3. Issue: the `conversation` input schema now requires `environment`, but two other launchers were left sending the old shape

Requirement: task ("Add the environment input to the registered Conversation workflow and pass
the UI/manager selection into its validated input"). Changing a registered schema requires every
caller to supply it (`AGENTS.md`: no compatibility shims; delete what is replaced).

- `cmd/examples/interactive/main.go:74-80` starts workflow `conversation` with
  `{"purpose": ...}` only. I ran the checkpoint's `Prepare` with that body:
  `environment: jsonschema validation failed ... missing property 'environment'`. The example
  compiles and fails at runtime.
- `cmd/gimbal/daemon_native_test.go:97` (`TestNativeRemoteInteractiveDaemonRestart`) still sends the
  old input plus an `http` `TaskRoute`. It is opt-in (`GIMBAL_TEST_NATIVE_INTERACTIVE=1`), so the
  suite is green while the test is invalid. The working tree has since rewritten it, which confirms
  the checkpoint's version was broken.

Impact: a shipped example and the only native conversation restart test are broken at the
checkpoint and nothing in the default suite notices.

### 4. Issue: named environments on the local machine can no longer start a conversation, and no replacement coverage exists

Requirement: R2 ("Conversation placement uses this declared input form ... it is not exempt from
preflight") and the task's statement that existing unrelated routes can remain. This is a
design-level regression that the checkpoint does not resolve or record as a decision.

`Manager.createLocalEnvironment` (`internal/conversation/remote.go:242-260`) creates a
conversation for a registered environment whose machine is "This Mac" and stores
`Environment: in.Environment` (the registered environment's name). `WorkflowStart` passes that name
as the workflow `environment` input with `Route{}`. `PrepareContext` accepts only the literal
`"local"` when the route has no name (`internal/invocation/invocation.go:104-108`), and the unit
test pins this (`{"environment":"bob"}` with an empty route returns an error).
`TestConversationEnvironmentPreflightIsPure` therefore documents that such a conversation is
rejected, and `Manager.Create` calls `WorkflowPrepare` before `createLocalEnvironment`, so creation
fails.

The checkpoint also deleted `TestLocalWorkflowRetainsRemoteTaskDirectoryAndRoute`, which was the
host-level test that started a conversation through `project.Start`, retained a native
working-directory path, drove an interactive finish and waited for completed teardown. Nothing
replaced it. A successful registered `conversation` start through the host with
`environment:"local"` is now covered only by `internal/runner/daemon_test.go` (admission and
dedupe), not by a launch-to-completion test.

Impact: either local named environments are intentionally unsupported (then that decision, and the
now-unreachable `createLocalEnvironment` plus the remote-preparation, `remoteStates` and retained
route machinery in `internal/conversation`, belongs in the plan or must be removed per R4 "Remove
replaced conversation-specific physical-route and recovery paths"), or the preflight needs a way to
accept configured local names. The checkpoint does neither. Related, and expected before the next
task: no production messaging plugin implements `ValidateEnvironment`
(`grep ValidateEnvironment` outside tests finds only the interface field and its call site), so at
this revision every non-`local` conversation, on any route, fails preflight. That is acceptable as
an interim state only if it is not presented as C1 progress; the plan already says it is not.

### 5. Nitpick: one guard expression is copied three times, and an unrecorded owner file

- The condition
  `selected != "" ... Messages(ctx)==nil && (selected != "local" || Endpoint(ctx) != "")`
  is pasted verbatim into `command.go:186`, `service.go:50` and `session.go:1021` with the magic
  string `"local"`. A single unexported helper in `internal/effects` would hold the rule once. This
  is not a new public name, so `AGENTS.md`'s "no new names" rule is not an obstacle.
- `service.go` is a production owner outside the plan's file map. The worklog records the
  reasoning, but the parent plan, which says changing the map requires an update before coding, was
  not amended.
- `internal/host/routes_test.go` import block now has `internal/conversation` inserted between
  `sync/atomic` and `testing`, splitting the standard-library group. Formatting passes, but the
  grouping is untidy.

## Requirement coverage summary

| Requirement | Status at 9fe8e769 |
| --- | --- |
| `WithEnvironment` literal / direct-input extraction and graph/catalog metadata | Done for direct body usage; incomplete for helpers and function values (finding 1) |
| Pure preparation rejects missing/empty/undeclared/unconfigured | Done and tested; depends on the incomplete inventory |
| Inheritance and captured session placement | Done and tested (create, turn, close) |
| Conversation workflow input and host launch passing the selection | Done; stale launchers (finding 3); local named environments (finding 4) |
| `Route.Environment` is not the conversation's placement authority | Done by rejection; replaced remote paths remain (finding 4) |
| Generated artifacts via generators | Incomplete (finding 2) |
| No live C1 claim | Respected |

Outcome: **material findings remain**
