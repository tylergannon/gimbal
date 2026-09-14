# Issue 117 result

## Implemented

- Codex native `error` notifications now project retryable failures as
  `session.retry.scheduled` and terminal failures as `session.step.failed`.
  A `willRetry` event leaves the turn open for the app-server-owned retry and
  does not invent an attempt, delay, or schedule.
- Claude native retry, assistant-error, result-error, and permission-denial
  messages now project into the same existing retry/error vocabulary.
- Codex approval server requests and Claude `CanUseTool` callbacks project as
  `permission.asked` followed by the fixed noninteractive decision in
  `permission.replied`. Replied permissions remain in both the Go and browser
  projections so completed runs retain the request and decision.
- Codex child-thread events and Claude `parent_tool_use_id` envelopes are kept
  losslessly below the tool that spawned the child. Their own projectors do not
  change parent steps, prose, tools, or usage. Late child events can update a
  completed parent tool, and the page renders the newest complete transcript.
- The observation page displays retry/error state, answered approval history,
  and expandable subagent transcripts using the existing message rows.

## Live validation

The production `web.NewRuntime` handler ran
`01M2H4C19CFCM1X3XVAXPPRMQ6.issue117-native-events` with Codex
`gpt-5.6-luna` and Claude `claude-haiku-4-5-20251001`.

- The persisted Codex child transcript contains 11 events and the exact
  `CODEX_CHILD_NATIVE_117` marker. It is attached to the original native
  `spawnAgent` tool even though a later `wait` tool names the same child.
- The persisted Claude no-tool child transcript contains two native envelopes
  and `CLAUDE_CHILD_NATIVE_117`.
- A second Claude child used Bash. Its five retained native envelopes include
  assistant tool use, the child `user` tool-result envelope, the command/output
  marker `CLAUDE_CHILD_TOOL_NATIVE_117`, and final
  `CLAUDE_CHILD_TOOL_DONE_117`.
- Both provider parent turns completed with their exact parent markers. No
  child event created a top-level parent step or contributed child usage to the
  parent session.
- An intentionally nonexistent Codex model produced and rendered an Assistant
  Failed row containing the native HTTP 400 `invalid_request_error`. The same
  bounded Claude probe rendered the native pre-step
  `Claude assistant error: model_not_found` and an execution failure.
- The production browser rendered expandable `Subagent transcript · 2 events`,
  `· 5 events`, and `· 11 events` sections with the exact markers above, plus
  the two harness errors.

Local retained evidence:

- Project: `/Users/tyler/.codex/worktrees/8e2d/gimble/ephemeral/attest/issue117/.gimble`
- Run: `01M2H4C19CFCM1X3XVAXPPRMQ6.issue117-native-events`
- Browser:
  `http://127.0.0.1:8117/runs/01M2H4C19CFCM1X3XVAXPPRMQ6.issue117-native-events`
- Attestation program: `ephemeral/attest/issue117/main.go`

## Native delivery limits

The production adapters retain their required fixed policies. Under Codex
`approvalPolicy: never`, app-server 0.153.4 reports `request_user_input` as
unavailable in the current mode and sends no approval request. Under Claude
`permissionMode: bypassPermissions`, the SDK does not invoke `CanUseTool`.
Consequently, approval request/decision projection is native-shape regression
coverage, not a live-observed callback claim.

The successful Luna and Haiku requests emitted no native retry notification.
Retry projection is likewise covered by native protocol fixtures and was not
manufactured through credential, daemon, or network disruption. Terminal
native errors were exercised live through the isolated nonexistent-model
turns described above.

## Checks

- `just build`: pass
- `just test`: pass, including all Go packages and 17 web tests
- focused `go test ./codex ./claude ./internal/sessionstate`: pass
- `pnpm check`: pass with zero errors and warnings
- `just vet`: Go vet and build pass; the recipe's repository-wide custom lint
  then reports five unrelated existing findings in
  `internal/workflows/sprint/sprints.go`, two old ephemeral workflow programs,
  and `issue159_test.go`. None is changed by issue 117.
