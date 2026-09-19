# Conversation workspace prototype

Build the user's requested 85% prototype, stopping once the coarse visible functionality works and is demonstrated. Do not polish the last 15%, invent infrastructure, or fix hypothetical edge cases.

User's desired experience: a ChatGPT-like list of conversations using any accepted harness. Opening a conversation requires selecting its provider. Each conversation creates a Git worktree and associates its branch with that tab/session. The person discusses work with the agent, the agent can start runs inside the same server, and the person monitors them inside this application. Single project now. Multi-project soon; agent UI navigation/follow mode is explicitly future work.

Core outcomes for this delivery:
1. Visible conversation list/new-conversation flow, required provider selection covering the currently accepted Codex, Claude, and agy/Gemini harnesses, useful current-provider display, and a normal chat transcript/composer.
2. A real new Git worktree and branch are created and associated with each conversation. Messages use that workdir and a continuing harness session. A follow-up message retains conversation context. Conversation list/history and branch/worktree association survive page navigation/reload; saved history remains available after server restart.
3. The conversation agent can launch useful existing Gimble workflow runs inside this server, directed at its worktree. The UI exposes a resulting run link/status and the existing run page/dashboard monitors it. Demonstrate an actual agent-requested launch, not a fake run id or instructions telling the human to use a separate terminal.
4. Working built frontend and actual cheap-provider chat/run evidence, relevant checks, and independent validation. Commit, merge and install through manager once working.

Implementation proceeds in coarse slices: first the usable chat/worktree UI, then agent-driven run launch/integration. This document retains the full goal across those slices. No requirement for accounts, multi-project routing, agent-follow UI, attachments, rich message editing, transcript search, elaborate provider configuration, or perfect restoration of native provider process state across server restart. Keep simple honest failure/status display.
