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

review correction: Cancellation flags previously replaced a joined release failure with bare context cancellation at two worker result boundaries. Preserve the ordinary final error text and context identity together, including confirmed-stop plus failed unsubscribe. The subprocess and outer adapter regressions exercise both boundaries.

validation friction: Running the whole repository concurrently with package race review exhausted unchanged one-second root test deadlines and the ten-second parallel Codex recovery deadline. Keep deadlines unchanged; rerun with serial package scheduling to separate resource contention from correctness failures. No race-detector warning accompanied those deadline failures.
