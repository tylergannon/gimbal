# Adversarial review: promises-ledger plan, round 05

> Recovered from the session log. The original `ephemeral/reviews/202610130900-promises-ledger-plan-round-05.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Issue — a failed directory sync is still acknowledged as a durable batch commit

The plan reloads `ledger.json` after any save error and, when the reloaded
cursor equals the attempted batch end, declares the batch saved and clears its
error (`plan.md:234-243`). That proves only that the rename is presently
visible. The reused writer explicitly acknowledges success only after syncing
both the temporary file and the parent directory
(`internal/transcript/durable.go:95-125`); an error from the final directory
`Sync` means the renamed directory entry has not been established as durable.
Reading the new file before a crash does not repair or acknowledge that missing
durability barrier.

A concrete failure sequence is: rename installs the new ledger; parent
directory sync fails; reload sees `Cursors[R] == end`; the plan clears the
error and retains no pending durability work; power loss or a filesystem crash
loses the unsynced rename. On restart the old cursor and entries return, so the
recording is reprocessed. That violates the stated demo guarantee that restart
leaves the ledger unchanged and processes no recording again
(`plan.md:38-47`), as well as the plan's claim that the atomic rename is its
whole crash-idempotency story (`plan.md:117-122`).

The post-rename branch needs to retain the already-computed ledger and retry
its durable write/acknowledgment without rerunning the model; it cannot call a
failed sync successful merely because the new bytes are currently readable.
The test plan still exercises only a pre-rename failure and explicitly declines
a post-rename fault-injection test (`plan.md:241-243,437-440`), so it would not
demonstrate this boundary.

### 2. Issue — shutdown releases the project writer lock without joining the promises worker

The plan starts the promises worker as a raw project goroutine and says only
that it exits when `p.ctx` is cancelled (`plan.md:189-191,256-260,477`). The
current host's shutdown wait covers `activeRuns`, not background project
goroutines; `Owner.Close` then closes each project's `owner.lock`
(`internal/host/host.go:177-183`). The control-server shutdown calls that close
as soon as its listener ends (`web/control.go:216-223`). Nothing in the plan
adds the new worker to a join that must complete before the lock is released.

Cancellation is not an instantaneous join. A Claude turn may spend up to the
adapter's cancellation grace winding down (`claude/claude.go:320-328`), and a
worker already in the lock/apply/save phase does not consult the context at all
(`plan.md:212-216`). A replacement Gimbal process can therefore acquire
`owner.lock` and start its own promises worker while the old worker is still
saving the same `ledger.json`. Atomic replacement prevents malformed JSON but
does not prevent the two writers from overwriting one another's entries or
cursors.

Impact: orderly restart can violate the store's declared “only writer”
invariant (`plan.md:67`) and lose a batch even though both processes individually
follow copy-save-publish. The worker needs an owned completion/join path, and
project ownership must not be released until it has stopped; a shutdown test
should block a turn or save, cancel the instance, and prove a second owner
cannot begin ledger work until the first worker has exited.

## Outcome

**material findings remain**
