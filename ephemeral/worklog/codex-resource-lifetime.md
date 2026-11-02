# Our Codex resource lifetime

correction: Tyler's requirement concerns resources created directly or indirectly by our Gimbal/agent/tooling activity, not a global two-server cap. Vendor ancestry does not absolve our owner lifetime. The historical roughly 30-process incidents have not been causally attributed.

decision: Preserve PR #664 on codex/fixed-release-candidate. This resource fix branches from main a3ee508b as codex/codex-resource-lifetime in the clean task worktree; the older root checkout and .gimbal remain untouched.

decision: Cancellation is a message followed by local scope release and ordinary final results. Remove the unbounded worker Close-retry loop and cleanup.json confirmation journal; report uncertainty without retaining a cleanup lifecycle. Durable human sessions must remain resumable. This supersedes contrary retained-cleanup language in older repository contracts.

decision: Centralize terminal WebSocket retirement, and release an idle adapter's connection after its last owned session and in-flight operation ends. Preserve the borrowed daemon and concurrent sessions without adding an unrequested public disposal API or an arbitrary concurrency cap.

constraint: No real Codex app-server, Gimbal daemon, provider/model session, native GUI, or service lifecycle action is authorized for this coding task. Focused Go test doubles and compilation are allowed. Deliver a draft PR plus a bounded real-workflow proof plan; actual descendant/FD growth and historical incident causation remain unproven.

decision: Use the existing HarnessAdapter scope contract: release local session objects and connection routing even if archive/unsubscribe fails, and preserve the error in the ordinary result. A local explicit closed-session marker prevents reuse of the ended scope; it does not attest to provider cessation or schedule retries.

decision: A detached human session retains its checkpoint; uncertainty crosses the native worker boundary in the ordinary final result instead of being overwritten by cancellation. A completed native result can win the cancellation race. Propagate the previously ignored fork-parent detach error and release its child on failed admission.

limitation: Aggregate cross-session capacity and accepted-startup identity reconciliation remain distinct ownership-policy questions. Preserve the existing per-operation/session leases and one-active-turn-per-native-session admission, without inventing a machine-wide cap or claiming vendor descendants have unloaded.

review correction: An ending active turn could register its route again after Close removed it; a second live session prevented idle retirement from masking that retention. Gate all session route admission on the same closing lock, and reject routing on terminal transports. Cover the two-session case explicitly.

implementation correction: A late reader of a terminal transport must still receive the transport error, rather than treating rejected route registration as session cancellation. Return an unregistered channel for that dead connection and let its readDone signal report the original failure; retain no routing state.

review correction: Cancellation flags previously replaced a joined release failure with bare context cancellation at two worker result boundaries. Preserve the ordinary final error text and context identity together, including confirmed-stop plus failed unsubscribe. The subprocess and outer adapter regressions exercise both boundaries.

validation friction: Running the whole repository concurrently with package race review exhausted unchanged one-second root test deadlines and the ten-second parallel Codex recovery deadline. Keep deadlines unchanged; rerun with serial package scheduling to separate resource contention from correctness failures. No race-detector warning accompanied those deadline failures.

test correction: The confirmed-stop-history test waited for the fake daemon's received request count, then could cancel before the adapter learned its turn ID. Wait for acknowledged active.turnID instead; a lost acknowledgement correctly reports uncertainty and is a separate scenario. Preserve the original timeout.

decision: On 2026-11-01 Tyler accepted proof-first completion: demonstrate that repaired Gimbal does not accumulate attributable resources, investigating the historical roughly 30-process buildup if growth recurs. The new acceptance plan is ephemeral/plans/resource-lifetime-acceptance.md. Written-plan approval is still pending; the earlier no-runtime restriction remains in force until then.

decision: Existing task worktree and remote PR #665 both remain at cd4ed425 at planning intake. Reuse this implementation rather than duplicating the repair. Preserve the separate Locked Use investigation and inspect concrete service ownership before future runtime operations.

review correction: Acceptance round 1 found the proof plan did not identify the actual production topology: native Codex adapters live in per-turn worker processes, while the Gimbal instance and native daemon persist. Observe worker exit/reap and daemon-side connections/descendants across repeated work; a disposable run-prompt process exiting cannot establish per-scope stabilization. Add small concurrent admission/cold-start cases and verify private daemon isolation before fault injection.

correction: Tyler challenged the speculative cold-start test as a completion gate and authorized an interprocess file lock instead. The lock must cover recheck/start/readiness for the selected daemon; subprocess tests demonstrate exclusion and OS release. This closes only that particular startup vulnerability and is explicitly not evidence that a process leak is fixed. Ordinary parallel workload stabilization remains required. The private-home/login experiment is retired; no credential file was copied and the attempted supported access-token login rejected the cached token format.

decision: Tyler authorized execution of the reviewed plan without another approval gate when the task owner recommends it. Respect the separate #664 release integration hold: no main merges, version preparation, release dispatch or stable-tag changes; continue isolated source and qualification work.
