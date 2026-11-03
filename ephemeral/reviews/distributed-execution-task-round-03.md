# Adversarial review: distributed-execution first-slice task definition, round 03

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one. Rounds 01 and 02 are unchanged.

## Target

`ephemeral/plans/distributed-execution-slice.md` at HEAD `58200eeea7a34c7c9de425b05ac7e3a27cbb040b`
("docs: clarify distributed ownership and recovery requirements"), sha256
`b9c5d7a7e2e7b2e834d87fcc2335752a6d93ec4b527478cdbf983ac172ae3d46`. Working tree clean; no
edits occurred during this round. Production source is unchanged since `a3ee508b`
(`git diff --stat a3ee508b..HEAD -- . ':!ephemeral'` is empty), matching the plan's new
"Inspected production baseline" wording.

Caller narrowing: none ignored. The launch prompt set the target, checkpoint, artifact path and
read-only constraint only.

## Evidence inspected

Same authoritative inputs as rounds 01–02. Diffed the plan `35d7c060..HEAD` to isolate this
round's amendments (R4 ownership paragraph, R5 directories, R7 recovery publication, R8
reconnect budget, remapped cells, D3/D5a, stage 2–3 test shape, explicit public-contract
amendment list, #664 hold update) and read the updated worklog.

Code re-inspected for the amended text: every remaining production reader or constructor of
`Route.Environment` (`registry.go:365-367`, `internal/runner/runner.go:37,49,79`,
`internal/host/routes.go:93-94`, `internal/environments/{service,operations}.go`), the
conversation launch path (`internal/conversation/manager.go:255-290`, `internal/host/host.go:150-170`,
`internal/environments/service.go:330-400` `Resolve`/`routeFor`, `internal/host/environments.go`),
run-level placement projection and display (`internal/host/routes.go:87-103`,
`internal/observation/registry.go:28,210`, `internal/observation/rows.go:39-40`,
`web/src/lib/run/RunHeader.svelte:23`, `RunTranscript.svelte:36,87`, `web/src/lib/workspace/projectHome.ts:79`),
the request envelope and its destination rule (`backend/messaging.go:8-25`), route reading
(`cmd/gimbal/workflows.go:188` `readRoute`) and the absence of any overlay mechanism in production
code, and the `internal/environments` package file list.

Recounted the three changed budget rows under the plan's rules: Declaration 3,262 (listed
generator subset), Transport 1,535 (adds `backend/messagefiles/store.go`), Provision/bootstrap
4,316 (adds `cmd/gimbal/workflows.go`, `internal/host/routes.go`, `internal/environments/*.go`).
All match the table. Native 2,606 and Context 2,729 are unchanged and were verified in round 02.

### Disposition of round-02 findings

- F1 unmapped callers / `Route.Environment`: addressed. `backend/messagefiles/store.go`,
  `internal/environments/*.go`, `internal/host/routes.go`, `cmd/gimbal/workflows.go` are mapped;
  R4 removes `Route.Environment` and names the migration of `internal/environments` callers.
- F2 execution-wait reconnect: addressed. R8 assigns bounded, identity-preserving reconnect to the
  River transport (30 s default, plugin-configurable, caller deadline honoured, explicit failure
  for permanent errors) and D3 exercises it against the real database.
- F3 baseline label: addressed.
- F4 Declaration dilution: addressed (explicit generator subset).
- F5 unnamed public contract: addressed (explicit amendment list for approval).

## Findings

### F1. Issue — "workflow declarations are the sole task placement authority" collides with the conversation feature, whose placement is chosen at runtime in the UI; the plan neither migrates nor excludes it

**Cited instruction.** R4: "Remove run-wide `Route.Environment`: workflow declarations are the
sole task placement authority … do not retain a compatibility field or silently override workflow
placement." R2: "No implicit local fallback." Approval item: "All environment names must be
statically enumerable."

**Current production path.** `internal/conversation/manager.go:266-281`: when the user picks a
non-local environment (`in.Environment`, from `web/src/routes/conversation.remote.go:56`), the
manager calls `service.Resolve(ctx, projectID, in.Environment)`; `internal/environments/service.go:330-342`
returns `Target{Route: routeFor(m)}` where `routeFor` (`service.go:384-390`) returns the configured
`*m.Route` for a `KindRoute` machine — including its `Environment` placement. `internal/host/host.go:160-166`
then starts the registered `conversation` workflow with `invocation.Request{TaskRoute: route}`, and
`internal/runner/runner.go:37,79` applies `effects.WithEnvironment(…, TaskRoute.Environment)` run-wide.

