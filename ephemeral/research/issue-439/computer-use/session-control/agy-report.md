# Externally Controlled Desktop Codex Sessions on macOS (AGY Report)

- **Investigator / Lane**: Antigravity CLI (AGY)
- **Model**: Gemini 3.8 Flash (High)
- **Session ID**: d1a2d5b4-7271-4d41-b7c8-0bd0b8f7f92d
- **Date**: 2026-10-09 (Capture timestamps recorded with UTC offset -06:00)
- **Assignment**: [/Users/tyler/.codex/worktrees/issue-439-proposal/gimbal/ephemeral/research/issue-439/computer-use/session-control/agy-request.md](/Users/tyler/.codex/worktrees/issue-439-proposal/gimbal/ephemeral/research/issue-439/computer-use/session-control/agy-request.md)
- **Raw Cache**: [/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy)
- **Manifest**: [/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/manifest.json](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/manifest.json)
- **Preserved v1 Report**: [/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/report-v1-unvalidated.md](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/report-v1-unvalidated.md)

> Parent audit: the original and first revision are preserved in the external cache. This copy corrects a fabricated packaging quotation, distinguishes the public app-server protocol from internal queue protocol, and fixes local source links. The conclusions below remain bounded source observations.

---

## 1. Executive Summary

This investigation evaluates whether Gimbal can initiate, observe, steer, and cancel desktop Codex sessions with native Computer Use on macOS, specifically assessing a custom Gimbal plugin as the integration boundary.

**Findings**:
1. **Plugin Integration Boundary**: Official plugin specifications document skills, Model Context Protocol (MCP) servers, and lifecycle hooks. None of the inspected public schemas define an inbound API for external harnesses to initiate desktop sessions or inject prompts. The first-party `codex-app-tools` plugin communicates over a dynamic domain socket (`CODEX_APP_TOOLS_PIPE_PATH`), checked by native peer logic `AuthorizeSocketPeer`. Because OpenAI-signed Node can execute third-party JavaScript, signature verification alone does not prove third-party script exclusion, but no public external harness API is documented.
2. **Deep Links / CLI Handoff**: `codex app [PATH]` formats `codex://threads/new?path=<workspace>` via `open -a <app_path>`. The desktop URL router forwards `prompt` to composer prefill state. Traced webview code formats a plugin mention and returns `{ prefillPrompt: ... }`, with no auto-submit handler in the traced path.
3. **Public App-Server vs. Desktop Computer Use**: The public `codex app-server` protocol supports programmatic session control (`thread/start`, `turn/start`, `turn/interrupt`). The Python SDK spawns a standalone app-server child over `stdio://`; the TypeScript SDK spawns `codex exec`. See the independently verified [protocol and SDK trace](codex-report.md). While the public repository contains `computer_use` configuration schemas, whether a public app-server instance can drive macOS native desktop Computer Use outside the installed desktop bundle remains unverified in inspected sources.
4. **AppleScript**: `/Applications/ChatGPT.app/Contents/Resources/scripting.sdef` contains only generic Chromium browser definitions (`window`, `tab`), with no Codex or Computer Use verbs.

---

## 2. Bounded Claim Table

