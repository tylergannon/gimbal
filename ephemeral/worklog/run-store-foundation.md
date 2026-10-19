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

correction: The independent review found malformed terminal-projection JSON tags and an orphan cleanup identity minted after its intent. Correct the tags; commit orphan intent plus binding atomically and derive each retained episode's request identity from its stored action. Explicit stop without a runner now reports unavailable rather than claiming success.

correction: HTTP messaging's explicit 400/404/405 rejection of an identified request was wrapped as uncertain delivery. Preserve ContractRejected through the transport; check no old-operation fallback or polling for commands, turns and helper receipts, including failed binding release before explicit retry.

proof: Focused integration checks now replay saved lifecycle/agent/cleanup facts once into a fresh reducer after a failed command and invalid-answer/correction sequence, comparing the complete snapshot including usage and transcripts. A 1 MiB command-output test removes executor-spool and verifies exact stdout/stderr through OpenBlob in a separate process, while RunCommand remains bounded to excerpts. An in-memory Store drives the engine and retained cleanup without a directory-backend state directory.

correction: Full build requires CleanupStatus to use polytype.Optional and generated transport outputs; ordinary omitempty is outside the type grammar. Full Go validation also exposed web tests assuming name-suffixed run IDs and live reads of unaccepted initialization; update those expectations to explicit ULIDs and incomplete reservations.

friction: The globally installed vp resolves a different Vite runtime than this workspace and fails test startup with vite_ssr_environment_not_runnable. The pinned web/node_modules/.bin/vp test --run executes all 201 tests successfully. No application configuration change is needed.


correction: Capturing messaging admissions plus their stored owner routes exposed session-close cleanup contexts constructed from Background. Execution still reached the adapter's connection, but the stored close intent mislabeled its route as local. Preserve the owning scope's values with WithoutCancel for initial and retained closes; verify HTTP fork and messaging create/turn/close admissions, as well as the retained close route.

friction: Do not run generators concurrently with skgo build. Its input fingerprint correctly rejects a build if generation rewrites inputs during frontend compilation. Run generation to completion before the full build.

friction: The runtime worktree disappeared externally during final validation, after five commits were pushed. Restored the same branch at the same path and reconstructed the last uncommitted context fix and tests; verify them again before committing. Keep disposable live-proof fixtures outside the checkout so removing the checkout cannot erase their run stores.
