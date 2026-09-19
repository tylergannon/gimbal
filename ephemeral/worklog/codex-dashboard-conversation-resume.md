# Codex dashboard conversation resume

decision: Scope is the human-facing Codex conversation introduced in #287. Workflow-run sessions retain archive-on-close, and Claude/agy restart continuation remains outside this implementation slice.
decision: Persist the native thread ID, detach with thread/unsubscribe on graceful shutdown, and resume lazily on the next message. Opening saved history must not contact Codex.
friction: The generated transport rejects `omitempty` on ordinary fields; durable native_session is therefore an explicit string in the conversation JSON and generated UI type.
friction: The first live proof assumed the shared Codex daemon was already running and failed at its PID precondition. Normal first-turn work starts the daemon, so the stable baseline belongs after that first turn and before detach/resume.
friction: The current daemon control service can report running without a persistent process matching the older `app-server --listen unix` probe. New proof compares the daemon's stable control socket before and after resume instead of relying on that outdated process shape.
friction: A race-enabled focused check found Manager.Get cloning after releasing the read lock while workflow completion mutated Runs. Clone under the read lock; this was a concrete detector failure in the touched manager, not speculative hardening.