| # | Claim | Status | Primary Citation | Quoted / Verified Evidence | Bounded Scope & Uncertainty |
|---|-------|--------|------------------|-----------------------------|-----------------------------|
| **C1** | Inspected plugin docs define skills, MCP servers, and hooks, but no external inbound session creation API. | Bounded Observation | `developers-openai-com-plugins-build-plugins-raw.md` (lines 22–38, 228–260) | The portable layout uses root `plugin.json`; `.codex-plugin/plugin.json` is a compatibility fallback. Skills, MCP connections, and desktop lifecycle hooks are documented. | Limited to inspected developer docs; does not prove impossibility of unannounced features. |
| **C2** | Bundled `codex-app-tools` MCP manifest configures `codex_app` server over dynamic pipe env var. | Verified Source | `codex-app-tools-mcp.json` (lines 2–48) | `mcpServers.codex_app` runs `./scripts/launch_codex_app_tools_mcp` with args `["./server.mjs"]` and `env_vars: ["CODEX_APP_TOOLS_PIPE_PATH", ...]`. | Verifies configuration schema; does not measure live process behavior. |
| **C3** | Desktop app tools socket undergoes native peer check for process hierarchy and team identifier. | Verified Binary Logic | `chatgpt-desktop-main-39FdJ_vp.js` (char offset 304740 `cp`, 308464 `Rae`) | Function `cp(e)` calls `browser-use-peer-authorization.node` -> `AuthorizeSocketPeer(e)`, checking ancestry and Team ID `2DC432GLL2`. | OpenAI-signed Node can run third-party JS. No live socket connection or rejection test was run. |
| **C4** | CLI handoff generates `codex://threads/new?path=...` and invokes macOS `open`. | Verified Source | `codex-cli-desktop-app-mac.rs` (lines 8–9, 108–115, 151–157) | `CODEX_BUNDLE_IDENTIFIER = "com.openai.codex"`. `codex_new_thread_url` serializes `path`. `open_codex_app` calls `open -a <app_path> <url>`. | CLI code handles path serialization; does not append prompt in `codex_new_thread_url`. |
| **C5** | Desktop deep link router extracts query params and forwards `prefillPrompt`. | Verified Source | `chatgpt-desktop-bootstrap-Du6FqCuH.js` (offsets 407366 `FE`, 413123 `HE`, 414744 `qE`, 415164 `JE`); `chatgpt-desktop-main-39FdJ_vp.js` (offset 3427005) | Router extracts `prompt`, `path`, and forwards `{ codexAppMode: t.codexAppMode, focusComposerNonce: Date.now(), prefillPrompt: t.prompt, project: d }`. | Traced routing logic only. Webview auto-submission was not observed in traced prefill code. |
| **C6** | Webview composer prefill asset formats plugin mentions from prompt parameter. | Verified Source | `chatgpt-desktop-composer-prefill-e60a85bec2c2.js` (lines 1–2, function `s`) | Reads `new URLSearchParams(e).get("prompt")`, prepends plugin mention `@...`, and returns `{ prefillPrompt: ..., prefillPromptFormat: "markdown" }`. | Confirms prefill formatting; does not alone establish full desktop new-thread submission pipeline. |
| **C7** | Public App-Server supports programmatic thread/turn lifecycle; SDK spawns dedicated child CLI. | Verified Source | [Codex lane, A and R](codex-report.md); `codex-sdk-python-client.py` (lines 253–278) | Python `CodexClient` runs `[self._codex_path, "app-server", "--listen", "stdio://"]`. Supports `turn/start`, `turn/interrupt`. | Public repository contains `computer_use` config fields, but desktop CUA driver linkage is unverified. |
| **C8** | Computer Use requires desktop app setup and macOS Screen Recording/Accessibility permissions. | Verified Documentation | `developers-openai-com-codex-computer-use.md` (lines 13–16, 52–60) | *"Computer Use in the ChatGPT desktop app is available on macOS and Windows... grant Screen Recording and Accessibility permissions when prompted..."* | Documentation describes desktop UI workflow. Bundle identifier is `com.openai.codex`. |
| **C9** | AppleScript dictionary in ChatGPT.app provides standard Chromium classes, no Codex automation verbs. | Verified Source | `chatgpt-desktop-scripting.sdef` (lines 1–84, 189–205) | Defines Chromium `Standard Suite` and `Chromium Suite` (`window`, `tab`). Zero custom Codex commands. | Bounded to `/Applications/ChatGPT.app/Contents/Resources/scripting.sdef`. |

---

## 3. Analysis by Surface

### 3.1 Codex Plugins & MCP Surface
OpenAI plugin documentation ([developers-openai-com-plugins-build-plugins-raw.md](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/developers-openai-com-plugins-build-plugins-raw.md:22)) describes a portable root `plugin.json` manifest, `skills/`, `mcp.json`, and optional hooks. The `.codex-plugin/plugin.json` and `.mcp.json` layout is a supported compatibility/scaffold format. This distinction corrects the lane’s original invented quotation.

MCP servers ([developers-openai-com-codex-mcp.md](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/developers-openai-com-codex-mcp.md:18)) configure `stdio` or `streamable-http` executables providing tools, resources, and prompts. In inspected materials, MCP servers respond to client requests; no external desktop session initiation API was identified.

The bundled first-party `codex-app-tools` MCP configuration ([codex-app-tools-mcp.json](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/codex-app-tools-mcp.json:2)) declares:
```json
{
  "mcpServers": {
    "codex_app": {
      "args": ["./server.mjs"],
      "command": "./scripts/launch_codex_app_tools_mcp",
      "cwd": ".",
      "tools": { "create_thread": { "approval_mode": "prompt" }, ... },
      "env_vars": ["CODEX_APP_TOOLS_PIPE_PATH", ...]
    }
  }
}
```
In the desktop main bundle ([chatgpt-desktop-main-39FdJ_vp.js](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/chatgpt-desktop-main-39FdJ_vp.js) at char offset 304740), `cp(e)` checks connecting socket peers via `AuthorizeSocketPeer(e)`, inspecting process hierarchy and Apple Team ID `2DC432GLL2`. Because OpenAI-signed binaries can execute external scripts, binary verification does not prove script-origin exclusion. However, no external public harness API was observed, and no live socket connection tests were performed.

