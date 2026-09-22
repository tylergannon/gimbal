# Parent workflow notifications research

discovery: A transport event, a visible notification, and scheduling another model turn in the original conversation are separate capabilities. Research must establish all three before claiming a parent can finish its turn and be awakened by Gimble.

discovery: At 73e7cbb, web.Runtime.launchConversationWorkflow already starts review/implementation under the runtime context and returns a run ID plus a completion channel. internal/conversation.Manager.watchRun saves the terminal state but does not start another parent turn. Existing observation SSE supports a stream/position cursor with snapshot fallback.

decision: Research only. Preserve the existing workflow API; investigate client wake support before proposing production implementation or new exported names.

discovery: Read-only inspection reached the running local Codex daemon and found the original desktop parent via thread/loaded/list and thread/read. Official turn/start documentation supports toolOutput with empty input and queues it when a regular turn is active. This is a concrete experiment candidate; idle callback execution and desktop rendering are still unproven.

discovery: Claude Code Channels document same-session event-driven turns but do not register on MCP 2026-07-28. The modern Tasks extension and legacy channel notifications are different paths; latest protocol support is not automatically the right integration choice.

discovery: Gimble's Codex worker adapter archives its owned threads on Close. A connector borrowing the user's parent thread must not reuse that ownership behavior.
