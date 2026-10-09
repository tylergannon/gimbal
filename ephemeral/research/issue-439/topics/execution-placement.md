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

## Cross-machine handoff and builds

Pass inputs and outputs as existing typed values and referenced artifacts; materialize them on the selected worker. A path on Linux has no implied meaning on macOS. Associate executable variants with one workflow source/build identity and the correct OS/architecture, whether built on the destination or supplied as artifacts. One workflow bundle need not mean one executable usable on every machine.

## What backend authors must demonstrate

Run one authored workflow across Linux and macOS with real artifact handoff and a real computer-use action. Demonstrate the Claude and ChatGPT session integrations separately before claiming support for both. Also show unavailable-worker reporting, exclusive desktop access, session continuity after reconnect, and cancellation without harming another run or the user's session. A compiler/linter can check that placement requirements survive lowering; only live execution proves that the selected desktop and provider integration actually work.
