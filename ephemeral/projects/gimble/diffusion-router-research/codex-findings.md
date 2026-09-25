# Codex harness and Diffusion Router research

Research date: 2026-09-25. Local probe: `codex-cli 0.147.0`. The Portal's Codex guide reports a successful short reply on `0.155.0-alpha.16.3` on 2026-09-23; that is a different build. This is design research, not a completed real-router coding-workflow verification.

## Answer to the app-server question

One app-server process does **not** inherently mean one model provider. The current [app-server protocol](https://learn.chatgpt.com/docs/app-server) and a locally generated v2 schema support `modelProvider` on `thread/start`, `thread/resume`, and `thread/fork`. `turn/start` supports `model` and `effort`, but **not** `modelProvider`; `turn/steer` cannot change turn settings. Register Diffusion as a custom provider in app-server's effective user config, then explicitly set `modelProvider: "diffusion"` when creating or resuming a Diffusion thread. Keep OpenAI threads on `modelProvider: "openai"`. This lets a single process host both kinds of thread, subject to an actual concurrent-turn test. [Protocol schema in OpenAI's repository](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartParams.ts); [official app-server lifecycle](https://learn.chatgpt.com/docs/app-server).

I verified the provider boundary with a disposable `CODEX_HOME`, two dummy custom providers, and one `codex app-server --stdio` process. Separate `thread/start` requests returned `modelProvider: "router_a"` and `"router_b"`. A `turn/start` on a dummy provider sent `POST /v1/responses` to a local fake server with the supplied arbitrary model ID and bearer authorization. The fake server returned 400 deliberately; this proves routing and request shape, **not** model inference or a complete agent loop. No real secret or router request was used. A `thread/start` with a per-thread `config.model_providers` map also accepted a dynamic provider, but the generated `thread/resume` schema has no `config` field, so do not rely on that shortcut for Gimbal's fresh-process resume path without an explicit persistence test.

## Current Gimbal code, with status caveat

The task worktree contains a **preserved legacy** Go implementation under `ephemeral/legacy/`; it is not an active root Go module. The root checkout has an uncommitted `api.go` sketch (`Session.Prompt`) and `docs/research.md` design draft, which I only read. The legacy adapter's own comment and code say it creates **one app-server per turn** and resumes the durable thread on later turns: [`adapter.go`](../../../legacy/harness/codex/adapter.go) lines 26-29, 68-107, 303-346. Its `processConfig` defaults to `codex app-server --stdio`, inheriting the parent's environment unless `config.env` is set: [`rpc.go`](../../../legacy/harness/codex/rpc.go) lines 83-87, 314-318. Thus the user's single-global-server concern describes a potential future architecture, not this preserved adapter's current process topology.

The actual constraint in the preserved adapter is **provider blindness**. `HarnessAdapter.CreateSession(model, workdir)` and `RunTurnInput` have no provider/profile identity ([`contract.go`](../../../legacy/harness/contract.go) lines 18-27, 54-58). `CreateSession` sends only `model` to `thread/start`; `open` sends no `modelProvider` to `thread/resume`; `turn/start` sends model and effort only. [`adapter.go`](../../../legacy/harness/codex/adapter.go) lines 68-101, 303-346, 374-393. The alias resolver currently recognizes OpenAI IDs by `gpt-`/`o1`/`o3`/`o4` prefixes and routes those to Codex, so a Diffusion ID such as `glm-5.3-flash` is rejected as an unknown provider unless a router-backed alias/selection route is added ([`modelalias.go`](../../../legacy/internal/modelalias/modelalias.go) lines 62-66, 184-194). A model ID alone cannot unambiguously select the backend.

This legacy adapter already has native `thread/start`, `thread/resume`, `turn/start`, `turn/steer`, `turn/interrupt`, `thread/compact/start`, event projection, and JSON schema output handling. Preserve those benefits unless an end-to-end router test shows a specific incompatibility. `turn/steer` requires an active turn and the expected turn ID; `turn/interrupt` finishes asynchronously. [Official app-server API](https://learn.chatgpt.com/docs/app-server).

## Router configuration and compatibility

The [Portal Codex guide archived in this project](portal/coding/codex.json) specifies:

```toml
[model_providers.diffusion]
name = "Diffusion Router"
base_url = "https://router.diffusion.io/v1"
wire_api = "responses"
env_key = "DIFFUSION_API_KEY"
supports_websockets = false
```

This matches [Codex custom-provider configuration](https://learn.chatgpt.com/docs/config-file/config-advanced#custom-model-providers). Current Codex accepts only `responses` for `wire_api`, so an OpenAI-compatible Chat Completions endpoint by itself does not suffice. The provider's `base_url` is used as a prefix; my local probe observed `/v1/responses` when the prefix ended in `/v1`. `env_key` names an environment variable containing the secret; do not place the key in checked-in TOML. The [config reference](https://learn.chatgpt.com/docs/config-file/config-reference) also supports header maps, command-backed bearer token auth, retry and stream timeouts, and `model_catalog_json` if catalog metadata needs explicit control. Project `.codex/config.toml` is explicitly barred from changing provider credentials or `model_provider`; provider registration belongs in user config or an isolated `CODEX_HOME` ([advanced config](https://learn.chatgpt.com/docs/config-file/config-advanced#project-config-files-codexconfigtoml)). `CODEX_HOME` scopes config, auth, logs, and sessions, and must point to an existing directory ([environment variables](https://learn.chatgpt.com/docs/config-file/environment-variables)).

The Portal's guide labels Codex support **beta**: its 2026-09-23 test got one short Responses reply through `glm-5.3-flash` on CLI `0.155.0-alpha.16.3`. It says native Desktop and full coding runs are unverified; model discovery failed and Codex used fallback metadata; DeepSeek rejected Codex `input_text` content upstream. Treat those as capability-specific facts rather than proof that every router model works with the Codex harness. Run a full tool-use/patch/resume/steer/compact test per candidate model, especially open-weight models.

The Portal recommends a CLI profile (`$CODEX_HOME/diffusion.config.toml` with top-level `model_provider = "diffusion"`, `model = "glm-5.3-flash"`) for interactive CLI. [Codex profile docs](https://learn.chatgpt.com/docs/config-file/config-advanced#profiles) confirm profile layering. For Gimbal's app-server, explicit `modelProvider` is the critical routing field. Profile selection is process-level and is not needed for different providers in concurrent threads. Do not globally replace the user's default provider merely to run Gimbal Diffusion threads. The Portal says Desktop requires a top-level default and restart; that is Desktop UI behavior, separate from this programmatic app-server use case.

## Launch options and tradeoffs

| Approach | Provider choice | Native live controls | Fit for Gimbal |
| --- | --- | --- | --- |
| One persistent app-server | Register providers once, pass `modelProvider` on thread start/resume | Full thread/turn/steer/interrupt/compact/event protocol | Best for high turn volume if concurrency and isolation tests pass. Requires lifecycle, multiplexing, bounded queues, and credential handling. |
| One app-server per session or turn | Register provider in each process's effective config; pass `modelProvider` | Same protocol; turn process can be restarted and `thread/resume` used | Closest to preserved adapter and isolates process crashes. More startup overhead. First thread must survive until first turn if not yet persisted, as current adapter notes. |
| `codex exec` per turn | `--profile` or `-c model_provider=...` / `-m` with configured provider | JSONL event stream and resumable session via `codex exec resume`; no documented interactive `turn/steer` RPC for a running exec | Good fallback for batch jobs. Loses Gimbal's active steer semantics unless extra process/input orchestration is built. |

`codex exec` is officially intended for scripting; `--json` streams JSONL events, `--output-schema` supports structured output, and `codex exec resume <session-id> <prompt>` continues stored runs ([non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode)). The installed CLI help confirms `--profile`, `--model`, and `--config` on `codex exec`; `codex exec resume` exposes `--model`/`--config` and can inherit the top-level CLI profile selector. `--ephemeral` deliberately discards durable session rollout, so it conflicts with resume. The app-server already gives Gimbal richer live control and explicit provider selection, so switching to pure CLI should be justified by measured compatibility or operating cost, not the assumption that a server supports only one profile.

## Proposed Gimbal changes to evaluate, not implemented

1. Add a backend/provider identity to the neutral session creation input and durable session metadata. Keep the visible model ID separate from the harness and from the Codex `modelProvider` ID. For a Diffusion-backed Codex session, record at least harness=`codex`, provider=`diffusion`, model ID, credential source/config identity, and `CODEX_HOME` or equivalent config scope. Avoid treating an `openai` model name prefix as a router decision.
2. Extend the Codex adapter to send `modelProvider` on both `thread/start` and `thread/resume`; keep it stable across turns. If later turn-level provider switches are needed, end the turn and resume/fork under the target provider, then test history compatibility. `turn/start` cannot switch provider.
3. Configure Diffusion without overwriting the user's default Codex provider. Prefer a stable registered custom provider plus explicit per-thread selection. If Gimbal must own provider configuration and secrets independently of personal Codex state, use a dedicated existing `CODEX_HOME` and manage its config/rollout lifecycle; assess the loss of user skills/plugins/auth from that isolation. A single process can host multiple registered providers.
4. Ensure the key reaches the app-server child process from a secure source. The preserved adapter inherits `os.Environ()` by default; an app launched from a desktop environment may not inherit an interactive shell export. Observe the Portal's separate Desktop environment guidance without assuming it applies automatically to Gimbal.
5. Probe the exact Codex build Gimbal will ship. `model/list` exposes the Codex catalog, and router model discovery may fail while direct inference succeeds. Treat catalog display, actual `/responses` reachability, and complete coding-agent behavior as separate checks. Avoid hard-coding `high` effort for every router model until the target model's supported parameters are known; the preserved alias resolver defaults to `high`.
6. Use permission policy appropriate to Gimbal's host; preserved legacy code hardcodes `never` and `danger-full-access` on start/resume/turn. This is a deployment decision and should be explicit when adding external models.

## Focused validation sequence

- With the target CLI build and a test Diffusion key, start one OpenAI thread and one Diffusion thread in the **same app-server**, verify each `thread.modelProvider`, launch turns concurrently, and inspect provider-side routing/usage. Repeat with separate app-server processes and an isolated `CODEX_HOME`.
- On a Diffusion open-weight model, perform a real coding task that invokes shell/file tools, produces a patch, completes, resumes in a fresh process, follows up, steers mid-turn, interrupts, compacts, and emits parseable events. Verify no tool-call-schema, `input_text`, reasoning, output-schema, or WebSocket mismatch. Include `glm-5.3-flash` as the Portal's only documented successful Codex baseline; test other models individually.
- Verify a fake/invalid key fails clearly, and that a missing key does not silently route to built-in OpenAI. Verify model metadata fallback does not conceal an incompatible context length or reasoning setting.
- Measure process startup and per-turn latency for persistent app-server, per-turn app-server, and `codex exec` on identical prompts. Do not choose a subprocess rewrite before this comparison.

## Sources

- [Codex App Server protocol](https://learn.chatgpt.com/docs/app-server)
- [Codex advanced configuration and custom providers](https://learn.chatgpt.com/docs/config-file/config-advanced)
- [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
- [Codex environment variables](https://learn.chatgpt.com/docs/config-file/environment-variables)
- [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode)
- [OpenAI's generated ThreadStartParams source](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/schema/typescript/v2/ThreadStartParams.ts)
- [Portal Codex guide archive](portal/coding/codex.json)
