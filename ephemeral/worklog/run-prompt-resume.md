decision: Issue #688 retains native conversations across CLI exits with --resume; caller defaults apply only when creating a session.
decision: Keep one-shot run logs and scope ownership intact; run-prompt owns the retained native conversation until --close.
friction: Fresh worktrees need generated web assets before the complete cmd/gimbal tests; run just build first.
correction: Live-test cleanup needs its own context because testing cancels t.Context before cleanup.
decision: --close submits an executor close operation, so the session lease rejects active turns atomically instead of cancelling another caller.
