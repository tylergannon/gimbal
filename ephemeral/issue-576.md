# Show truthful machine connection progress, consequences and recovery

## Outcome

A person connecting or reconnecting a machine can see what Gimbal actually checked, why it failed or warned, what that means for sessions/workflows, and the available next action. Deliver this independently of session-picker and workflow-view changes.

This replaces the machine connection work bundled into the unaccepted draft #572. Start from current main and selectively reuse useful work. Neither that combined draft nor the other split issues is a prerequisite.

## Scope

Report ordered, live connection checks for existing SSH and daemon-address connections. For SSH these cover sign-in, finding Gimbal, reaching or starting the installed daemon, health, harness availability and build compatibility. Omit checks that do not apply to address or route-backed machines. Support a deliberate Connect anyway decision for nonfatal warnings and meaningful recovery for fatal failures.

Preserve existing configuration, environment inspection, opaque plugin/route settings and retained-session ownership. This issue does not build automatic installation/upgrades, release distribution, a general provisioning/supervision system, or new session/workflow UI. A design suggestion for an installer is not a required capability.

## Architectural guidance from the existing work

- The existing operation path already supplies daemon/helper facts. Build progress from those actual outcomes rather than fabricating intermediate success or introducing a second connection stack.
- A small attempt record associated with a prospective machine identity can expose progress before configuration is saved. Match a warning confirmation to the exact configuration that was checked; edited settings require a fresh check.
- The reference's change notifications avoid a permanent polling loop for idle machine screens. Subscribers should wait for actual progress changes and release their resources when the view closes.
- Separate reachability, harness readiness and build compatibility. A missing harness or different build is not automatically a failed connection; explain which work is affected. Unverified sign-in stays unverified.
- Keep existing system SSH trust and daemon bootstrap behavior. A daemon that can be started is not a fatal failure merely because it was initially absent.
- Label supported actions honestly. Opening Terminal is not an installation or upgrade; the reference's first review caught that misleading label and the later revision corrected it.

## Completeness criteria

- [ ] In the native connection screen, actual checks appear in order as they run and complete. SSH, daemon-address and route-backed paths show only applicable work. A fatal failure stops at its actual failing step with a named cause, consequence and usable next action.
- [ ] A successful check saves the intended machine once. Nonfatal harness/build warnings explain their consequence and offer Connect anyway. Fatal failures cannot be bypassed, and changing connection details invalidates an earlier warning confirmation.
- [ ] Connecting and reconnecting use the real existing transport. Observe SSH access and daemon reachability, an address connection, a fatal access/reachability failure and a nonfatal warning. Identify loopback versus another physical host accurately; fabricated progress does not count as proof.
- [ ] Build mismatch permits supported sessions while explaining the workflow/plugin risk. Missing, signed-out and unverified harnesses are distinguished. Terminal/configuration actions perform exactly what their labels promise, without suggesting automatic installation or upgrade.
- [ ] Existing Git and plain-directory environment inspection and saved configuration still work. Opaque route/plugin fields survive edits, and route checks do not invent SSH or daemon-health results. Existing session ownership and cleanup protections remain intact through lost contact and reconnect.
- [ ] Closing the connection/detail view releases its progress subscription. Idle views do not repeatedly perform checks, and loss of the progress stream is recoverable without silently repeating the underlying connection action. The screen remains usable at default and minimum native window sizes.

Completion evidence must name the final Gimbal revision, actual transport/environment and observed native success/failure/warning cases. Automated checks should cover warning-confirmation validity, applicable check ordering and subscription shutdown; green tests alone do not prove the progress screen. This issue can be accepted independently of the other split issues.

## References

- Design cards at [the design hub](https://design.diffusion.io): connect-machine, connect-checks, machines and the applicable environment-states behavior. Installation/update machinery described as a proposal is outside this issue.
- Reference only: [unaccepted draft #572](https://github.com/tylergannon/gimbal/pull/572), frozen at [e75c6c197e9421da90f9aeac8f64c2edefbaf846](https://github.com/tylergannon/gimbal/tree/e75c6c197e9421da90f9aeac8f64c2edefbaf846).
- Preserved local worktree: /Users/tyler/.codex/worktrees/stage-five-file-reader/gimbal. Relevant areas are environment connection/service operations and the machine connection/detail screens.
- The reference has notification, cancellation, check-order and warning-confirmation coverage. Earlier native address proof used a real loopback daemon; it did not establish second-machine or final native SSH acceptance.

## Related split issues

- [Finish the project file reader and first-message discussion handoff](https://github.com/tylergannon/gimbal/issues/573)
- [Finish session setup, chat commands and workspace read state](https://github.com/tylergannon/gimbal/issues/574)
- [Finish workflow launch, Runs and live agent focus inside the workspace](https://github.com/tylergannon/gimbal/issues/575)
