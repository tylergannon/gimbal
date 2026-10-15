# Independent idiomatic upgrade review, round 2

## Target and outcome

Reviewed the current implementation and uncommitted upgrade in the original issue-574 worktree at HEAD `62225f95`, against `ephemeral/issue-574.md`, repository instructions, and the user's prohibition on using `$effect()` for application state management. This is a fresh examination of the revised source, not acceptance of the implementer's account.

**Outcome: no findings.** The previously identified state-management paths are corrected. The upgrade uses simple, idiomatic ownership and data flow; I found no material correctness defect or need for re-engineering in the reviewed work. Native acceptance remains pending, so this is not a declaration that the issue is ready to merge.

No scope-narrowing instruction was supplied or accepted. Only this review artifact was edited.

## Prior finding independently reassessed

- **Run tab state:** `ProjectWorkspace` now owns the live run inventory shared by tab labels and the Runs view. `runTabs` derives from that inventory. `ProjectRuns` renders supplied data and exposes user actions; its effect-driven callback no longer copies tab state into the parent. This follows the repository's shared-data ownership guidance and removes the duplicate mutable tab inventory.
- **File popup state:** the active-only `Popover.Root` in `FileViewer` owns its lifetime. Deactivation destroys the popup rather than synchronizing a `menu` variable in an effect. The viewer itself remains retained.
- **Recording navigation:** the bridge derives its recording query from native status. The native navigation handler sends completion after the awaited navigation and DOM flush; `arrival` and its clearing effect are gone. Initial project selection is sent at mount, and subsequent selection follows the navigation callback.
- **Menu projection:** the workspace's remaining effect only renders derived capability data into AppKit through `nativeMessage`; it does not mutate Svelte application state or persist/copy state into the application's store. This is an appropriate external side effect, analogous to rendering into the DOM. It also avoids missing native-menu updates when any contributing data changes.

I do not retain the round-one finding against these revised paths. This is a review of the issue-574 upgrade and the relevant surrounding flow, not a claim that every unchanged component elsewhere in the application's frontend has undergone a global migration.

## Architecture and correctness

The direct `NewSession.applySetup` operation is a suitable replacement for the counter-prop/effect command. Draft choices remain component-owned and keyed by environment; the form survives picker and same-project tab changes. The explicit defaults-arrival handler respects choices already made while history loads.

Active-only `ChatActions` is a coherent boundary: it owns Rename and cleanup dialogs, while `ConversationView` retains the message form. The shared inventory and explicit mutation/refresh callbacks remain the source for seen-reply acknowledgments. Persisted effort/setup intent remains conversation-owned, and sidebar/read facts remain host-owned. No generalized controller, event framework, duplicate model catalog, or new storage system has been introduced.

The empty-harness recovery case now renders safely. Unresolved conversation outcomes receive attention precedence. I found no source-backed reason to reject these bounded changes or replace their architecture.

During review I found incomplete callers after the `ProjectRuns`/`RunPanel` prop changes and a missing `afterNavigate` export in the recording test mock. I inspected their corrections in the current source, including the route caller, retained-run harness, and retained-run tests. They are no longer findings.

## Evidence

Inspected the issue, repository instructions, the full current upgrade diff, the prior implementation diff against `origin/main`, changed component callers/tests, current native command/recording handlers, run inventory remote, and the installed SvelteKit navigation implementation. The prior review's official Svelte documentation remains applicable: [effects](https://svelte.dev/docs/svelte/$effect), [bindings](https://svelte.dev/docs/svelte/bind), and [navigation](https://svelte.dev/docs/kit/$app-navigation).

Independently executed:

- Expanded Chromium run covering NewSession, ConversationView, conversation status, WorkspaceShell, FileViewer, retained run page/panels, and RecorderBridge: six suites and 50 tests passed; the recording suite initially failed to import because its navigation mock omitted `afterNavigate`.
- After inspecting the mock/caller repairs, independently reran `src/lib/workspace/RecorderBridge.svelte.test.ts`: **three tests passed**, exit 0. The combined affected coverage is seven suites and 53 passing tests; the recording rerun followed the only reported failure in the expanded command.
- `pnpm check`: **passed**, zero errors; two existing CSS compatibility warnings in `RunsList.svelte` about the standard `line-clamp` property.
- `git diff --check`: **passed**.

Round one's independent Go checks passed with cached results. This round changes frontend state ownership and does not present those cached Go results as fresh native acceptance.

These checks support the exercised browser behavior and type contracts. They do not establish native menu execution, real inexpensive-model delivery, relaunch persistence, or visual usability at native window sizes. Those observations remain the final acceptance work.
