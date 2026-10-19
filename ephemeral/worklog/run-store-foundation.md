# Run-store foundation

decision: Implement the five ordered milestones with a commit after each, then independent adversarial review of the seam, completeness, bypasses, and application proof. Scope and reviewed mechanics are in ../run-store-design.md and ../run-store-session.md.

decision: This Gimbal worktree is a sibling of the attached Gimbal View worktree in the same task directory; runtime implementation belongs in Gimbal, per both repositories' instructions.

decision: Unit tests cover meaningful ownership, persistence, identity and failure contracts. Final acceptance includes running application behavior and an independent reviewer, not only passing checks.

decision: Launchers allocate bare ULIDs, the runner forwards invocation.Request.RunID, and the engine accepts only its explicit Run argument. Scope/session/action identity is separate from existing prefix-based display/control addresses. Constructor I/O happens outside scope.mu.

correction: Registry.add previously replaced a live object on duplicate registration, and IDs listed bare directories. Reserve without replacement and distinguish missing runs from incomplete initialization before opening an external projection.

decision: Productive HTTP/messaging operation names are identified-session/turn/process/close. In-tree registrations declare identity support; unsupported HTTP operations are definitive pre-admission contract rejections and do not enter result polling. Environment mutation receipts settle such rejection before binding release writes them.

decision: Action intents and executor bindings commit together before native admission. Command output projections tail a separate executor spool and authoritative outcomes own streamed blobs; retaining executor paths alone would not meet independent readback.

correction: A rejected remote session creation must return an empty native binding, rather than a nonempty run-plus-empty-ID transport key that creates a false cleanup obligation.

decision: New-run lifecycle and native events commit/write through runstore before existing logs/reducers. Observation errors are reported separately and do not replace native success or poison subsequent execution. Canceled event writes retry the same RecordID on a bounded cleanup context so a canceled turn can leave its session usable.

correction: Supervisor attachment names the next turn before dispatch; reserve that turn's stored slot before recording attachment, then reuse it in turn execution.

decision: New-format readers never take runner.lock or terminalize a run. Orphan mutation takes the store writer before the pre-existing runner lock; status is read-only. Background cleanup keeps one episode and suppresses unchanged facts. Positive terminal/released projections skip Store.Open.

correction: Service task failure does not prove cleanup failure. Track actually unconfirmed service cessation separately, join descendant service output after ancestor cleanup, and retain recording ownership until known pending resources finish.
