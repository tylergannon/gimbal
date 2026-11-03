# Adversarial review: distributed-execution first-slice task definition, round 05

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one. Rounds 01–04 are unchanged.

## Target

`ephemeral/plans/distributed-execution-slice.md` at HEAD `c673601f6b06fa1bccfe61a2232e4644c4d13a5e`
("docs: specify worker registration and placement projection"), sha256
`1a2b7731c6c457af14183d09d6173c157026189f326608ede8fb883977e7c68f`. Working tree clean; no edits
during this round. Production source unchanged since `a3ee508b` (verified again).

Caller narrowing: none ignored.

## Evidence inspected

Same authoritative inputs as rounds 01–04. Diffed the plan `7372cb00..HEAD`: the amendments are
the R3 worker-identity/registration-authority paragraph, the D3 restart/duplicate clauses, the R10
decoration-override and event-identity paragraph, and the stage-2 indentation fix. Re-read the
whole plan end to end for coherence of the accumulated edits, and the updated worklog.

Code re-inspected for the amended text: `plugins/messaging-river/main.go:100-125` (existing
dedicated-connection `pg_advisory_lock` pattern in `initialize`; no registration, identity or
liveness concept elsewhere), `cmd/gimbal/messaging.go` (message-worker: no local state-directory
lease), `cmd/gimbal-worker/worker.go:48-56` (the HTTP worker's `executor.Lock(stateDir/taskdaemon)`
lease), `internal/taskdaemon/daemon.go:New` and `persistence.go:restore` (state directory is the
identity carrier for retained sessions/dispositions), `internal/observation/registry.go:209-213`
(`decorate`, now the designated override point), `internal/observation/store.go:380,553` (row
construction), `internal/effects/messaging.go` `Await` (late `completed` without `accepted` already
accepted).

Budgets: no row changed this round; all six were verified in rounds 02–04.

### Disposition of round-04 findings

- F1 active registration vs restart: addressed. Identity is plugin-configured, stable across
  restarts and bound to the state directory; one serving instance holds database-owned exclusive
  authority (session advisory lock); a concurrent instance, even with the same identity, is
  rejected while that authority is held; after release the same identity re-registers and recovers
  retained sessions without row cleanup; unresolved authority waits within the startup budget or
  fails explicitly; no heartbeat/lease scheme; distinct identity cannot claim a bound name without
  rebind. D3 exercises these cases. The River plugin already uses a dedicated connection for an
  advisory lock, so the mechanism is realistic with pgxpool.
- F2 R10 contradiction / unmapped label source: addressed. `registry.decorate` (mapped) replaces
  the host-derived `local` summary for new-composition runs from retained per-task placement; the
  legacy summary applies only to out-of-slice runs; `internal/host/routes.go` need not change.
- F3 worker identity in the event path: addressed (accepted/completed facts carry worker identity
  and logical environment; A records them as lifecycle facts for replay).
- F4 formatting: fixed.

Verified and not findings: the checkpoint boundary, rejection rule, R6 request addressing, R8
reconnect, R10 override and the cell map are mutually consistent after re-reading the full text;
the Observation cell owns both the reducer and the decoration point; nothing in the amendments
reintroduces network denial, a scheduler, workspace management, a retention policy or a new
exported workflow function beyond `WithEnvironment`.

## Findings

### F1. Issue — the serving worker's behaviour when it loses its own registration authority is unspecified, and nothing local prevents a second admitter on the same state directory

**Cited instructions.** R3: "One serving instance holds database-owned exclusive registration
authority (a session advisory lock is sufficient), and only that instance may admit its addressed
work. A concurrent second instance … cannot take over while the first still owns that authority."
R3 also binds the worker identity "to its retained native state directory." R8 contemplates
transient database/connection failures as normal.

**The gap.** A session advisory lock lives exactly as long as its database session. The plan
specifies A's listener reconnect (R8) but says nothing about the worker whose authority
connection drops while it is serving: the lock is released server-side, yet the worker's River
client (on its own pool) keeps consuming the logical name's queue and admitting work without
authority. The plan's guarantee that a second instance "cannot take over while the first still
owns that authority" silently stops holding after an ordinary blip, and the first instance does not
know it. If an operator (or the D7 procedure, or a supervisor restart) then starts a second
instance with the same identity, it acquires the lock legitimately, and two processes now admit
from the same queue and write the same executor state directory. Today nothing stops that
locally: `cmd/gimbal/messaging.go` (message-worker) takes no state-directory lease, unlike
`cmd/gimbal-worker/worker.go:48-56`, which refuses to start when `executor.Lock(stateDir/taskdaemon)`
is held ("another worker owns %s"). Two daemons over one `taskdaemon/` state root would interleave
`writeState` for dispositions, tasks and sessions — exactly the identity corruption D4/D7 forbid.

**Needed (both in mapped files).** (a) R3: when the authority session is lost, the worker stops
admitting new addressed work, attempts to re-acquire within the same plugin-owned reconnect budget
as R8, and on exhaustion exits explicitly (or publishes unresolved for anything admitted
in-between); native work it already owns continues and its outcomes are still published, since
publication needs no authority. (b) Participant bootstrap: `message-worker` takes the same local
state-directory lease `gimbal-worker` takes, so the identity's state directory can never have two
live daemons regardless of database state. Add the lost-authority case to D3.

### F2. Nitpick — "fails at use" for an unowned logical name should say definite rejection, not a wait

D5a: "missing/failed registration fails at use without another route." With a queue, a request
to a configured-but-unserved name simply sits until the caller's deadline and surfaces as
`DeliveryUncertain`, which the DoD treats as an honest unknown, not a failure. Since the plugin
holds registration authority in the database, A can check for an active owner at publish time and
return a definite `ContractRejected`-class error. Say which behaviour D5a means.

### F3. Nitpick — abrupt-loss release of the authority depends on Postgres connection settings

"After the database releases the departed instance's authority" is prompt for a killed process
(the OS closes the socket) but, after a hard power loss or network partition of B, depends on the
server noticing the dead session (`tcp_keepalives_*`, `idle_session_timeout`), which on default
Linux images can exceed two hours. The plan's "wait within the startup budget or fail explicitly"
is honest; add one line to the live-checkpoint prerequisites that the Compose Postgres sets a
short keepalive, or that operator intervention after abrupt loss is an accepted limitation.

## Outcome

`material findings remain`