### 3.2 Deep Links & CLI Desktop Handoff
In [`codex-cli-desktop-app-mac.rs`](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/codex-cli-desktop-app-mac.rs:102):
- `CODEX_BUNDLE_IDENTIFIER` is `"com.openai.codex"` (line 8).
- `codex_new_thread_url(workspace)` serializes path:
  ```rust
  let mut serializer = url::form_urlencoded::Serializer::new(String::new());
  serializer.append_pair("path", workspace.as_ref());
  format!("codex://threads/new?{query}")
  ```
- `open_codex_app` invokes `/usr/bin/open -a <app_path> <url>`.

In the desktop bootstrap bundle ([`chatgpt-desktop-bootstrap-Du6FqCuH.js`](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/chatgpt-desktop-bootstrap-Du6FqCuH.js)):
- Char offsets 414744 (`qE`) and 415164 (`JE`) parse URL search parameters including `prompt`, `path`, and `projectId`.
- In `main-39FdJ_vp.js` (char offset 3427005), routing dispatches `{ codexAppMode: t.codexAppMode, focusComposerNonce: Date.now(), prefillPrompt: t.prompt, project: d }`.
- In [`chatgpt-desktop-composer-prefill-e60a85bec2c2.js`](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/chatgpt-desktop-composer-prefill-e60a85bec2c2.js:1), function `s` formats a plugin mention from `prompt` into `{ prefillPrompt: ..., prefillPromptFormat: "markdown" }`.

The traced path sets composer prefill text; auto-submission was not observed in these routines.

### 3.3 Public App-Server & Computer Use
The public app-server provides `thread/start`, `turn/start`, and `turn/interrupt`; see [the protocol and implementation trace](codex-report.md). The separately cached `protocol_v1.md` describes internal submission/event queues and should not be cited as the external JSON-RPC contract. Python client ([codex-sdk-python-client.py](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/codex-sdk-python-client.py:253)) spawns `codex app-server --listen stdio://`.

The public repository contains references to `computer_use` configurations (e.g. `openai_codex/generated/v2_all.py` lines 8037, 10875, 11892, 11930; `models-manager/src/model_info_tests.rs` line 81). However, documentation for Computer Use ([developers-openai-com-codex-computer-use.md](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/developers-openai-com-codex-computer-use.md:13)) specifies the ChatGPT desktop application workflow and OS-level Screen Recording/Accessibility permissions. Whether native Computer Use can be operated headlessly via standalone `codex app-server` remains unverified.

---

## 4. Key Sources & Provenance

Primary sources cached at `/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/sources/`. Fingerprints in `/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/manifest.json`.

- `developers-openai-com-plugins-build-plugins-raw.md`: `b2ac6ccf24210cc4cd805e8940ea270f5dbe96a13d9775ae9123326e9c5a9ce4` (2026-10-09T17:47:38-06:00)
- `developers-openai-com-codex-mcp.md`: `66754c287e84f54ab0125bf3cd7e2ad4986a9fd95260dc4e80b0c22276104f53` (2026-10-09T17:48:18-06:00)
- `developers-openai-com-codex-computer-use.md`: `510246898577035259f7aaff40ddc064ce6b3a2ad45039b53424a63838b897aa` (2026-10-09T17:50:22-06:00)
- `codex-app-tools-mcp.json`: `3a44facf512fac70deb827934cf9f976a1768e295ca6928fea989c2dcd092b0b` (2026-10-09T17:50:52-06:00)
- `codex-cli-desktop-app-mac.rs`: `5f08e5e057ade6474c3e5c369c5f7880ba7cba48981ac6f18bf94d80ab01f5ff` (2026-10-09T17:50:52-06:00)
- `chatgpt-desktop-main-39FdJ_vp.js`: `78ffc19681d7605d0b48c8cbfdc185211a58d218c4512647de567519dd5ca3bc` (2026-10-09T17:50:52-06:00)
- `chatgpt-desktop-bootstrap-Du6FqCuH.js`: `7c8665efb642d0683d42bb0c2b4ac6c80bfd3c19eecd0fa0ee5a8a9934fa17c4` (2026-10-09T17:50:52-06:00)
- `chatgpt-desktop-composer-prefill-e60a85bec2c2.js`: `016cb9e39b873a202889bc83b1e45415c50ea0fc3e68605bc38bc8a23c058fe1` (2026-10-09T17:50:52-06:00)
- `chatgpt-desktop-scripting.sdef`: `cd0453c2e166a0b664f8ea1d1ac04b53a54eed14835569b6104a7823dbb950f2` (2026-10-09T17:50:52-06:00)
- `codex-manual.md`: `b4f13975177586c7f8fed475e9ed73ff8e04d1bf759350e24741fc9970822c80` (*Access limitation*: endpoint returned placeholder UI).
