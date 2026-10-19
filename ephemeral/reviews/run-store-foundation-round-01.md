# Adversarial review: run store foundation, round 01

Date: 2026-10-18. Reviewer: Claude (adversarial-review skill, performed directly, read-only apart from this file).

## Review target

- Worktree `/Users/tyler/.codex/worktrees/run-store-implementation-plan/gimbal`, branch `codex/run-store-foundation`, four commits over `main` (c5c9b142): a5c4bfb2 (directory-backed run store), 78e02672 (workflow identities), 478d46b4 (intents and executor admissions), df87da7e (observations and terminal cleanup through the store). 143 files changed, +4222/-681.
- Plus the uncommitted working tree at review time: `internal/effects/identity.go` (HTTP route mapping in `effects.Route`), `internal/observation/{owned,rows,store}.go` and `web/src/lib/skgo/runview/types.ts` (`CleanupStatus` becomes `polytype.Optional[string]`), `internal/skgo/skgo_gen.go` (regenerated), `run_recording_test.go`, and the untracked `run_store_substitution_test.go` (in-memory Store seam test).
- Authoritative sources: `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md`, `ephemeral/run-store-session.md`, `AGENTS.md`, `docs/definition-of-done.md`, `justfile` (`vet` recipe), `.github/workflows/release.yml`.
- Caller constraints (read-only, artifact path) are valid operating constraints. No caller narrowing of scope, files or verdict was present, so nothing was ignored.

## Evidence inspected

- Read in full: `runstore/store.go`, `runstore/directory/directory.go`, `run_recording.go`, `internal/runstate/{records,store}.go`, the engine diff (`run.go`, `scope.go`, `session.go`, `command.go`, `service.go`, `group.go`, `interview.go`, `supervise.go`, `remote_execution.go`, `event_persistence.go`, `artifact.go`, `registry.go`), the transport diff (`backend/*`, `internal/effects/*`, `internal/taskdaemon/*`, `agentexecutor/agent.go`, `internal/environments/operations.go`, `internal/conversation/remote.go`, plugins), the observation/runner/host diff (`internal/observation/{store,owned,terminated,follow}.go`, `internal/runner/{runner,task_cleanup}.go`, `internal/host/{host,routes}.go`), the worklog `ephemeral/worklog/run-store-foundation.md`, and the new tests in `runstore/directory`, the root package, `internal/runner`, `internal/taskdaemon`.
- Ran: `go build ./...` (passes; the five `plugins/*` "function main is undeclared" messages are `-buildmode=plugin` packages and predate this branch), `go vet` on the touched packages (fails, see finding 1), `go test` for `runstore/...`, `internal/runstate`, `internal/observation`, `internal/runner`, `internal/taskdaemon`, `internal/environments`, `internal/effects`, and the root package (all pass).
- Grep checks: no remaining `effects.RunID`/`effects.WithRunID`; old wire operation names gone from producers and adapters; `executor.NewCommandID` remains only in the standalone `files.go` path; the `orphanCall` plus `CleanupExited` double dispatch survives only on the legacy-format branch of `internal/runner/runner.go:276-283`; `runner.go:114` `id[:26]` is safe because `internal/host/routes.go:108` validates the RunID with `ulid.ParseStrict` before spawning; the web short-ID derivations (`ProjectWorkspace.svelte:92,239`, `projectHome.ts:69`, `Topbar.svelte:144`, `CancelGuard.svelte:29`) slice the trailing characters and accept bare ULIDs without new parsing.
- Seam assessment: the `runstore.Store`/`runstore.Run` contract matches the design (exclusive Create, exclusive Open with busy/incomplete/corrupt errors, slot-idempotent EnsureNode, digest-idempotent Append with `(node, kind)` uniqueness, committed-only Read, streamed PutBlob). No filesystem path leaks through the record interface; the engine uses the interface only (`run.records`), and `run_store_substitution_test.go` drives the same engine path with an in-memory Store through retained cleanup. Observation is a projection fed after facts (`eventResult` records Committed, agent events Buffered), the live registry is registered before `Store.Create` and removed on failure (`internal/observation/store.go:143-160`), and the `recording-format` marker plus `runstate.Recognized` select the new path in `follow.go`, `terminated.go` and the runner. I found no application path for new-format runs that writes authoritative state outside the store handle.

## Findings

### 1. Issue: `go vet ./...` fails on two corrupted struct tags in design-mandated terminal projection code

