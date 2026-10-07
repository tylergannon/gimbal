# Issue 430 diagnosis

decision: Diagnose against fetched origin/main at 4377acbc7581bc8a20551eb0dde7f7276ba1fc61. This task is diagnosis and a half-page plan, not implementation.

friction: Codex readTurn returns on lost transport, RunTurn clears active ownership, and implement exits before independent validation. Reconnection resubscribes sessions but discards returned turn history; it does not reconcile the interrupted operation. Lost turn/start acknowledgement is also ambiguous and must not cause blind replay.

decision: Installed codex-cli 0.160.0's generated protocol exposes thread status, turn status and items, and rejoins a running thread by threadId. These shapes support a reconciliation design but do not establish live disconnect/restart behavior. Verify active, completed, and interrupted cases with an isolated daemon before choosing continuation behavior.

decision: Keep bounded automatic recovery inside the Codex adapter so the existing Generate and workflow stack remain intact. If reconciliation cannot safely continue, retain same-run task state and an explicit operator resume path. Limit continuation work to the required existing workflow; do not introduce arbitrary Go workflow replay.

friction: Close deletes ownership before archive, and the current dead-connection test requires that deletion. Retained cancellation and retryable cleanup need coordinated changes with issue 422, including replacement of that test's incorrect expectation. Shared-daemon restart and unrelated-thread control remain outside this repair.

decision: The existing Claude unknown-tool retry sends a new follow-up; broadening it to Codex EOF would permit duplicate execution. Recover native state first. Prove preservation of tool effects, no concurrent replacement, one task validation, and no replay of accepted outcomes across active-turn loss, lost completion, durable-history restart, persistent outage, and cancellation.

correction: Tyler authorized implementation and clarified that adapters own connection maintenance, reconnection and responsible best-effort turn completion. Put the contract in HarnessAdapter Godoc and the canonical Gimbal View promises, without claiming every adapter is already proven to meet it.

decision: Bounded automatic reconciliation stays inside the original RunTurn. Ambiguous interruption or exhausted retries pauses that same call; the existing session steer accepts resume as an explicit adapter control. Cancellation returns its original cause, while failed Close retains ownership and a terminal hosted run retains its cleanup controller. The cancel CLI reaches that controller without dispatching another assignment. Host-process restart is outside same-call recovery.

friction: Live daemon protocol rejects full-history thread/read. Read thread metadata and explicitly page thread/turns/list; a brand-new thread is not materialized until its first user message. The generated shape alone did not establish usable history.

decision: The approved diagnosis plan explicitly included preserving recovery and stop control after cleanup failure and coordinating ownership with 422. Terminal cleanup control is necessary to make retained adapter ownership actionable; this is not a claim that every part of 422 is completed. Make that ownership explicit in the internal live Controller contract so compiled hosted runs inherit it.

correction: Independent review identified cancellation amplification through shared writes, inadequate transient retry spacing, stale native turn status after restart, and confirmed stopped-daemon cleanup. Use bounded connection writes independently of one caller, retry at 0/10/30 seconds within a minute, allow only operator-confirmed continuation on an idle thread even with stale turn status, and accept verified stopped-daemon cessation. Connection refusal while running remains pending.

decision: Round-02 performance finding is valid: pre-start reconciliation only needs native turn IDs. Request notLoaded item views on the healthy path; fetch full durable items during recovery.

disposition: Round-02 proposal to time out the paused state is not required behavior. Issue 430 explicitly asks to retain recoverable same-run state with a supported resume action when automatic continuation is unsafe. Automatic attempts stop, a stable actionable paused notice is emitted, and cancellation remains available; callers can bound lifetime with their own context deadline. Returning from Generate after an arbitrary timeout unwinds the Go workflow, removing that same-stack resume path. Keep the documented human-controlled pause; do not introduce workflow replay.

friction: Independent native probing observed idle thread metadata beside an inProgress turn immediately after acknowledgement. Ambiguity is not necessarily stable: re-read within the existing retry budget before pausing, without dispatching work. Closing sessions need archive, not resume; exclude them from resubscription so missing rollout history cannot block unrelated healthy sessions or its own cleanup retry.

friction: An unread per-thread notification channel can block the shared WebSocket reader during recovery backoff or pause. Bound display buffering by retaining the latest notifications and reporting/counting dropped older events, so RPC replies and terminal results can proceed. Keep native answers, terminal notifications and interactive requests; retire the connection for reconciliation if the bounded buffer contains only such critical messages. One operator confirmation belongs to one pause; consume its eligibility at acceptance, not later.
