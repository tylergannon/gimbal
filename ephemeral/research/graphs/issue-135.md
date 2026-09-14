# Complete native event adapter coverage for resumed turns and tool attribution

URL: https://github.com/tylergannon/gimble/issues/135
State: open
Updated: 2026-09-13T16:37:33Z

The initial native session observation UI is demonstrated for a live first Codex turn and deterministic streaming/reconnect fixtures. Remaining adapter work from #130:

- Enable and demonstrate exact raw response events on resumed/forked Codex turns. The adapter starts a fresh app-server process per turn; the current opt-in is on thread/start. Do not infer response completion from token-usage notifications.
- Correct tool-to-model-response attribution when tool activity arrives after rawResponse/completed. Content currently renders, but can appear in the following displayed step.
- Run the Claude adapter through the production browser with a cheap model; adapter and SDK tests alone do not demonstrate the live path.

Acceptance: use the production handler and inspect the actual native events and rendered tool/final content for these paths. Reuse the existing browser proof rather than building another validation framework.