- `internal/observation/owned.go:25` has the tag `json:"time"\n "os"\n "path/filepath"` and `internal/observation/terminated.go:91` has `json:"time"\n "github.com/tylergannon/gimbal/internal/runstate"`: import lines were spliced into the struct tags.
- Reproduction: `go vet ./internal/observation/` reports `struct field tag ... not compatible with reflect.StructTag.Get: key:"value" pairs not separated by spaces` for both lines.
- Impact: `just vet` and the `go vet ./...` step in `.github/workflows/release.yml:170` fail, so the branch cannot pass the repository's own gate; `golangci-lint` (default `govet`) fails for the same reason. Runtime behavior is unaffected only by accident (`StructTag.Get("json")` happens to return the first pair). The files are the new owned terminal/cleanup projection writers that the design requires, so this is in-scope code, not a stray edit.

### 2. Issue: the orphan cleanup owner dispatches with an operation identity it never recorded

- `internal/runner/task_cleanup.go`, in `cleanupStopped`: the intent fact appended before dispatch carries `operation`, `target`, `episode_id`, `route` only, and the dispatch then mints a fresh identity inline: `effects.Cleanup(ctx, endpoint, id, target, backend.NewExecutionIdentity("control").RequestID)`. The request ID actually sent to the executor is never stored, and a second sweep (intent present, outcome absent) mints a different one.
- The in-process path does this correctly: `run.go` `retryCleanup` derives `actionIdentity(r.rootNode, "", attempt.ID, "cleanup")` and records it through `prepareAction` before any native call.
- Requirement: design, "store the resolved intent before dispatch"; session acceptance, "Repeat from CleanupExited after confirmed runner death; ... Assert recorded action/operation IDs" and "assert recorded action/session IDs and owner routes match actual executor admissions". `internal/runner/recording_test.go` asserts one dispatch and stable fact counts but never compares the executor-observed request ID with a stored fact, which is why this passed.
- Impact: after runner death, a lost cleanup acknowledgment cannot be correlated with the executor's admission from the store, which is the one situation the orphan owner exists for; the store is not the authority for this action's identity.

### 3. Issue: several explicit acceptance scenarios have no repeatable check and were not demonstrated

Per `AGENTS.md` proof is a run reported in the PR, and the request requires an independent assessment of completeness; neither the worklog nor the tests cover these session items, and I could not confirm them from code reading alone:

- Old-daemon-shaped dispatcher: `run_recording_test.go:210` covers only a workflow command over HTTP. The session requires the same fixture through a workflow turn, through the environment helper, and on the messaging route, and requires the helper to settle its receipt as not admitted (empty Result/Error, `Attempted=false`, `CleanupPending=false`) plus the "fail binding release, retain only that real obligation" variant. `grep ContractRejected --include='*_test.go'` finds only the two root tests; `internal/environments` tests never return 404/405. The handling appears to exist at `internal/environments/operations.go:159-240`, but the receipt settlement and the released binding were not shown.
- Output independence: no test removes the executor spool (`<run>/executor-spool`) after an outcome commit and reads complete output back through `OpenBlob` in another process; `grep executor-spool --include='*_test.go'` finds nothing. `commandOutcome` (`run_recording.go:167`) does store stdout/stderr blobs from `ended.StdoutFile/StderrFile`, which `command.go:229-254` and `service.go:74-81` populate, so the mechanism is present but unproved.
- Observation readback: the session asks to feed saved facts once into a fresh reducer and compare events/usage, including a failed command and an invalid-answer/correction sequence. The root tests read records back and check kinds/IDs, not a reducer comparison.

Impact: these are the exact seams the request names as the integration risks (identity through adapter/transport boundaries, competing writers replaced without changing observation). Without them, "completely implemented" is asserted, not shown.

### 4. Nitpick: an explicit orphan scope stop reports success when `runner.lock` is absent

- `internal/runner/task_cleanup.go`, in `cleanupStopped`: when `os.OpenFile(.../runner.lock)` returns `ErrNotExist` the function returns `settled=true, nil` for both the sweeper (`op == nil`) and an explicit operator `scope` operation routed from `runner.go:254`. The session text intends the settled answer for the sweeper's "ready non-runner entry"; for an explicit stop it silently reports success without dispatching or recording anything. Returning an error ("no runner for this run") for `op != nil` would be more honest.

### 5. Nitpick: review was performed against an uncommitted working tree

- The `CleanupStatus` change to `polytype.Optional[string]` (needed so `ReleasedProjection` distinguishes "never set" from empty), the `effects.Route` HTTP mapping, the regenerated `skgo_gen.go` and the in-memory Store test are uncommitted. They are part of what passes the tests reported above. Commit them before any further proof so the branch history and the proof agree ("Commit, and push, whenever something interesting has happened").

## Outcome

material findings remain
