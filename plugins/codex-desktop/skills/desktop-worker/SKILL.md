---
name: desktop-worker
description: >
  Bind and use Gimbal's experimental local Codex desktop worker. Use when the
  user asks to provision this worker or submit, inspect, message, or cancel its
  owned desktop tasks.
---

# Gimbal desktop worker

Read [the plugin README](../../README.md) for setup, socket requests, and recovery.
The bridge is private and version-dependent; the owning desktop task and its MCP
process must remain loaded.

For provisioning, invoke `gimbal_desktop.bind_worker` from the intended owner task
with `{}`. Let Codex supply executor metadata. Binding is the only persistently
approved plugin tool. Native app consent is separate: the user provisions each
actual app by choosing Always allow in its first-use permission prompt; Settings
→ Computer Use manages saved access. Never edit the consent
store or automatically approve GUI prompts.

Before work, call `status` with the required native bundle IDs. Saved approval is
not proof of OS permission or successful computer use. Submit authorized work
with a stable `requestId`, a real desktop target, and local absolute paths for any
requirements files. Default to `gpt-5.6-luna` unless a model was explicitly chosen.
Inspect running work through `read`; it attempts archive-and-stop when approval
is required and returns an explicit failure. Check cancellationConfirmed and retry
`cancel` if cleanup is unconfirmed. Declared requiredApps are preflight, not app
access enforcement.

Use `message` for authorized follow-ups; it does not promise active-turn steering.
`cancel` archives and stops the owned task. On a timeout or `outcome_unknown`,
inspect before reconciliation and preserve the original request identity. Never
create a replacement task blindly. Only ledger-owned tasks are addressable.

Report the task ID, model, observed result, and any permission or lifetime blocker.
Separate fake tests, direct MCP execution, installed-plugin behavior, and observed
native-app behavior. This worker does not satisfy the full Gimbal HarnessAdapter
contract.
