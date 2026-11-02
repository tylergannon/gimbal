# Adversarial review: distributed-execution first-slice task definition, round 06

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one. Rounds 01–05 are unchanged.

## Target

`ephemeral/plans/distributed-execution-slice.md` at HEAD `b46be5ef369db18329a675327bff0cff46ea14ea`
("docs: define behavior on lost worker authority"), sha256
`33900a3ee14e8913d0ae20e4f11f644b1fe96a688dcee3b4859953c55e0267eb`. Working tree clean; no edits
during this round. Production source unchanged since `a3ee508b` (verified again).

Caller narrowing: none ignored.

## Evidence inspected

Same authoritative inputs as rounds 01–05. Diffed the plan `c673601f..HEAD`: the amendments are
the R3 lost-authority paragraph (admission stops, authority-validated commits, reacquire within
the R8 budget, exit on exhaustion, admitted native owners preserved, message-worker takes the
executor state-directory lock), the matching D3 and D5a clauses (definite no-owner rejection vs
uncertainty; no second daemon over one state root), and the live-work note bounding the restart
specimen to process loss with retained state. Re-read the full plan against the round-05 text to
confirm the amendments fit the surrounding requirements and cells, and read the updated worklog.

Code re-inspected: `plugins/messaging-river/main.go:147` (`MaxAttempts: 1` on every request
insert), `:244-257` (`actionWorker.Work` returns `river.JobCancel` on handler error), `:100-125`
(dedicated-connection advisory lock pattern), `cmd/gimbal/messaging.go` (no lease today),
`cmd/gimbal-worker/worker.go:48-56` and `internal/executor/files.go:84` (`executor.Lock`, the lease
R3 now names), `internal/effects/messaging.go` `Submit`/`Await` (definite `ContractRejected` vs
`DeliveryUncertain` already distinguished, matching D5a's wording).

Budgets: no row changed; all six verified in rounds 02–04. The lease addition lands in
`cmd/gimbal/messaging.go` (Participant bootstrap, mapped) and the authority logic in the River
plugin (Transport, mapped).

### Disposition of round-05 findings

- F1 lost authority / local lease: addressed. Admissions stop on loss or uncertainty; commits
  validate authority so a healthy data pool cannot admit alone; reacquisition within the R8
  budget restores admission only under confirmed ownership; exhaustion exits with an actionable
  error; already-admitted native owners are not cancelled and their results are published on
  reconnect or by the next same-identity restart; message-worker holds the executor
  state-directory lock. D3 and D5a exercise both halves.
- F2 "fails at use": addressed. D5a now says an authoritative no-owner result at submission is a
  definite rejection before publication, and inability to determine ownership is explicit
  uncertainty — the same split `effects.Submit` already makes between `ContractRejected` and
  `DeliveryUncertain`.
- F3 abrupt-loss release: addressed. The live-work note scopes the required restart to process
  loss with retained state, names PostgreSQL's own failure detection as the release mechanism after
  abrupt loss, treats waiting or operator closure of the stale session as an accepted limit, and
  forbids a second worker from assuming authority.

No material findings remain. The requirements, scope boundary, cell map, semantics, DoD, staged
plan, exclusions and approval list are mutually consistent at this revision, every requirement
has an owning cell and an observable claim, and each code-level objection raised in rounds 01–05
now has a stated, implementable answer in mapped files. What follows are implementation pitfalls
worth one sentence each in the plan, not defects in it.

## Findings

### F1. Nitpick — "stops consuming" under River's single-attempt policy must pause, not error

Every request is inserted with `MaxAttempts: 1` (`plugins/messaging-river/main.go:147`) and the
worker converts a handler error into `river.JobCancel` (`:255`). If lost authority is implemented
by refusing jobs from `Work`, a request that arrives during the loss window is cancelled
permanently and never admitted after reacquisition, so A waits to its deadline and D3's
"reacquisition restores admission" would fail for that request. State that the worker pauses or
stops its queue consumption (or snoozes without spending an attempt) while authority is
unresolved, so pending requests remain deliverable.

### F2. Nitpick — do the ownership write on the authority session to make "commits validate that authority" race-free

Validating a session advisory lock from a *different* connection is a check-then-commit: the lock
can be released between the `pg_locks` check and the ownership write. Performing the registration
and ownership writes on the same dedicated connection that holds the lock makes validity implicit
(a dead session fails the write). One sentence allowing either design, or preferring the latter,
removes the ambiguity.

### F3. Nitpick — D3 now carries seven distinct claims

D3's cell-local row covers duplicate registration, lost authority, reacquisition, restart
re-registration, stable identities and redelivery, rebinding, and listener reconnect. Splitting it
(for example D3 registration/authority, D3a records/redelivery, D3b rebind/reconnect) would make
the validator's pass/fail per claim legible without changing scope.

## Outcome

`only nitpicks remain`
