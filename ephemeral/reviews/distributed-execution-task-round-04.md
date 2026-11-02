# Adversarial review: distributed-execution first-slice task definition, round 04

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one. Rounds 01–03 are unchanged.

## Target

`ephemeral/plans/distributed-execution-slice.md` at HEAD `7372cb003bb60410ad7d809d03f5239828935a4d`
("docs: bound distributed checkpoint and placement proof"), sha256
`10c35faeeb13aaed8d7b293d48e7332e92e1bec62196f7cfb93ac02e7935e056`. Working tree clean; no edits
during this round. Production source unchanged since `a3ee508b` (verified again).

Caller narrowing: none ignored.

## Evidence inspected

Same authoritative inputs as rounds 01–03. Diffed the plan `58200eee..HEAD` to isolate this
round's amendments: the proposed checkpoint boundary (manual workers, legacy routing left in
place and rejected when mixed into the new composition, conversation/orchestrator migration
deferred), R4 rewritten accordingly, R6 request addressing, R10/D5b/D10 placement legibility, the
new Participant-bootstrap and Observation/display cells, the ordered-array overlay rule, the
database-address allowance in the attestation rule, and the #664 hold release. Read the updated
worklog.

Code re-inspected for the amended text: `internal/host/routes.go:87-103` (`placement()`),
`internal/observation/registry.go:209-213` (`decorate`), `internal/observation/rows.go` (`RunRow`,
`SessionRow`, `CommandRow`), `internal/observation/store.go:380,553` (row construction from
lifecycle records), `events.go:105-112,177-185` (`SessionCreated`, `CommandStarted`),
`web/src/lib/run/{RunHeader,RunTranscript,SessionTimeline}.svelte`, `web/src/lib/workspace/projectHome.ts`,
`plugins/messaging-river/main.go` (no registration, identity, lease or liveness concept exists),
`registry.go:343-385` (`Connect`), `internal/runner/runner.go:30-55`, `internal/effects/messaging.go:30-37`.

Recounted the three changed budget rows under the plan's rules: Transport 1,343 (no
`environment_messaging.go`), Participant bootstrap 910, Observation/display 2,044 (all nine
listed files exist). All match the table. Declaration 3,262, Native 2,606, Context 2,729 are
unchanged and were verified in earlier rounds.

### Disposition of round-03 findings

- F1 conversation placement vs "sole authority": addressed by scope decision. The checkpoint
  leaves `EnvironmentRouting`, `environment_messaging.go`, `Route.Environment` and the local
  orchestrator unchanged for out-of-slice compositions, rejects them when mixed into the new
  River composition before effects (R4, D5a), and lists conversation/orchestrator migration under
  Exclusions as still required before the architecture gate. "Sole placement authority" is now
  scoped to the new composition. Coherent, and explicitly flagged as pending Tyler's approval.
- F2 placement display: addressed. R10, D5b, D10 and an Observation/display cell add per-task
  placement to existing rows and the run page, with pixel inspection required.
- F3 request envelope: addressed. R6 requires each new-work request to name its logical
  environment and directs the `MessageRequest` comment revision; the amendment list includes it.
- F4 overlay supply: addressed (`--task-route @file` takes one Route or an ordered array; one
  overlay rule).
- F5 D3 wording: addressed (listener-level phrasing).

Verified and not findings: the lifecycle-to-row reducer is in `internal/observation/store.go`
(mapped); `SessionCreated`/`CommandStarted` are emitted from `session.go`/`command.go` (Context
cell) and defined in `events.go` (Observation cell), so the per-task placement path has owners end
to end. The database-address allowance correctly resolves the round-01 ambiguity about B
connecting to Postgres on A's host. Leaving legacy code unmodified is not a shim under
`AGENTS.md`; the rejection rule prevents it from supplying operations in the specimen.

## Findings

### F1. Issue — "duplicate active registration is rejected" is undefined against the mandatory worker-restart specimen

**Cited instructions.** R3: "Initially one active worker per logical environment, with duplicate
active registration rejected." D3: "Duplicate active registration is rejected … Exercise the
actual database implementation." R7/D7: restart the task participant and recover the retained
result under the same identity. Exclusions: no "timing promises"; design inputs forbid a
cancellation/release ledger.

**The gap.** Nothing defines when a registration stops being *active*. Today the River plugin has
no registration at all (`plugins/messaging-river/main.go`: no identity, lease, heartbeat or
deregistration; `serve` is an anonymous queue consumer), so the implementer invents the rule. The
two obvious choices each collide with a requirement: (a) a registration is active until the
worker deregisters — then the D7 worker, killed after native completion, cannot re-register on
restart because its own stale row is a duplicate, and D7 fails or needs manual cleanup; (b) a
heartbeat/lease expiry — a timing mechanism the plan's anti-goals disfavour, and it still leaves
a window where a restarted worker is rejected. The plan's R3 text "existing sessions retain their
owner after rebinding; new independent work uses the current binding" presumes the owner
identity is stable across restarts, which only works if the *same configured worker identity*
may replace its own registration.

**Needed.** State the rule: registration is keyed by the plugin-configured worker identity; a
worker re-registering under its own identity supersedes its prior registration (and inherits its
retained sessions), while a *different* identity for an already-registered logical name is
rejected; no liveness timer is required for this slice. Add that case to D3 ("the restarted D7
worker re-registers under the same identity; a second distinct worker is rejected").

### F2. Issue — R10 contradicts itself for the new composition, and the file that produces the offending label was dropped from the map this round

**Cited instruction.** R10: "Mixed placement must not be labelled as one local/run-wide
destination" and, two sentences later, "Existing out-of-slice run-wide display is not removed."

**What the code does for a new-composition run.** `internal/host/routes.go:87-103` `placement()`:
`tasks = TaskRoute.Label` (empty for a River route), `TaskRoute.Environment` is empty by R4's
rejection rule, so `tasks = workflow`, and `workflow` defaults to `"local"`. `internal/observation/registry.go:209-213`
`decorate` writes that into `RunRow.TaskEnvironment`; `web/src/lib/run/RunHeader.svelte:23` renders
it as the run's identity badge and `RunTranscript.svelte:87` renders "where: local". So the D6/D10
run — A's command plus B's session — is labelled **local** at the top of the page, which is
exactly what R10's first sentence forbids, while R10's last sentence tells the implementer not to
touch the run-wide display. The round-03 Provision row listed `internal/host/routes.go`; the new
Participant-bootstrap row does not, so the source of the label is now outside every cell.

**Needed.** Decide and state one of: (a) for runs on the new composition, `decorate`
(mapped) replaces the run-wide label with the mixed summary ("A: env-a · B: env-b" or "mixed") and
`placement()` is unchanged; or (b) `placement()` is taught the new composition and
`internal/host/routes.go` returns to the map. Either way, rephrase R10 so "not removed" applies to
*out-of-slice* runs only, not to the specimen's own page.

### F3. Nitpick — worker identity in the event path is implied, not stated

R10 requires "admitted worker/machine identity" per command/session. The amendment list covers
request-side logical/retained-owner addressing and "stable source sequence/identity" for records,
but does not say that the `accepted`/`completed` events carry the admitting worker's identity and
that A records it into the lifecycle record (`SessionCreated`/`CommandStarted` or a new fact).
`MessageEvent` has no such field today (`backend/messaging.go:55-63`). One sentence closes it.

### F4. Nitpick — stage-2 list formatting

The second bullet's continuation line "ordinary workflow and uses real…" lost its indentation in
this revision; harmless, but the file is the approval record.

## Outcome

`material findings remain`
