# Desktop-owned native Mac control: operating recipe

This is the bounded configuration recovered on Amritanandamayi on November 1,
2026. It uses an experimental, private Desktop interface. It is not a supported
public OpenAI integration or a Desktop-independent Gimbal daemon solution.
Revalidate after changing Desktop, its helper, the worker or the controller.

## Components and ownership

| Component | Observed configuration |
| --- | --- |
| Desktop | ChatGPT 26.1021.31345, build 21828, `com.openai.codex` |
| Bundled Codex CLI | `0.164.0-alpha.11.1` |
| Native helper | Build `1001631`; executable SHA-256 `47ad8bdd901e4e55de649c319e8590eda3308f5f69c11d75486e13bd90bedeb7` |
| Existing Gimbal controller | Resource-fix build `cd4ed425c0258d3ba21870e895b1c644f6e80a7a`; owned qualification LaunchAgent, not restarted |
| Desktop worker | `gimbal-codex-desktop@gimbal-native-qualification` 0.1.0; runtime files matched repository revision `8d530da757fe8c2adef1b70517a67a185da8f6ab` |
| Models | Caller selected alias `luna`; actual caller and successful native-task metadata recorded `gpt-6-luna`, effort high. Resolve the alias rather than pinning that observed model in future invocations |

Keep one explicitly owned worker on the existing Amrita Desktop. The worker's
owner is an ordinary genuine Desktop task, not an extra coordinator. Its MCP
process obtains actual executor metadata and the Desktop-supplied app-tools pipe.
The successful owner was the local task titled “Gimbal native-control worker”.
Its binding turn finished before admission and the subsequent native tasks.

The owner must remain loaded and the MCP process alive. A socket or ledger alone
does not establish availability. The tests do not demonstrate automatic owner
recovery, Desktop cold start or behavior after an app update.

## Scoped installation and binding

Use only the owned marketplace directory:

`/Users/tyler/Library/Application Support/Gimbal Qualification/desktop-integration`

The worker state directory is:

`/Users/tyler/Library/Application Support/Gimbal Qualification/desktop-worker`

The scoped plugin registration must **set** that value in
`mcpServers.gimbal_desktop.env.GIMBAL_DESKTOP_DIR`. An `env_vars` whitelist merely
inherits a value; the default repository registration is insufficient by itself.
Verify the effective installed configuration before binding. Preserve the
separate historical `~/.gimbal/codex-desktop` ledger and its owner.

The approved experiment copied the already-staged marketplace after comparing its
runtime bytes to the revision under test, then used normal CLI marketplace and
plugin registration. Installed cache:

`/Users/tyler/.codex/plugins/cache/gimbal-native-qualification/gimbal-codex-desktop/0.1.0`

The resumed inspector did not discover `bind_worker`; one fresh local Desktop
owner did. Invoke `gimbal_desktop.bind_worker` through its actual MCP tool. Do not
manufacture metadata, pipe context, signatures or native authorization. Verify
that its returned socket is in the qualification state directory. No Desktop or
app-server restart was needed. The traced internal-marketplace reload is not a
general user-plugin refresh recipe.

The controlling client's remote-control host address is not the bridge's host
identity. Worker calls run on Amrita's local Desktop; returned child `hostId`
values were `local`. Do not substitute the controlling client's address there.

## Admission before native work

Use the installed worker client with an explicit state directory. Verify saved
Finder approval with `requiredApps: ["com.apple.finder"]`. This succeeds only if
the actual worker can read the protected approval store; do not edit its content,
change protections or omit app requirements to get past an error.

`status` establishes binding and saved approval, not a live bridge request or
native readiness. After the binding turn ends, perform a harmless task's actual
start/read/cancel cycle and confirm unload/no running turn. Check actual active
turn cancellation as well; archiving an already completed task is insufficient.
Compare the resolved Luna model with the host's advertised model choices first.

Use a stable request identity for each logical submission. The same input returns
the same task; changed input with that identity is a conflict. A timeout or unknown
creation outcome is not permission to submit a replacement. Preserve the ledger
and reconcile known tasks. The worker's `read` attempts cancellation when it sees
a task waiting for approval; inspect that outcome rather than assuming termination.
Its `cancel` passes only after observing `notLoaded` and no in-progress turn.

## Start native work through the existing Gimbal controller

Use the already registered `conversation` workflow with role
`agentic-dialogues=luna` and the qualification project/work directory. Its purpose
and human opening message point to the actual local task/request files. Supply
the opening message through the normal Gimbal interaction and Finish after the
owned task has been reconciled. A standalone `run-prompt` is not attachment to
this controller. Do not use the code-review workflow for a native execution task.

Have the Gimbal caller submit exactly one worker request, observe its real child
identity and outcome, and cancel/reconcile that child before finishing. The
experiment used existing controller input machinery without adding a workflow,
restarting the catalog or creating another app-server. No maintained runtime
source was changed and no operating/proof program is committed.

## Native Finder interaction and evidence

Read current native tool documentation and fresh accessibility state. Click the
observed Finder Go menu and Go to Folder item. Verify that the path field has
focus, then type a unique marker **without submitting navigation**. On macOS the
documented call is `app.typeText(text)` at current focus. Supplying the optional
element index fails as Linux-only before input is sent.

Capture an original native screenshot showing the marker, then dismiss the sheet
without navigation and verify the original Finder window remains. On this build,
the observed Close control cleared the path field; there was no Cancel control.
Escape dismissed the sheet. **Both successful interactions used Escape, not a
literal Cancel click.** This is a recorded difference from the approved scripted
procedure; it is not hidden by a claim that the exact procedure passed.

The tool returned screenshots inline. The original JPEG bytes were recovered from
the explicitly owned tasks' retained tool outputs, without recapture, conversion
or editing. Keep originals and provenance outside Git. A missing export option is
not permission to substitute a shell screenshot, reconstruct an image or claim
media that nobody inspected.

Before and after a protected trial, observe the CG session lock indicator; record
IOConsole and display observations separately. Unknown lock state cannot pass.
The two successful trials began while the display was online but inactive/asleep.
Narrow system-log intervals contained authorization ALLOW, successful screen unlock
and direct relock. These are correlated lifecycle observations, not task-tagged
causal traces. Separate keybag telemetry is not a claim about screen-lock success.
No manual unlock or security-setting change was used by the experiment.

## Scope and remaining limits

Two fresh execution tasks under the same loaded owner demonstrated native menu
clicks, focused typing, original screenshots and restored Finder state through
Gimbal, with the cancellation-procedure difference above. The standalone daemon
route remains unqualified. This does not establish unattended reliability across
Desktop restart, owner unload, every idle/screensaver condition or future versions.
The worker is not a full HarnessAdapter.

All five execution requests from the gate were reconciled/cancelled; all three
Gimbal conversations were finished. The one worker remains intentionally bound
and idle. Keep its ownership explicit; never clear a socket before checking its
live owner. Coordinate future native-runtime use and preserve unrelated runner,
cache, signing and service resources. No release or merge action belongs to this
operating recipe.
