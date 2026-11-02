# Desktop control interface audit

Research notes, November 1, 2026. **Independently reviewed within the stated scope;
only nitpicks remain.** See `../reviews/native-control-round-07.md`.
Owner: Sol in the native-control feasibility chat. This is a source/configuration
audit, not a native-control success report or a proof of universal impossibility.

## User requirement and conclusion

Tyler accepts a Desktop-dependent route and prefers sharing one application
instance. Before substantial integration work, investigate settings/interfaces
that could make Desktop itself expose a usable control socket, and retain a
reviewed coverage checklist. Do not assume the private worker is necessary.

The installed build has two concrete alternate **client connection** selectors:
`CODEX_APP_SERVER_WS_URL` and `CODEX_APP_SERVER_USE_LOCAL_DAEMON`. Neither inspected
branch opens a listener in Desktop's existing app-server. Its default local
transport launches a child with piped stdio. There is also an authenticated
Desktop app-tools socket, used by the historical Gimbal plugin. Current live
acceptance of a newly bound worker is untested.

No inspected surface yet establishes a supported externally attachable,
Desktop-owned native-control runtime. This is an unresolved feasibility result,
not a declaration that such a route cannot exist.

## Version and inspection boundary

Read-only SSH on Amritanandamayi inspected installed ChatGPT Desktop
26.1021.31345, build 21828, bundle `com.openai.codex`. Its bundled Codex CLI
reports `0.164.0-alpha.11.1`; this differs from the standalone discovery CLI
and the qualification daemon described in earlier notes.

The installed ASAR's main entry and its local-network startup module were
read directly, without modifying the bundle or attaching a debugger. SHA-256:

- `.vite/build/main-0FVbxCQU.js`:
  `fad45aa7d81de6fe166704e8879e3b7cbfc85151e72df7946853336c7cbc5a3c`
- `.vite/build/application-network-startup-B4Wezh-6.js`:
  `9775ead462eb9ab9c31b4a8aedf92b18a5a657f186be21f207a9b7adb143895f`
- `.vite/build/src-Dl21kCYQ.js`:
  `a42052ee3309565865f2d20fc45dee297935cbb50d1fc1a404b90c18d824b93c`

Search inventory: 15,513 JavaScript files, 442,374,290 bytes, under
`.vite/build/` and `webview/assets/`. Search covered server/socket/listen/daemon
environment names; targeted transport identifiers and command/host overrides
were traced in main-process modules. This is lexical coverage, not semantic
review of every byte. Compiled native internals, server-side feature gates,
future builds and unpublished interfaces are outside the certified boundary.

Local uncommitted inspection cache, for independent reproduction:
`/tmp/gimbal-native-control-audit-20261101/`. It contains shipped main/renderer
modules and official documentation pages; it is not maintained product code.
The installed app remains the owning source. No configuration values, tokens
or raw user state were copied into these notes.

## Coverage checklist

“Inspected” means the named source was searched and a relevant branch read.
“Untested” means no configuration was activated or live request made. A negative
search is limited to that source/build, not an absence claim about the product.

