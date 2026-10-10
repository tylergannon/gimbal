# Mixed operating systems and remote computer use

[Back to TL;DR](../proposal.md)

**A workflow may place different tasks on Linux and macOS, including remotely invoked agent tasks that operate a real computer through Claude or ChatGPT sessions.** This is a required backend capability, not merely permission to run an entire workflow on a different host. The central server observes and controls the logical run; the backend chooses and operates execution sites.

## Task requirements, backend placement

Keep three facts distinct: what a task requires, which worker the backend selects, and which agent/desktop session it actually uses. A task may require Linux, or macOS plus a particular agent harness and an available computer-use session. OS alone is insufficient. Requirements may be mapped through simple backend-owned execution profiles or queues; no generic Gimbal scheduler or new public placement API is specified here.

The authored task's requirement must survive compilation into backend metadata. Do not bury it solely in the agent prompt or a hard-coded machine choice. The backend matches it against workers with the required OS/architecture, harness, tools, authenticated session, and desktop access. An unavailable capability is visible as waiting/unavailable or a clear admission error; silently substituting a headless worker or different provider is not valid.

For example, one workflow can build on Linux, hand its artifact to a macOS task using a Claude computer-use session, and return the result to Linux for checks. A task may instead require a ChatGPT session. These are required scenarios, not claims that either integration already works.

## A desktop is a stateful resource

Place the agent execution or its computer-control connection where it can access the intended desktop. Merely running a command over SSH does not establish a usable interactive session. Backend setup must supply the actual login/session access and permissions needed by the chosen integration; Gimbal does not copy credentials between machines to manufacture that access.

Reserve one interactive desktop/session for one controlling task at a time unless the backend provides separate isolated surfaces. Related turns stay attached to the same agent session and desktop while they depend on its state. A lost connection must not trigger blind replay of clicks or create a second controller. Reconnect to the existing task/attempt/session, or report an uncertain outcome and require explicit recovery.

Record the run/task/attempt, worker identity, OS, harness, and opaque session identity in observations so the UI can show where work ran and route steering, interviews, and cancellation. Cancelling a task ends its owned activity and releases its reservation; it must not kill an unrelated user-owned desktop application or shared worker. Existing user sessions are attached resources, not automatically owned subprocesses.

## Candidate macOS backend: a user-session LaunchAgent

A concrete backend can install a launchd LaunchAgent in the intended user's GUI login session. That long-lived worker receives authenticated remote task requests and starts run-owned local execution, or invokes a supported already-running desktop-session integration. Remote submission and desktop execution have different lifetimes: SSH may transport a request or tunnel, but the task is admitted and launched by the resident worker. Keep each run's termination boundary separate from the shared worker.

This is a candidate implementation, not a demonstrated fix for the earlier SSH failure. The October 9 investigation in [Plan initial Voice Notes release](codex://threads/01a11e66-975c-7253-a92e-d3542f6f1e39) recorded SSH-origin `untrusted-process-ancestry` and `cgWindowNotFound`, while [Diagnose Computer Use on Mac](codex://threads/01a121a2-a1f6-7023-90de-8726eee75625) captured Finder successfully from a local desktop task. The original investigation later reported log confirmation of successful local Locked Use. It did not demonstrate a LaunchAgent-launched Gimbal task. A separate disabled-plugin-toggle issue was identified in [Find remote computer use thread](codex://threads/01a1223b-3c68-70a0-8726-e21d48c3d35d); it is not evidence that process authorization is repaired.

Apple distinguishes a user-context LaunchAgent from a system-context LaunchDaemon; use the user-session model for desktop work ([Apple guidance](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/DesigningDaemons.html)). Launchd ancestry or root privileges do not by themselves establish provider authorization, consent, or working desktop access. The backend may need to invoke the provider's supported desktop integration rather than spawn its CLI. Official OpenAI documentation describes remote access using the host's existing Computer Use setup, while SSH starts a remote app server in the login shell ([remote connections](https://learn.chatgpt.com/docs/remote-connections)); neither establishes that an arbitrary LaunchAgent is accepted by the desktop bridge.

The acceptance check is a remotely submitted task, launched through this exact worker path, that performs a real capture and input action through the selected provider session. Test locked operation separately when promised, plus reconnection and cancellation. Advertise the capability only after that path works; installation and permission checkboxes are insufficient. Worker supervision may restart the worker, but must reconcile admitted run identities rather than blindly relaunch uncertain desktop actions.

**Subsequent source/binary research narrows this candidate:** the installed Codex desktop bridge requires a caller descended from the desktop process, plus code-signing checks. A sibling LaunchAgent does not satisfy that rule merely by inhabiting the GUI session. Native capture has additional authorization whose full conditions remain unknown; the observed bridge rejection is not a proven cause of `cgWindowNotFound`. Keep worker placement separate from the provider submission interface; the subsequent live MCP check below narrows its remaining risks. See [the evidence and exact scope](../computer-use/findings.md) and [the independent public-source trace](../computer-use/public-source-report.md).

## Provider submission is the first integration risk

The desired direction is **Gimbal harness → provider desktop session**, potentially through a desktop-installed Gimbal plugin. An already-running agent polling a queue is not the assumed design. Public app-server control exists, but the inspected desktop had no usable public listener; its first-party plugin uses a different private bridge. The [live spike](../computer-use/session-control/live-spike.md) demonstrated ordinary desktop-launched MCP creation/read requests while its owner was idle, plus separate native capture/input and active steering/termination checks. It did not demonstrate a remotely initiated fresh native task without approval. The backend must support **zero per-run approval prompts** after explicit worker provisioning; an interactive app-access request defeats automated execution. The [multi-harness assessment](../computer-use/session-control/assessment.md) distinguishes those observations from packaging, remote ingress, and unattended integration still to prove.

## Cross-machine handoff and builds

Pass inputs and outputs as existing typed values and referenced artifacts; materialize them on the selected worker. A path on Linux has no implied meaning on macOS. Associate executable variants with one workflow source/build identity and the correct OS/architecture, whether built on the destination or supplied as artifacts. One workflow bundle need not mean one executable usable on every machine.

## What backend authors must demonstrate

Run one authored workflow across Linux and macOS with real artifact handoff and a real computer-use action. Demonstrate the Claude and ChatGPT session integrations separately before claiming support for both. Also show unavailable-worker reporting, exclusive desktop access, session continuity after reconnect, and cancellation without harming another run or the user's session. A compiler/linter can check that placement requirements survive lowering; only live execution proves that the selected desktop and provider integration actually work.
