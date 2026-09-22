# Parent workflow notifications research

discovery: A transport event, a visible notification, and scheduling another model turn in the original conversation are separate capabilities. Research must establish all three before claiming a parent can finish its turn and be awakened by Gimble.

discovery: At 73e7cbb, web.Runtime.launchConversationWorkflow already starts review/implementation under the runtime context and returns a run ID plus a completion channel. internal/conversation.Manager.watchRun saves the terminal state but does not start another parent turn. Existing observation SSE supports a stream/position cursor with snapshot fallback.

decision: Research only. Preserve the existing workflow API; investigate client wake support before proposing production implementation or new exported names.

discovery: Read-only inspection reached the running local Codex daemon and found the original desktop parent via thread/loaded/list and thread/read. Official turn/start documentation supports toolOutput with empty input and queues it when a regular turn is active. This is a concrete experiment candidate; idle callback execution and desktop rendering are still unproven.

discovery: Claude Code Channels document same-session event-driven turns but do not register on MCP 2026-07-28. The modern Tasks extension and legacy channel notifications are different paths; latest protocol support is not automatically the right integration choice.

discovery: Gimble's Codex worker adapter archives its owned threads on Close. A connector borrowing the user's parent thread must not reuse that ownership behavior.

decision: Tyler requested Claude Fable consensus on a composition pattern and delivery plan. The proposed boundary is one observer of existing run observations plus an injected parent delivery function, composed once at application launch rather than interleaved through workflow execution.

friction: Installed agent CLI launched codex app-server despite explicit anthropic/Fable flags in this session. Stopped that attempt and launched Claude CLI directly with claude-fable-5-1; only the actual Fable review can count toward the requested consensus.

correction: Fable round 1 found that a runtime-held delivery function did not fit the out-of-process Claude channel. Revised the design so the same MCP connector process owns the observer and bound sender for both Codex and Claude Code; the runtime only launches workflows and exposes its existing observation stream.

correction: Fable identified missing external worktree ownership, competing receivers of the one-value completion channel, an unproven source of parent identity, and a missing ordinary Claude Desktop experiment. The revised plan makes these concrete and removes speculative durable notification ledger machinery.

correction: Fable round 2 confirmed the shared connector composition but found that review is read-only by prompt, not by sandbox. Both external workflows now use isolated worktrees. The plan also explicitly publishes the runtime's actual run URL and gates Codex binding on observed host identity/process scope instead of treating a model-supplied ID as authenticated.

decision: Claude Fable 5.1 session 5dc86ff9-69dc-4263-853f-2b20ae8efb53 reached only nitpicks remain in round 3. Consensus is on the delivery plan and observer-plus-adapter composition, not proof of host wake behavior. The host experiments remain the first delivery gate; no application code changed.

correction: Tyler identified the active migration in task Investigate issue 326. Its plan and current 2128 worktree introduce one multi-project Instance, ordinary CLI submission, project-qualified routes, and remove the direct conversation callbacks. The prior notification plan was based on stale topology; its consensus is superseded. Revised around instance-owned per-subscription observers and bound destinations, with Claude stdio only a thin transport client. No global provider mode, alternate launcher, or new worktree policy.

decision: Fable round 04 independently inspected the revised design and active migration code and found no material issues, only nitpicks. Recorded its clarifications about registration CLI identity, retained web origin, explicit caller workdir guidance and the new selected-update control route. Consensus applies to composition, not proven host wake capability.

decision: Wait for #326 to land before implementing notification integration; rebase and verify landed interfaces first. Independent host experiments do not depend on that merge. The migration worktree was read only; no application code changed in this task.
