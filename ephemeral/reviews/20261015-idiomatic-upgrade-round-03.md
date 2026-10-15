# Independent idiomatic upgrade review, round 3

## Target and outcome

Reviewed the current issue-574 implementation at **`e8ab9ac34fc01d4056fa5b9ed7d46d68398afbf7`** in its original worktree against `ephemeral/issue-574.md`, repository instructions, and the user's rule forbidding `$effect()` for application state management. Re-examined the current source and branch diff, including the new `SessionSetup` extraction, rather than accepting the earlier verdict or the implementer's claims as evidence.

**Outcome: no findings.** The design is valid, simple, and idiomatic for the requested session/workspace work. I found no material code or architecture defect. Native acceptance is still in progress; this review does not establish that the full issue has been observed working or is ready to merge.

No scope-narrowing instruction was supplied or accepted. Only this review artifact was edited.

## Assessment

The new `SessionSetup` component has a concrete responsibility: it owns the mutually exclusive active pickers and their short lifetime. Its single `picker` discriminator changes only through binding/user callbacks. Removing the active component closes every popup without an effect-driven reset. `NewSession` retains the form and the per-environment choice map, so removing the controls preserves the draft and explicit choices. This boundary addresses popup lifetime and exclusivity without introducing a controller or duplicating draft state.

The parent's direct `applySetup` action remains an appropriate replacement for the nonce/effect command. Last defaults arrive through an explicit async handler, which respects prior choices. The form's hidden values continue to derive from the same selected setup used by the controls.

The rest of the architecture remains proportionate: persisted setup intent stays on Conversation, host window state owns sidebar/read facts, native menus project active-view capabilities, and explicit navigation/mutation/refresh completions acknowledge viewed replies. Run tab metadata derives from the shared live inventory. Active-only chat/file popup ownership replaces state-synchronizing effects. The retained effects in the changed flow perform external rendering, DOM work, or error-driven navigation rather than copying application state.

The empty observed-harness case is guarded, unsupported explicit effort remains disallowed, and unresolved chat state projects as attention. The prior round's resolved findings remain resolved; I found no basis for reintroducing them or recommending a replacement architecture.

## Evidence inspected

- Current repository instructions and the authoritative issue; agent-protocol, adversarial-review, Svelte component factoring, definition of done, and repository Svelte idiom guidance loaded during this review session.
- Branch diff from the merge base with `origin/main`, plus the latest extraction commit and surrounding component callers: composer/setup/pickers, chat actions, retained shell, workspace/navigation, recording bridge, run inventory, route/layout, native menu capability and command bridge, and affected tests.
- Conversation creation/effort admission and delivery metadata, observed machine/harness setup, durable Rename, host window/read-state persistence, and the generated transport extensions. The latest extraction does not modify those backend contracts.
- The official Svelte documentation previously inspected in this review session for [effects](https://svelte.dev/docs/svelte/$effect), [bindings](https://svelte.dev/docs/svelte/bind), and [navigation](https://svelte.dev/docs/kit/$app-navigation).

Independently ran against this revision:

- Seven affected Chromium suites covering NewSession, ConversationView, FileViewer, RecorderBridge, retained run panels, conversation status, and WorkspaceShell: **54 tests passed**, exit 0. The run includes the new picker mutual-exclusion regression and retained-draft/no-frame teardown coverage. No unhandled errors were reported.
- `pnpm check`: **passed**, zero errors. The two CSS `line-clamp` compatibility warnings are unchanged in `RunsList.svelte`.
- Working-tree and staged `git diff --check`: **passed**. The reviewed worktree was clean before this review artifact was added.

Earlier independent Go package checks passed; no Go implementation has changed since those checks. This round does not present cached package results as fresh native acceptance.

Component/type checks do not prove native menu execution, real inexpensive-model delivery, minimum-window usability, or persistence across relaunch/project switching. Those are explicitly pending acceptance observations, not claims supplied by this report. This review is also not a claim that every unchanged frontend component outside the issue-574 flow has undergone a global state-management migration.
