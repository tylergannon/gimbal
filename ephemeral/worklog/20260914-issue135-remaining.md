# Remaining issue 135 work

decision: User requested resetting this worktree to updated origin/main and handing a plan to Sol for implementation. Reset completed at 2ac8940; work continues on codex/issue-135-remaining.
correction: PR #202 already covers delayed/serial tool attribution and live Claude. Remaining work is native fork/resume capability and reconnect coverage; an upstream limitation must not trigger synthetic history or a proof framework.
friction: regenerated codex-cli 0.153.4 experimental types still put `experimentalRawEvents` only on `ThreadStartParams`; native fork and resume after restart/unload cannot opt in -> leave those requirements open instead of reconstructing history or inferring response boundaries.
evidence: closing only the adapter WebSocket and redialing the still-running daemon preserved exact raw response IDs for a gpt-5.6-luna tool response and its separate final response; both production-browser rows carried nonzero usage.
decision: retain completed live-run files through `GIMBAL_LIVE_PROJECT` so the existing production handler can render the exact reconnect transcript after the live-gated test exits; this adds no production API or validation framework.
