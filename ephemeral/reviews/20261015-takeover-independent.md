# Independent takeover review: issue 574

Target: the uncommitted implementation in this worktree on base revision
`1e387b342bc88ce29d325f422d8314c43fb79009`, inspected on 2026-10-15.

Outcome: material findings remain.

## Evidence inspected

I read the authoritative `ephemeral/issue-574.md`, repository `AGENTS.md`,
`docs/definition-of-done.md`, the relevant Gimbal skill entry point, the full
handwritten diff, generated serialization changes, and surrounding session,
environment inspection, native menu, retained view, and window persistence code.
I did not rely on the predecessor's reports as verification.

Independently executed checks:

- `go test ./internal/conversation ./internal/modelalias ./internal/host`: passed.
- `pnpm exec vp test --run src/lib/conversation/NewSession.svelte.test.ts src/lib/conversation/ConversationView.svelte.test.ts src/lib/workspace/conversations.test.ts src/lib/shell/WorkspaceShell.svelte.test.ts`: 20 passed, 2 failed at the initially inspected snapshot. Both failures were exact-label expectation drift: the unavailable-harness label used a middle dot, and the environment chip included its directory. These failures do not establish broken product behavior. The caller subsequently changed those assertions; I have not treated the earlier results as proof of the revised suite.
- `git diff --check`: passed at inspection time.

I checked the effort policy against the current primary Anthropic effort
documentation at https://platform.claude.com/docs/en/build-with-claude/effort;
the inspected Claude maximum-effort choices are supported by that documentation.

## Findings

### Issue: reusing a setup can open a picker with no harness and crash its content

Locations: `web/src/lib/conversation/NewSession.svelte:63-69,82-88,122`;
`web/src/lib/conversation/HarnessPicker.svelte:16-18,41-42`;
`internal/conversation/setup.go:54-62,68-70`.

The newly retained/reused setup makes `choice` nonempty independently of the
currently observed harness inventory. A remote environment can legitimately
return an empty inventory: `internal/environments/connect.go:319-323` explicitly
treats that condition as a readiness warning, and `SetupEnvironment` then returns
no harnesses. `NewSession` still mounts `HarnessPicker` because its guard checks
only `setup && choice`. The picker computes `shown` from an empty array and
dereferences `shown.models`, `shown.efforts`, and `shown.id` when its content is
opened. This throws instead of allowing the person to inspect the missing
availability and recover by selecting another environment with the draft intact.

Reproduction: have a saved conversation in environment A, make A's successful
inspection return `harnesses: []`, choose native New Session Like for that saved
conversation, then open Harness and model. The existing NewSession browser-test
mock can reproduce this by setting `remote.setup.harnesses = []` and
`remote.recent` to a saved chat before render, then clicking that button. This
finding is based on the complete causal path in source; I did not add a test or
claim a live native reproduction under my review-only boundary.

The issue requires previous setup reuse and machine-specific availability to
remain usable. Handle the empty inventory without discarding the selected setup
or typed draft and cover that transition with a regression check.

### Issue: a chat whose outcome needs attention is represented as Working

Locations: `web/src/lib/workspace/ProjectWorkspace.svelte:71-74`;
`internal/conversation/manager.go:216-225`;
`internal/conversation/requests.go:71-72`.

The state expression checks `unresolved` in the Working branch before it checks
the error. On restart during a local request the manager explicitly says that the
turn is no longer active, clears its active request, retains the unconfirmed
outcome and an actionable error, and refuses replacement prompts. Nevertheless,
the sidebar and tab show the Working status because `status === 'unresolved'`.
The person cannot distinguish this blocked chat from an agent doing work by
looking at workspace state.

Reproduction: interrupt the server during a local chat turn and reopen the saved
workspace, or project a conversation with `status: 'unresolved'`, a restart error,
an empty `active_request`, and no running runs. The current expression returns
`'working'` unconditionally. `TabStatus.svelte` exposes that result as `Working`,
whereas its attention variant is `Needs you`.

This classification predates the diff, but the issue explicitly requires
working/attention states to remain distinguishable and this change rewrites the
same status projection to add unread state. The finding concerns requested
workspace truthfulness, not a redesign of workflow or machine state.

## Completion evidence boundary

The source and passing package tests support implementation existence and the
specific tested behavior. They do not establish native setup controls, command
routing, restart persistence, minimum-window legibility, or an inexpensive live
model delivery against a final revision. The caller is conducting native proof
separately. There is no final committed implementation revision in the inspected
snapshot, so I cannot certify the issue as completed or merge-ready from these
artifacts alone.

The existing-worktree selection wording in the issue is not fully resolved by
the implementation, which exposes new-worktree creation, base branches, and
shared-directory use. I have not elevated that ambiguous interpretation above
the two concrete behavior findings.
