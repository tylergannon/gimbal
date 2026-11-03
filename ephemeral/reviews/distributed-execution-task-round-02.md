# Adversarial review: distributed-execution first-slice task definition, round 02

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one. Round 01 is
`ephemeral/reviews/distributed-execution-task-round-01.md` and is not modified.

## Target

`ephemeral/plans/distributed-execution-slice.md`, working-tree version with
sha256 `5bc7dd0515f3ac7f0d724a84374bafa33ab3f84b2aae8fe18b9006b29f68f454` (mtime 20:54:44),
which differs from the committed copy in `35d7c060` ("docs: define first distributed execution
milestone"). The file was being edited during this review: an earlier read in the same window
showed a five-file Transport row at 950/1,800; the pinned version has the seven-file row at
1,440/2,300. Findings below refer to the pinned version. Consensus rounds should cite a hash.

Worktree HEAD is `35d7c060`; the plan's "Inspected baseline" still says `a3ee508b`. Verified
`git diff --stat a3ee508b..HEAD -- . ':!ephemeral'` is empty, so no production source changed.

Caller narrowing: none ignored. The launch prompt set the target, artifact path and read-only
constraint only.

## Evidence inspected

Same authoritative inputs as round 01, plus the newly cited
`/Users/tyler/src/gimbal-view/ephemeral/reviews/20261031-distributed-proposal-review-synthesis.md`
and the current `ephemeral/worklog/distributed-execution-slice.md`. Diffed the plan against
`HEAD` to isolate the round-01 amendments.

Code re-inspected for the amended requirements: `internal/environments/{operations,service}.go`
(callers of `EnvironmentRouting`, `AcquireEnvironment`, `Route.Environment`), consumers of that
package (`cmd/gimbal/desktop_environment.go`, `internal/conversation/*`, `internal/host/*`,
`web/runtime.go`), `backend/messagefiles/store.go` (shared `ListenEvents` implementation used by
`plugins/messaging-filesystem` and `plugins/messaging-http`), `internal/host/routes.go` and
`cmd/gimbal/workflows.go` (`Route.Environment` readers), `plugins/messaging-river/main.go`
`ListenEvents` error path, `internal/effects/messaging.go` `Await`, `internal/taskdaemon/persistence.go`
(`disposition.EventID`), `internal/generate/declarations.go`/`expr.go` (how `gimbal.With*`
decorators are followed statically), `service_bindings.go` (existing exported `WithRequirements`
pattern the proposed `gimbal.WithEnvironment` mirrors).

Recounted all five budgets under the plan's rules (direct children, exclude `*_test.go` and
`*_gen.go`): Declaration 5,569; Transport 1,440; Native dispatch 2,606; Context 2,729;
Provision/bootstrap 1,139. All match the pinned table.

### Disposition of round-01 findings

- F1 orchestration: addressed. R4 now removes `EnvironmentRouting`/`environment_messaging.go`,
  allows prestarted workers without an orchestrator, forbids a dummy orchestrator, and a
  Provision/bootstrap cell maps `backend/orchestration.go`, `localorchestrator`, `runner.go`,
  `invocation`, `message-worker`. D5a adds the observation.
- F2 restart publication: addressed. R7 assigns recovery publication to native dispatch, requires
  idempotent republication of the persisted completion identity without A resubmitting, and the
  semantics block gains `publish_owner_outcome_idempotently_even_without_redelivery()`.
  `disposition.EventID` in `internal/taskdaemon/persistence.go` supports it as claimed.
- F3 seam files/wall clock: addressed. `backend/messaging.go`, `backend/backend.go`,
  `messaging_registration.go` and the two other plugins are in Transport; R8 forbids filtering
  execution waits by A's timestamp and requires changing the listener contract.
- F4 working directory: addressed. R5 makes directories explicit owner-interpreted inputs and
  requires preserving the native path in replay intent; no daemon-wide override.
- F5 proof program: addressed. Stage 2 specifies an opt-in package integration test with real
  `message-worker` participants and forbids a standalone specimen or relabelled example.

Verified and not findings: proposed `gimbal.WithEnvironment(ctx, name)` matches the existing
exported `WithRequirements`/`WithYields` decorator pattern in `service_bindings.go`, maps to the
existing internal `effects.WithEnvironment`, and the extractor already follows `gimbal.With*`
calls through context objects (`internal/generate/declarations.go:36-66`), so static
enumeration is feasible as described. "A accepts … a completion whose acceptance record was
missed" already holds in `effects.Await` (`e.Kind == until || e.Kind == "completed"`).
D3 now exercises rebinding, closing a round-01 lesser note.

## Findings

### F1. Issue — the map "covers known required work" but two mandated changes break or require production files outside it

**Cited instruction.** Cells section: "File ownership below covers known required work; … Any
required production change outside this map is a spec-defect intake before coding."

**R4 deletion breaks an unmapped production package.** "Remove `EnvironmentRouting` and
`environment_messaging.go`" and "remove its route-returning methods" (local provisioner).
`internal/environments/operations.go:304-305` type-asserts `b.Connection.Orchestrator.(backend.EnvironmentRouting)`
and calls `Destinations`; `operations.go:369-373` asserts again and calls `ActiveEnvironments`;
`service.go:449,475` call `AcquireEnvironment(ctx, owner, route.Environment)`; `operations.go:139,302`
decorate with `effects.WithEnvironment(..., b.Target.Route.Environment)`. The package is
production: it is imported by `cmd/gimbal/desktop_environment.go`, `internal/conversation/{manager,remote,setup}.go`,
`internal/host/{environments,host}.go` and `web/runtime.go`. Deleting the interface fails
compilation there; none of these files is in any cell (`internal/environments/operations.go` +
`service.go` alone are 938 lines).