**The conflict.** The `conversation` workflow's Go source cannot statically declare the machine a
person selects per conversation; its placement is inherently a runtime input. Under R4 as written,
deleting `Route.Environment` leaves exactly two outcomes for a route-kind machine: placement is lost
and the conversation runs locally (forbidden by R2), or the feature fails. The R4 paragraph
migrates `internal/environments` callers of the deleted *routing interface*, but says nothing about
the conversation's runtime-selected placement, and `internal/conversation/*`, `internal/host/{host,environments}.go`
and `web/src/routes/conversation.remote.go` are outside the map. The current `KindRoute` placement
is the one product path that already places tasks on another machine through messaging.

**Needed.** A recorded decision, not implementer discretion: either (a) this slice excludes
conversation placement and route-kind machines keep working through an explicitly retained,
named mechanism until a later milestone (then "sole authority" and "no compatibility field" must
be qualified), or (b) runtime-supplied placement is admitted as a declared workflow *input* that
preflight checks against the same namespace, with the conversation workflow declaring it so. Map
the owning files either way.

### F2. Issue — removing `Route.Environment` deletes the only placement display, and the plan neither replaces it nor excludes UI legibility, contrary to a cited design input

**Cited input.** `20261031-distributed-proposal-review-synthesis.md` §5 ("accept an explicit
acceptance addition"): a person must be able to identify which environment a step/session used;
current placement is two run-level fields that cannot explain mixed placement within a run.

**Current path being removed.** `internal/host/routes.go:87-103` `placement()` reads
`TaskRoute.Environment` → `observation.Registry.Placement` (`registry.go:28,210`) →
`RunRow.TaskEnvironment` (`rows.go:39-40`) → rendered at `web/src/lib/run/RunHeader.svelte:23`,
`RunTranscript.svelte:36,87` ("where") and `projectHome.ts:79`. R4 says to "update its host/CLI
display/help readers" but not to what. After the change, a D6 run that executed its session on B
shows "local" or nothing on the run page, which is a regression from today, and D6's "show actual
machine and native-session identities" is satisfied only in chat/log output.

**Needed.** Either adopt the synthesis's acceptance addition in a bounded form (the recorded
per-session/per-command placement is visible on the run page; no layout redesign) and map the
owning projection/UI files, or list "run-page placement legibility" under Exclusions explicitly so
the regression is a known, approved consequence rather than a surprise at validation.

### F3. Issue — the request envelope must name its logical environment, but that public contract amendment is missing from the approval list and the existing contract comment forbids it

**Why it is unavoidable.** With `EnvironmentRouting` deleted (R4) and placement captured at session
creation (R1), the River plugin's `PublishRequest` must learn a new session's logical destination
from the request. `backend.MessageRequest` (`backend/messaging.go:14-25`) has no such field;
`effects.Environment(ctx)` is an internal context key the plugin cannot read; `Owner`, `Session`
and `Process` carry no placement. So a new envelope field (or equivalent) is required.

**Why it needs saying.** The approval list enumerates `OrchestratorDefinition.Construct`,
`backend.Dependencies`, `MessagingDefinition` schema/namespace/addressing, deletions, record
sequence/identity and byte delivery — not the request envelope. Meanwhile `backend/messaging.go:11-13`
states "Destinations belong to plugin configuration, never the action vocabulary", which an
implementer will read as prohibiting the field. The plan should add the amendment ("a request
names its logical environment; physical destination remains plugin configuration") and direct the
comment's revision, so the one change every other requirement depends on is approved rather than
discovered as a contradiction mid-implementation.

### F4. Nitpick — overlay supply is unspecified

R3/D2 define overlay merge semantics, but no production code composes overlays today:
`readRoute` (`cmd/gimbal/workflows.go:188`) reads one JSON value or `@file`. State how overlays are
supplied (repeated `--task-route`, an ordered file list, or deferred to implementation with the CLI
shape returning for approval), since `registry.go` already reserves `task-route` among role-flag
names and `internal/generate/command.go` lists it as taken.

### F5. Nitpick — D3's reconnect clause spans two cells

D3 is labelled "Cell-local / transport" but "resumes the same execution wait" is observable only
through `effects.Await` (Native dispatch). Either phrase D3 in transport terms (the listener
delivers the original completion after reconnect) or label it integration.

## Outcome

`material findings remain`
