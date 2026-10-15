# What permits computer use in a remote task?

[Proposal](../proposal.md) · [Backend placement](../topics/execution-placement.md) · [Public-source trace](public-source-report.md)

## TL;DR

**A GUI-session LaunchAgent is not a demonstrated solution.** In the inspected desktop build, one concrete authorization gate requires the connecting process to descend from the desktop process itself, followed by code-signing identity checks. An independently launched worker does not acquire that trust merely by sharing the user's login session.

There are several distinct gates: exposing the tool to the task, authorizing access to desktop bridges, authorizing native computer-use requests, and obtaining usable access to the intended desktop/window. We traced the first two. The native service has its own authorization machinery, whose complete decision logic remains unestablished. The earlier `cgWindowNotFound` failure is therefore not explained conclusively by the bridge rejection.

**Design consequence:** keep the resident macOS worker as a backend option, but require a demonstrated provider integration before advertising computer-use capability. The subsequent [implemented worker](session-control/implemented-worker.md) demonstrates submission into provider-owned desktop sessions through a private interface. It remains a local experimental integration, not a supported public desktop API. No extra Gimbal service or permission abstraction is justified by this finding.

## Subsequent session-control research

A [Codex, Claude Fable, and agy investigation](session-control/assessment.md) examines the harness-to-desktop direction specifically. The first live attachment check found private stdio connections and a refused connection at the documented public socket path. That investigation led to the [implemented worker](session-control/implemented-worker.md): third-party MCP admission and retained native Computer Use are now demonstrated locally, including two fresh sessions with no observed per-run approval for preapproved Chrome. The earlier ancestry finding must not be enlarged into a claim that all third-party plugin scripts are rejected: the check concerns process and binary identity.

## 1. When is the tool exposed?

The public Codex source controls plugin loading, per-thread plugin selection, MCP server enablement, tool visibility, and action review. Installed/enabled is necessary in those paths, but is not a native permission grant. The independent [public-source report](public-source-report.md) pins every finding to revision `07757f448f2d3cf475d0f328732c1c277bce8aa1`.

The installed desktop adds its own assembly of the unified computer-use surface. Its `ire` function requires the unified plugin to be installed, enabled, and available; the relevant feature flag; runtime paths for Node and the REPL; app-server support for MCP tool exposure; and a non-WSL path. In `ks`, adding the macOS `computer` surface additionally requires Darwin, the computer-use feature, an enabled Computer Use plugin, and a resolved native service application path. These describe this unified path, not every legacy path or platform.

Thus an absent catalog entry, a disabled plugin, or missing runtime support can prevent tool exposure before any native authorization occurs. Conversely, listing the tool does not establish that capture or input will succeed.

## 2. What exactly does the desktop bridge trust?

Read-only disassembly of the installed `browser-use-peer-authorization.node` establishes the following production-path checks in `AuthorizeSocketPeer`:

1. Obtain the socket peer's audit token and PID from the kernel.
2. Walk its parent process chain, bounded to 64 steps. An ancestor must equal the authorizing process's own PID. Otherwise return `untrusted-process-ancestry`.
3. Obtain code identities for the peer, its parent, and its grandparent. Missing identities fail.
4. Require all three identities to have OpenAI's team identifier and an allowed signing identifier. Otherwise return `untrusted-code-signing-identity`.

The team identifier is `2DC432GLL2`. The production signing-identifier allowlist includes the Codex application variants, `com.openai.codex.runtime`, `com.openai.codex.cli`, `codex`, `node`, and `node_repl`. These are code-signing identifiers, not executable filenames. This inspection establishes identity comparisons, not an independent audit of signature validation.

The JavaScript wrapper `cp` loads that module and passes the accepted socket descriptor to it. Call sites include the browser-use pipe and dynamic app-tools pipe; a conditional node-REPL host-services pipe also uses it. The October 9 logs contain the exact ancestry rejection under the browser and dynamic-app-tools components.

**Bounded conclusion:** a separate LaunchAgent's descendants cannot directly pass this ancestry check solely by running in the same GUI session. This is a property of these desktop bridge endpoints in this build. It is not a proof that every supported remote integration fails, or that every native computer-use request traverses these endpoints.

## 3. What remains unknown about native capture?

The separate `SkyComputerUseService` binary contains `ComputerUseIPCSenderAuthorization`, sender/parent/responsible identity fields, and authorization failure names including `untrustedParent` and `relayWithoutTrustedAncestor`. It also contains a distinct lock-screen authorization broker and socket server. These symbols establish additional machinery; strings and exported symbols do not establish its complete algorithm or which branch the failed request took.

The public Codex checkout contains neither this service nor the desktop peer-authorization implementation. Public Locked Use requirements and configuration mapping do not supply the missing native implementation. A separate public native-user-verification API concerns enrollment/challenge signing and must not be mistaken for capture authorization.

Earlier [Voice Notes investigation](codex://threads/01a11e66-975c-7253-a92e-d3542f6f1e39) reports local desktop-origin capture and Locked Use success, with SSH-origin locked capture failing. [Local capture evidence](codex://threads/01a121a2-a1f6-7023-90de-8726eee75625) and the separate [catalog/setup investigation](codex://threads/01a1223b-3c68-70a0-8726-e21d48c3d35d) were read for context. This research did not repeat those actions. The coexistence of `untrusted-process-ancestry` and `cgWindowNotFound` is not yet a causal trace connecting them.

The unresolved question is the supported remote-to-provider-session path that actually performs capture and input, including locked operation if promised. Its readiness check remains the real task through that exact integration, with reconnection and cancellation. Claude requires its own investigation and demonstration; this Codex result says nothing conclusive about Claude's native integration.

## Reproducible evidence locators

Inspected on October 9, 2026, without changing app settings, services, permissions, or desktop state. Downloaded public code was not executed. Extracted application code and disassembly stayed outside the repository.

| Artifact | Version / locator |
| --- | --- |
| Desktop | `/Applications/ChatGPT.app`, version `26.1007.21159`, build `20052` |
| Native bridge authorizer | `Contents/Resources/native/browser-use-peer-authorization.node`; SHA-256 `ec1dde4e429936612dc1f89feb8bd0e2eb42fe55772ba54c7452ebb2452a0790` |
| Authorizer disassembly | `AuthorizeSocketPeer`: audit token/ancestry at `0x21e0–0x2284`; identity acquisition at `0x2354–0x24e8`; allowlist loop/result at `0x26b0–0x29dc` |
| Desktop JavaScript | `Contents/Resources/app.asar` → `.vite/build/main-39FdJ_vp.js`; SHA-256 `78ffc19681d7605d0b48c8cbfdc185211a58d218c4512647de567519dd5ca3bc` |
| JavaScript locators | Zero-based character offsets: `Pa` 99832; `ks` 156542; `ire` 158208; `cp` 304740. Minified function names are build-specific. |
| Native capture service | `/Users/tyler/.codex/computer-use/Codex Computer Use.app`, version `26.1005.1001425`, build `1001425`; executable `Contents/MacOS/SkyComputerUseService`; SHA-256 `ac8bfe406d01a7cb43f7550c080587ae3a5f404f82fa6dcc399bf72a85cd765d` |
| Observed bridge rejection | `~/Library/Logs/com.openai.codex/2026/10/09/codex-desktop-17d6dbc3-569e-459c-80b5-3506ee29f235-2824-t0-i1-000018-0.log`, lines 6568–6569 and 19822–19824 |

These are findings about identified artifacts, not a stable third-party integration contract or a claim that the public revision built the installed app.