**R8 listener change requires an unmapped shared implementation.** "Change and document the
shared listener contract and adapt all implementations/callers affected." The filesystem and HTTP
plugins are now mapped, but both delegate to `backend/messagefiles.Store.ListenEvents`
(`backend/messagefiles/store.go:61-95`, `!e.Created.Before(since)`), imported at
`plugins/messaging-filesystem/main.go:22` and `plugins/messaging-http/main.go:55,65`. The HTTP
producer also parses `since` for its SSE endpoint (`main.go:280-291`). `backend/messagefiles/` is
not in the map.

**`Route.Environment` has no stated disposition.** R1 says workflow source selects placement; the
approval item says configuration "uses the existing Route envelope (`Name`, `Configuration`)".
Today `Route.Environment` is a run-wide placement set by `--task-route` and applied in
`internal/runner/runner.go:37,79` via `effects.WithEnvironment`; it is also read by
`internal/host/routes.go:93-94` (placement display), `cmd/gimbal/workflows.go:98` (flag help) and
`internal/environments/*`. Kept, it is a second placement authority competing with source
declarations (violating R1/R2's single inventory); removed, it adds more unmapped files. The plan
must say which.

**Impact.** The intake clause fires on the first compile of R4, contradicting the map's
completeness claim, and the Transport/Provision budgets omit code the requirements force to
change. Add `backend/messagefiles/store.go` to Transport, state the `internal/environments`
disposition (migrate to logical addressing, or delete the SSH-setup path the design input already
classifies as a bypass) and map it, and decide `Route.Environment`.

### F2. Issue — the execution wait's behaviour across a lost intermediary connection is unspecified, and today one failed poll converts a running remote turn into an unresolved failure

**Requirement gap.** R8 covers the observation subscription and forbids timestamp filtering;
Stage 3 says "the observer is independently reconnectable". Nothing states what A's *execution
wait* must do when its River/Postgres connection fails while B's native session is still
running. `docs/definition-of-done.md` ("Execution and observation") assigns bounded,
observable reconnection to adapters and forbids replacing a harness's authoritative success with
failure, but the plan does not locate that obligation in any cell contract (Transport
`ListenEvents` vs Native `effects.Await`/`taskdaemon.Harness`).

**Current behaviour (verifiable).** `plugins/messaging-river/main.go` `ListenEvents` returns
`storeError(err)` on the first failed `pool.Query` (poll loop, every 50 ms). `internal/effects/messaging.go:69-110`
`Await` treats any non-sentinel return as `DeliveryUncertain{"event delivery unavailable"}`;
`session.turn` (`session.go`) then records `TurnEnded{Unresolved: true}` and the workflow's
`Generate` returns an error. No retry or reconnect exists in `effects.Await`, `MessageCall` or
the River listener (grep for reconnect/retry: none). pgxpool heals the pool for the *next* query,
but the in-flight listener has already exited.

**Why it matters for this slice.** Across two computers, A→Postgres is A's only path to the
outcome. A transient TCP reset, a Compose restart, or a `pg_hba`/idle timeout during a
multi-minute luna session produces a false "unresolved" for D6/D7 while B completes the work;
R7's republication then lands in a database A has stopped listening to. A live-run failure of
this kind would be unclassifiable against the plan: neither a defect nor expected behaviour is
defined. State either (a) the execution wait reconnects with a bounded, observable budget and
resumes by record identity (consistent with R8's replay-by-identity), naming the owning cell, or
(b) a listener failure is an accepted unresolved outcome for this slice and D6/D7 live runs
must report it as such.

### F3. Nitpick — stale baseline label

"Inspected baseline: `a3ee508b…`" while HEAD is `35d7c060`. The only difference is the plan's
own commit; say so or update the label, since D-claims and budgets are "bound to the exact
reviewed source".

### F4. Nitpick — the Declaration tripwire is diluted by unrelated generator code

`internal/generate/*.go` contributes 4,832 of the cell's 5,569 lines, most of it inspector,
mermaid, page and cache code (`inspect.go` 609, `mermaid.go` 367, `cache_source.go` 346,
`source.go` 440, `read.go` 331) that environment extraction will not touch. A 631-line
headroom on a 5.5k base is a weak signal; list the extractor files that will change
(`declarations.go`, `expr.go`, `stmt.go`, `graph.go`, `definition.go`) with their own baseline.

### F5. Nitpick — the orchestrator's new transport capability is a public contract change not named in the approval list

R4/D5a require the orchestrator to "receive the selected transport's narrow worker
bootstrap/readiness capability" and for `AcquireEnvironment`'s ready result to "correspond to
registration in that namespace". That changes `OrchestratorDefinition.Construct`'s signature or
`backend.Dependencies`/`MessagingDefinition` (new exported fields or types). The approval list
says only "any materially new public contract must return for approval"; name this one
explicitly so it is approved with `gimbal.WithEnvironment` rather than discovered mid-implementation.

## Outcome

`material findings remain`
