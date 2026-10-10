# Codex desktop worker

correction: Automated runs must not depend on per-run human approval. Native app access is separate from MCP tool approval and OS permissions; only an actual first-use native prompt can provision Always allow. Settings documents review/revocation, not pregranting an unseen app.

decision: Keep this integration a concrete desktop MCP program with a local socket client. The private desktop messaging API cannot promise Gimbal's conditional active-turn Steer contract, so it is not presented as a HarnessAdapter or wired into shared workflow semantics.

decision: Persist ownership and logical start identities before external creation. An uncertain result is owned work to reconcile, never permission to create a duplicate. Cancellation archives and verifies native stopped state; it is stronger than a turn interrupt.

decision: Saved app approvals are a preflight declaration, not enforcement of an agent's later app choices or proof of TCC readiness. Reading a task waiting on approval attempts cancellation and reports a blocked unattended run. Callers must keep reading; there is no hidden background monitor.

friction: A project-local MCP server requires the fixture project to be trusted and its particular binding tool provisioned. Tool absence alone did not establish that ordinary MCP could not inherit the desktop bridge. Use real executor metadata rather than manufacturing first-party identity.

friction: macOS native Computer Use has app-level addressing rather than a hard window binding. A real preapproved app with a disposable frontmost window, title verification before input, and no unrelated content is a bounded native check; it is not a window-isolation guarantee.

decision: The shipped executable was exercised through direct trusted-project MCP configuration without restarting Codex. Marketplace packaging is supplied, but marketplace installation/reload was not exercised. A bound loaded owner task remains a runtime dependency; source packaging does not establish boot or login service behavior.

decision: Raw source caches, temporary fixture programs, screenshots, run outputs, and independent review evidence stay outside the repository. Native behavior and cancellation are reported in the PR; package-level protocol checks live beside implementation.
