# Independent idiomatic upgrade review, round 1

## Target

Reviewed the implementation on `codex/issue-574-session-workspace`, HEAD `62225f95`, together with the current uncommitted upgrade in the original worktree. The authoritative requirements are `ephemeral/issue-574.md`, repository instructions, and the user's explicit rule: **`$effect()` must never be used for application state management.** This review assesses both correctness and whether the approach is simple, idiomatic, and proportionate. Native acceptance is separate and remains incomplete; this report does not certify native behavior.

No scope-narrowing instruction was supplied or accepted. The review remained read-only except for this artifact.

## Architectural assessment

**Retain this architecture.** The upgrade is a sensible, bounded improvement, not a new framework or a replacement of the working ownership model.

- `NewSession` keeps its form and environment-specific choices. `applySetup` expresses the imperative action directly, removing the nonce prop and effect-triggered command. Awaiting navigation and `tick()` before invoking the mounted composer is a small, appropriate bridge.
- Recent defaults arrive through an explicit async handler. The `touched` boundary prevents late history from replacing explicit choices. The choices map keeps the draft with the component that owns it.
- The active-only `ChatActions` component gives Rename and cleanup dialogs a lifetime that ends with chat activation, without destroying the message form. This is an appropriate product boundary and a straightforward way to discard stale popup state.
- Shared conversation inventory remains owned by the workspace. Explicit mutation and refresh completions acknowledge the viewed reply and publish native capabilities and persisted window facts. No transcript cache or second domain store is introduced.
- Durable effort/setup metadata remains on Conversation, read/sidebar facts remain host-owned, and Swift remains a thin menu projection and command bridge.
- The picker now tolerates an empty observed harness inventory, and unresolved chat state receives attention precedence.

No new architectural layer, generalized controller, event framework, or re-engineering is warranted. The remaining instruction-compliance issue below is bounded and should not be described as an untenable implementation.

## Finding

### Issue: the surrounding workspace still manages application state through effects

**Requirement:** the user's unconditional prohibition, now also recorded in `AGENTS.md`, forbids copying or synchronizing application state from effects. Removing the effects in `NewSession` and `ProjectWorkspace` does not remove the same mechanism from their active workspace descendants.

**Evidence:**

- `web/src/lib/workspace/ProjectRuns.svelte:14-16` builds tab records in an effect and invokes `onstate`. `ProjectWorkspace.svelte:238` receives that callback as `value => runTabs = value`. The callback does not turn this into a data-arrival handler: its caller is still an effect, and its result is application tab state.
- `web/src/lib/workspace/FileViewer.svelte:15-16` keeps `menu` in state and resets it from an effect when `active` changes. This is the same popup-state synchronization pattern that the upgrade correctly removed from the session and chat components.
- `web/src/lib/workspace/RecorderBridge.svelte:59-63` clears `arrival` in an effect; `68-71` assigns the recording query into `stream` from an effect. These are application/navigation state transitions, distinct from the adjacent effects that solely communicate with the native bridge.

These sites predate this uncommitted upgrade. They are not evidence that the new composer or `ChatActions` design is wrong, and I did not reproduce a functional failure caused by them. They do prevent an unqualified claim that this workspace applies the user's rule throughout its application-state flow. Resolve these bounded cases with derived values, actual navigation/native callbacks, or active-only popup ownership, or explicitly establish that the requested upgrade is limited to the session/chat implementation before claiming compliance. Do not perform a broad framework rewrite.

## Evidence inspected

- `AGENTS.md`, agent protocol, adversarial-review, Svelte component factoring, `skills/gimbal/SKILL.md`, `docs/definition-of-done.md`, the issue, and the repository Svelte idiom guidance.
- The relevant implementation diff against `origin/main`, current upgrade diff, new `ChatActions.svelte`, affected tests, and surrounding conversation creation/resume, observed harness setup, host window persistence, retained workspace shell, recording bridge, native menu bridge, generated transport declarations, and route/layout state.
- Official Svelte documentation for [effects](https://svelte.dev/docs/svelte/$effect), [component/function bindings](https://svelte.dev/docs/svelte/bind), and [navigation callbacks and goto](https://svelte.dev/docs/kit/$app-navigation).

Independently executed:

- `pnpm exec vp test --run src/lib/conversation/NewSession.svelte.test.ts src/lib/conversation/ConversationView.svelte.test.ts src/lib/workspace/conversations.test.ts src/lib/shell/WorkspaceShell.svelte.test.ts`: **passed**, four files, 26 tests, exit 0. This includes the empty-inventory, late-defaults, setup retention, popup teardown, Rename teardown, and conversation status regressions. No unhandled errors were reported.
- `go test ./internal/modelalias ./internal/conversation ./internal/host`: **passed**, exit 0; all package results were cached.

During the review, background-tab closing initially omitted native menu publication. The implementation was corrected to publish immediately in `ProjectWorkspace.svelte:156`; that is not a retained finding.

The component checks establish their exercised behavior, not native command operation, relaunch persistence, or inexpensive-model delivery. Those remain acceptance work, not proof supplied by this review.

## Outcome

**material findings remain** — the architecture and scoped replacements are valid; the remaining finding is the explicit rule's application to adjacent workspace state effects, not a foundational design defect.
