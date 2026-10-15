## Outcome

A person can deliberately choose a session's harness, model, effort, environment and checkout, reuse a previous setup, and understand the workspace's chat state. Deliver these session and workspace controls independently of file, workflow and machine-screen redesigns.

This replaces the session/workspace work bundled into the unaccepted draft #572. Start from current main and selectively reuse useful work. The combined draft and the other split issues are not dependencies.

## Scope

Finish the new-session composer and its harness, environment and worktree pickers. Preserve last-session defaults and recent setup reuse, including explicit effort. Keep unavailable harnesses visible with machine-specific reasons.

Provide Rename and New Session Like through the native menus, truthful command enablement for the selected tab, tab selection/cycling, and Reveal in Finder. Finish the project header, New chat row and bottom Runs/Promises entries, remembered sidebar preference, and unread-reply indicators. The file-specific document-window action belongs to the file issue; run-specific controls belong to the workflow issue.

This issue does not redesign recording or Promises contents, provision machines, change workflow execution, or introduce general multiwindow support. Cross-project unsent draft persistence remains #561. Related workspace-chrome history is recorded in #494; reconcile the affected subset without inheriting its entire historical scope.

## Architectural guidance from the existing work

- Keep session choices with the draft that owns them, distinguishing choices made for different environments and previous setups. Changing selection should not recreate the message form.
- Reuse the admitted environment and observed machine/harness facts. Missing installation, required sign-in and unverified sign-in are distinct states; do not replace observed availability with guesses.
- Persist the selected effort and checkout intent with the conversation so later setup reuse reproduces the original choice. Default effort leaves the decision to the harness. Avoid a second model catalog or a growing parallel provider-policy system where existing knowledge can serve the picker.
- The existing host-owned window state is a useful home for sidebar preference and seen-reply facts. Stable project/conversation and completed-request identities avoid duplicating transcript contents or deriving unread state from timestamps alone.
- Swift should dispatch commands and receive capabilities from the active web view. Each retained view owns its popups and must remove them when inactive without discarding its draft.

## Completeness criteria

- [ ] The native new-session composer exposes harness/model, environment and checkout choices. Missing or signed-out harnesses remain visible, disabled, with the selected machine's actual reason; unverified sign-in is presented as unverified.
- [ ] Last-session defaults, recent Start like choices and native New Session Like reproduce the chosen harness, model, effort, environment and checkout/base intent. Switching pickers, environments or same-project tabs preserves the typed draft and explicit choices.
- [ ] Explicit effort reaches the selected harness and remains attached to subsequent turns; Default leaves it to the harness. Unsupported effort is not offered or silently substituted. Available Git worktrees and required shared-directory behavior are represented accurately.
- [ ] Native Rename updates the saved title, tab and sidebar while preserving session identity and history. Rename, Stop Response, tab selection and cycling operate only on the appropriate active view; a workflow agent cannot acquire top-level chat stop controls.
- [ ] The project header, New chat entry and bottom project tools remain usable at default and minimum window sizes. Reveal in Finder identifies the active local project. An explicit sidebar choice survives resize, project navigation and app relaunch.
- [ ] A completed background chat reply gains an unread indicator; viewing that chat clears it. Read facts remain associated with the correct project and chat across project switching and relaunch, and working/attention states remain distinguishable.
- [ ] Leaving a retained draft or chat removes its open picker/dialog immediately, including through native menu navigation. Returning preserves the unsent message and selected setup rather than reopening stale UI.

Completion evidence must name the final Gimbal revision and show the native setup, command, persistence and state transitions. Use a real inexpensive-model session for delivery observations; component tests alone do not prove native commands or restart behavior. Cross-project draft retention is not claimed unless #561 is separately completed. This issue can be accepted independently of the other split issues.

## References

- Design cards at [the design hub](https://design.diffusion.io): new-chat, session-harness, session-environment, session-worktree, menu-bar, b-sidebar, states and the applicable default-window behavior.
- Existing related work: #494 and #561. Existing recording controls are invariants to preserve, not a new recording task.
- Reference only: [unaccepted draft #572](https://github.com/tylergannon/gimbal/pull/572), frozen at [e75c6c197e9421da90f9aeac8f64c2edefbaf846](https://github.com/tylergannon/gimbal/tree/e75c6c197e9421da90f9aeac8f64c2edefbaf846).
- Preserved local worktree: /Users/tyler/.codex/worktrees/stage-five-file-reader/gimbal. Relevant areas are session setup, harness selection, conversation metadata, host window state, project workspace and the native menu bridge.
- Earlier native observations support Rename and setup reuse. Final-build restart/cross-project unread and sidebar behavior was not fully established.

## Related split issues

- [Finish the project file reader and first-message discussion handoff](https://github.com/tylergannon/gimbal/issues/573)
- [Finish workflow launch, Runs and live agent focus inside the workspace](https://github.com/tylergannon/gimbal/issues/575)
- [Show truthful machine connection progress, consequences and recovery](https://github.com/tylergannon/gimbal/issues/576)
