# Durable Interactive replan

correction: Read the full source conversation from original requirements through late simplifications. Retain steering, Stop, normal archive completion and local then remote execution; optional cuts proposed by the old agent were not accepted.
correction: The human rejected recovery machinery again during this rewrite. Remove new replay, startup reconstruction, recovery scans and reconciliation rather than rename them or implement a smaller recovery system.
decision: Waiting parks ordinary Go goroutines; one interaction admits input and one session owner serializes native delivery with turn completion. Existing journal records input and reply/wait transitions.
decision: Replace contradictory spec text alongside the plan. This is a planning-only branch from cc976137; the failed implementation and its uncommitted repairs stay preserved in the original worktree.