| Surface | Inspection and finding | Runtime status / remaining gap |
| --- | --- | --- |
| Official app-server transport docs | Unix/WebSocket listeners and stdio are documented for app-server; Desktop participation in attestation is documented separately | Does not establish Desktop launch/external attachment or native eligibility |
| Official configuration reference | No documented app-server or Desktop-owned listener selector established; other proxy/OAuth/network listener settings exist | Documentation search cannot rule out private options |
| Bundled CLI `app --help` | Workspace launcher, config overrides, feature switches and installer override; no Desktop listener option advertised | Help only; no app launch |
| Bundled CLI `app-server --help` | `--listen` supports stdio, Unix, WebSocket and off; daemon and proxy commands exist | These launch/connect CLI servers; not proof of Desktop-native ownership |
| Default local Desktop launch | Network startup module `Oc` uses fixed code-mode config plus `app-server --analytics-default-enabled`; `_c.spawnProcess` uses piped stdin/stdout/stderr | Existing Desktop child has stdio descriptors; no named app-server listener seen in its scoped Unix FD inventory |
| WebSocket endpoint override | `Ss` chooses `CODEX_APP_SERVER_WS_URL` or host `websocket_url`, unless FORCE_CLI is set | A client selector, not a listener; no override activated |
| Local daemon selector | `vc.connect` selects existing control socket when USE_LOCAL_DAEMON=1, compatible daemon, local host, no config overrides, no CLI override, not Windows | Client attachment only. Control path is a symlink; resolved socket is held by standalone daemon PID 53333. No connection attempted |
| CLI command/runtime override | `Ec`/`Dc` accept host `codex_cli_command`; `CODEX_CLI_PATH` selects executable; `Oc` fixes normal args | Could alter child launch. No supported dual stdio/listener recipe or preserved native trust established; no wrapper introduced |
| Manager location / feature overrides | MANAGER_IN_MAIN is gated by development/unpackaged build. Desktop feature override parser exists; no listener setting established in traced transport branches | Not activated. Remote feature gates not exhaustively knowable |
| SSH / remote-control transports | Main's `--listen unix://` occurrence is in SSH remote app-server bootstrap. Remote-control routes are separate host connections | Does not expose existing Desktop-local child; no bootstrap, pairing or server start performed |
| Existing remote task read/resume/follow-up | Sol observed a read-only app-tool metadata response for the Amrita inspector: the recorded chat ID, remote-control host address and not-loaded status appear in `spec.md`. This observation has not been independently replayed. `send_message_to_thread` is the advertised existing-task follow-up tool | Follow-up execution has not been invoked here. Remote-control host IDs identify the controlling client's address, not proof of Desktop-local ownership on Amrita. Gimbal access to this surface is also unproved. No direct Gimbal-callable endpoint has been established; this interface is proposed only for bootstrap |
| Plugin discovery/reload on resumed tasks | The only traced `config/mcpServer/reload` occurrence in main is gated by internal marketplace `openai-internal-testing`, not the proposed user marketplace. Earlier general-reload inference was overstated | No general user-marketplace refresh trigger established. CLI registration may require a fresh owner task. No Desktop restart or invented reload protocol is authorized; discovery is an early live gate, not an expected pass |
| Official app commands and deep links | Documented chat deep links open/prefill the composer and do not submit automatically. Config/settings/plugin links navigate to UI | No documented control listener or unattended task lifecycle established by these links; none invoked |
| Official public environment variables | Public variable reference inspected in addition to installed private environment-name search | Public list explicitly omits internal variables; no Desktop listener selector established |
| Desktop app-tools IPC | Main owns a private app-tools listener with peer authorization and user-only socket mode; current Desktop FD inventory includes app-tools socket | Exists now; genuine MCP owner binding and task submission still untested |
| App-tools peer authorizer | `vd` selects a native `browser-use-peer-authorization.node` verifier in packaged modes. Binary strings identify audit-token, code-signing and peer/parent/grandparent checks, with rejection categories for ancestry and signing identity | String/static inspection does not establish exact allowed ancestry or acceptance of the proposed MCP Node process; actual admission is an explicit prerequisite |
| Effective app-tools list and dispatch | `tools/list` routes to the live dynamic registry; `tools/call` routes through Desktop's dynamic tool handler. One default MCP configuration enables only create/send/handoff/fork. Separate renderer definitions include task-read and archive names/schemas; main-only absence is not proof those handlers are absent | Actual effective create/read/cancel tools and parameter/result schemas must be verified before creating native work. No creation is allowed if observation and termination routes are unavailable |
| Renderer and owner-turn lifetime | Main's `callDynamicAppTool` routes through ready-window dispatch and `canCallTool` checks; the historical worker caches the binding turn's IDs | Whether a finished owner turn remains admissible, and whether a locked/asleep renderer remains ready, are unresolved. Both are admission tests before any native action |
| Node host-services / browser / single-instance IPC | Distinct listener families found; their existence is not generic app-server control | No direct calls or protocol imitation attempted |
| Desktop client coordination IPC | `src-Dl21kCYQ.js` includes IPC router/client machinery; current Desktop owns `.codex/ipc/ipc.sock` endpoints. `HM` registers owner discovery and follower start/steer/interrupt/history operations; main handles follower start through the existing conversation owner | Credible additional private route for existing loaded tasks. Fresh task creation, admissible external caller, native trust and stability are untested; not generic public app-server attachment |
| macOS launch settings and preferences | Relevant launchctl environment keys unset; preference key names inspected, no listener key found | launchctl environment is not proof of current process environment; no process environment dump taken |
| Persisted Desktop configuration | Relevant config key names and global-state setting key inventory inspected; no listener selector established | User values not published; unknown future/dead code paths remain possible |
| Settings UI implementation | Renderer JavaScript included in lexical search | Actual Settings UI not exercised on remote Mac; no native runtime experiment before approval |
| Existing Gimbal worker | Local README/server/client/library read; worker uses genuine MCP metadata and Desktop app-tools pipe; not a HarnessAdapter | Historical capability only; installed current registration/bind/task lifecycle not proved |
| Saved-app readiness storage | Existing worker reads the Computer Use approvals container on `status`/`start`; reviewer observed that SSH cannot read the existing file despite readable mode bits | Worker-context access remains unknown. Verify only approval/error status through the genuine worker, and stop on denial rather than omitting app requirements or changing protections |
| Current staged installation | Qualification-specific marketplace and worker directories absent; temporary staged directory present; historical default worker socket absent. The separate historical default directory and ledger exist and belong to the old owner | New qualification registration remains uninstalled. Use only explicit isolated `GIMBAL_DESKTOP_DIR`; preserve historical ledger and owner. Prior rejected install is not authorization |
| Explicit scoped worker environment | Read-only inspection of staged marketplace's `plugins/gimbal-codex-desktop/.mcp.json` confirms `mcpServers.gimbal_desktop.env.GIMBAL_DESKTOP_DIR` sets the qualification directory. The source default has only an `env_vars` whitelist | Use the staged scoped registration, not the default. Verify effective installed value before bind; no Desktop environment change or restart required to specify this configuration |
| Documented no-restart project registration alternative | Worker README shows a trusted project `mcp_servers.gimbal_desktop` registration as an alternative to marketplace registration | Its shown whitelist also needs an explicit env map for isolation. This alternative was inspected but is not a second activation path authorized by this gate |

