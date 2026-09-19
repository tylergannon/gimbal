# Live run status, timing, and connection accuracy

Implement the complete requirements in the two local issue files:
- /Users/tyler/.codex/worktrees/803e/gimble/ephemeral/requirements/ui-live-state/271.md
- /Users/tyler/.codex/worktrees/803e/gimble/ephemeral/requirements/ui-live-state/273.md

This is the next UI bundle after #274 (isolated E2E server), already merged.
The repository is /Users/tyler/.codex/worktrees/803e/gimble, branch codex/ui-live-state.

## Required outcomes

1. Interrupted turns and commands appear ended, not failed or waiting, including when an interruption carries an error string. Genuine failures still appear failed. Keep the map and history views consistent where they present these same records.
2. A folded running scope displays its actual elapsed time and advances while the run is idle. Ended scopes retain their final duration; recorded runs do not keep accumulating elapsed time.
3. An open SSE connection reads Live even when it sends no new events. A genuine connection loss reads Disconnected with elapsed time that continues across retry attempts, then returns to Live after reconnection. Historical runs read Recorded. Initial connection establishment can remain transiently Connecting.
4. Add focused regression coverage beside the affected code, and demonstrate the visible behavior in the running built app or existing browser component tests. Include idle connection establishment, outage/reconnect, ticking folded duration, interrupted turn/command, and genuine failure. Unit tests alone do not establish the visible UI claims. The independent validator must personally inspect or run relevant evidence and report what was actually observed.

## Boundaries and context

Follow AGENTS.md, docs/definition-of-done.md, docs/web-app.md, and the relevant design claims in docs/design/milestone-1-storybook.md. Use the svelte-component-factoring skill for Svelte changes. Keep this bundle limited to #271 and #273. #269 topbar search/navigation/run duration, #270 detail-pane content/selection, and #272 runs-list layout/loading are later bundles.

Potential investigation points, not prescribed fixes: layout.ts and the folded Sheet summary; status mapping in Node and HistoryLanes; the run route's EventSource retry state; and internal/observation/http.go's initial SSE header flushing when there is no new event after the SSR snapshot.

Do not modify these requirements, the implementation workflow, or weaken the fixed validation command. Do not commit, push, merge, install, or edit docs/. Parent agent owns delivery. Do not add proof programs or retain run output in tracked files. Use existing test infrastructure; temporary manual probes may stay outside the repository. Preserve the plan and other unrelated work.

The fixed gate is: just build && just vet && just test && just fmt-check && just e2e ui-live-state.
The parent seeds the existing cancelled-run fixture in ignored .gimble/runs for the current E2E prerequisites. E2E starts the fresh built binary on its own free port; the workflow's web server uses the installed pre-change binary and is for observing workflow progress, not validating the changed frontend.
