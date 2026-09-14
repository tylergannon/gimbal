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
