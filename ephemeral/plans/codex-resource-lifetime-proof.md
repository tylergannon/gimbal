# Our resource ownership: bounded real-workflow proof still required

This draft implementation fixes local owner retention; it does not establish
the cause of the reported roughly 30 app-server processes or prove that native
tool descendants unload. Parent coordination owns real GUI-login setup and
authorization. No real provider/session/service probe was run for this coding
task.

## Prevention concept

One explicit owner selects the shared Codex endpoint and effective CODEX_HOME.
Sessions borrow it; ending our session never stops the shared daemon. A scope
requests archive for workflow-owned threads or unsubscribe for retained human
threads, reports remote failure, and releases local routing. The last session
and in-flight operation releases its socket, reader and debug descriptor.
One active turn per owned native session prevents concurrent duplicate
assignments; different sessions and pending admissions remain parallel.

The native worker preserves continuation identity and ordinary final results,
then releases its scope on normal, cancelled and failed outcomes. An unresolved
native outcome remains an error rather than a background Close-retry worker or
cleanup journal. An explicit local session closure can retain its error in its
ordinary final state; it is not a claim that provider descendants stopped.

Private worker subprocesses use the existing executor owner and cmd.Wait reap
path. Startup acceptance/lease identity remains the basis for reconciliation,
not a blind new model assignment. No aggregate cross-session capacity was added:
the code has per-operation/session ownership and duplicate-admission guards,
but an owner-chosen aggregate resource budget still needs an explicit capacity
contract. A machine-wide two-server cap is not this task's requirement.

## Future observation under parent authorization

1. Identify the exact Gimbal/CLI/Desktop revisions, GUI-login identity, chosen
   shared daemon owner/endpoint and effective home. Inventory our workers,
   sockets/FD counts, loaded owned threads and identifiable Node/tool descendants
   before work. Preserve unrelated clients; do not read their private transcripts.
2. Run a small sequential normal/cancel/error sequence through the real Gimbal
   workflow, including one authorized tool-use turn, against that same endpoint.
   Observe each actual owner exit and session archive/detach boundary. Record
   process, descendant, socket and FD deltas after each completed owner lifetime.
3. Resume one retained human session with the same native identity, and keep an
   unrelated authorized client active to demonstrate that release does not stop
   the borrowed daemon or interrupt legitimate parallel work.
4. Apply bounded malformed/disconnect/startup fault injection only under the
   agreed disposable owner. Verify no fresh duplicate assignment, normal child
   wait/reap and local descriptor release; record remote uncertainty honestly.
5. Counts must return to or stabilize at an explained owner-bounded baseline.
   Growing unexplained retention is a failed observation. Local worker exit or
   a successful archive reply alone is insufficient proof of descendant unload.
   Do not use a 30-replica stress run, kill watcher or strict workflow timing gate.

A compatible first check, if parent approves it, is a read-only loaded-thread
query and socket/FD comparison against the existing healthy shared daemon,
using one explicitly closed client and no model turn. It establishes only
that client's transport lifetime; actual GUI/tool-descendant qualification
remains outstanding.
