# Adversarial review: promises-ledger plan, round 07

> Recovered from the session log. The original `ephemeral/reviews/202610131000-promises-ledger-plan-round-07.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Issue — `Owner.Close` can wait forever because it does not cancel the worker it joins

The plan starts the promises worker on `p.ctx`, says that it exits only when
that context is canceled, and changes `Owner.Close` to wait on
`p.promisesDone` before releasing the project lock
(`ephemeral/promises-ledger/plan.md:188-198`, `:485`). Its justification is
that the owner context is already canceled when `Close` runs through the web
instance shutdown callback (`web/control.go:216-223`). That is true for that
one caller, but it is not an invariant of `Owner.Close`: the method itself
does not cancel the owner or project context before closing project resources
(`internal/host/host.go:177-183`).

Current callers deliberately close an owner whose context is still live. For
example, `TestCreateProjectRejectsMissingDirectory` creates the owner from
`t.Context()`, admits a project, and defers `owner.Close()`
(`web/src/lib/workspace.remote_test.go:13-37`). The host route tests do the
same with both `context.WithCancel` whose cancellation is deferred after
`owner.Close` and with `t.Context()`
(`internal/host/routes_test.go:115-123`, `:129-138`). After the proposed worker
is started for every admitted project, these closes block on a worker whose
context no caller has canceled; deferred cancellation cannot run because the
deferred close has not returned.

Impact: existing package tests hang, and any direct host owner lifecycle using
`Close` as the shutdown operation can deadlock while retaining `owner.lock`.
The plan needs the close path itself to signal this project-owned worker to
stop before joining it; relying on the web instance's external cancellation
ordering does not make `Owner.Close` safe.

## Outcome

**material findings remain**
