# Issue 126: run completion and synchronization

Retain blocking operations and make ownership explicit using existing Go
constructs. The intended outcome is that workflows and tests no longer need
one-off buffered channels merely to run a blocking operation concurrently and
collect its error. Readiness signals and event transport may still need channels.

First, review representative call sites: a concurrently observed run, a turn
receiving steering or cancellation, and a reader following a live log. Establish
who starts, cancels, and joins each goroutine on every exit path, and who receives
its result. Current Run, Group.Wait, and reader documentation already addresses
parts of the issue; identify the remaining gaps before changing the contract.

Keep Run and Generate blocking. Use Gimble Group for concurrent workflow work
and ordinary Go synchronization for external orchestration. Cancellation requests
a stop; returning or joining establishes completion. Run's returned error remains
authoritative for execution, including cleanup and recording failures. Reader
errors describe observation separately; ending observation does not establish
execution success or cancel execution unless the caller explicitly chooses that.

Apply the contract at the affected call sites, removing redundant error channels
where existing constructs suffice. A reader waiting for the final log record
cannot be joined inside the body whose return produces that record. Do not add
a high-level wrapper or a new exported name. If an existing primitive cannot
express a required case, document the concrete deficiency for a decision.

Completion requires focused evidence for normal completion, cancellation,
worker failure, reader failure, and early observer exit: deterministic joining,
preserved authoritative errors, and no stranded goroutines. Demonstrate changed
workflow behavior in a live run using the repository's cheap attestation models
and report the model. Deliver the concise contract decision, representative
before/after code, and behavioral evidence, scoped to issue 126.

## Contract decision

`Run` and `Generate` remain blocking. A workflow starts concurrent children
through `Group` and returns `Wait`; the body owns that join. External code that
must inspect a live run starts `Run` in its own goroutine, records its one
result in a variable, and calls its `sync.WaitGroup.Wait` before using it.
That replaces `runDone <- Run(...)` and a later receive without adding a
Gimble wrapper.

`runlog.Read` is an observer, not a participant in execution. Its context
only ends the reader; its returned error is reported separately and never
rewrites the result returned by `Run`. A reader that requires `complete` runs
outside the body, because returning from that body produces the record.
Readiness gates and log/event transport remain channels where they carry a
signal rather than smuggling a goroutine's result.

When an external observer waits for a run directory, it uses a child context
cancelled only after `Run` returns. If readiness never appears, the owner joins
first and reports `Run`'s error before the observer's cancellation; a failed
startup cannot strand the observer or hide the execution verdict.

## Evidence

`TestIssue126CompletionContract` joins each started run and demonstrates a
successful concurrent reader, independent reader and worker failures, a reader
that exits early without cancelling the run, and a cancelled run that returns
only after its join. The focused kill/cancel and runtime registry tests retain
the same checks with explicit joins.

The live record under `ephemeral/attest/issue126/logs/` is run
`01M2GRK1K6X5N539KCB0R66DFF.completion`: Codex `gpt-5.6-luna` returned
`ISSUE_126_LIVE`; its log has a clean `turn_ended`, `run_ended`, and final
`complete`, while the external reader reached that record before the joined
`Run` returned nil.