## Trace anchors

Offsets below are Unicode character offsets in decoded UTF-8 module strings,
not byte offsets or stable public symbols. Recompute after any bundle change.

- Network startup: `Ss` ~234800; local daemon selector `vc.connect` ~270000;
  daemon version check `yc` ~271020; runtime and host command selection ~271300–275900;
  `Oc` fixed arguments ~275859.
- Main: private app-tools listener ~338454; host-services listener ~390002;
  SSH server bootstrap `--listen` ~4494910; attestation handler registration
  ~4621601; development-only manager placement ~5032295.
- Shared source: IPC router listener ~743961.
- Shared source: follower/owner registrations `HM` ~755400; main's follower
  start-turn handler ~1000457 and host-manager coordination `Kje` ~1679750.

The bundled CLI's feature list was also inspected. It exposes app-server
WebSocket compression and daemon auto-start features; no listed feature
advertises a Desktop listener. This does not exhaust undocumented native code.
Selected process flags corroborate Desktop child PID 37320 using
`app-server --analytics-default-enabled`, while standalone daemon PID 53333
uses `--listen unix://`. These are distinct existing processes.

App-tools follow-up sources: main `rue` ~340000–342000 dynamically dispatches
calls and list requests rather than implementing every tool inline. Renderer
`thread-handoff-dynamic-tool-progress-39890007cf47.js` defines task read/archive
tool names and schemas. These establish shipped definitions, not live availability.
The cache now includes relevant renderer modules for independent inspection.

## Certification criteria

Independent reviewer must inspect the notes against actual shipped code,
available configuration and local documentation cache; identify missed credible
surfaces and overclaims; record verdict and coverage gaps. A review of this
checklist certifies the stated audit scope, not that every possible interface
in all Codex versions was examined. Configuration candidates remain untested
until explicitly approved experiments. No conclusion may convert “not found”
or a timeout into universal impossibility.

## Sources

- [Official app-server documentation](https://learn.chatgpt.com/docs/app-server)
- [Official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
- [Official developer commands](https://learn.chatgpt.com/docs/developer-commands)
- [Official app commands](https://learn.chatgpt.com/docs/app/commands)
- [Official public environment variables](https://learn.chatgpt.com/docs/config-file/environment-variables)
- Installed bundle `/Applications/ChatGPT.app/Contents/Resources/app.asar`
  on Amritanandamayi; bundled CLI help, preference/configuration key inspection,
  scoped existing-process Unix descriptor inspection.
- Historical and current evidence: `/Users/tyler/.codex/worktrees/release-qualification-design/gimbal-view/NATIVE-COMPUTER-USE.md`
  and its three linked native-control reports.
