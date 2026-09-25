# OpenCode and Diffusion Router: integration findings

Research date: 2026-09-25. Scope: OpenCode harness only, with Gimbal's checked in proof of concept in `ephemeral/legacy/`. No router credential was used. The local OpenCode binary is **1.2.27**. A local mock OpenAI compatible endpoint was used for turn and resume tests; those tests do **not** prove that Diffusion Router accepts the same requests or tool transcripts.

## Decision

Add OpenCode as a fourth Gimbal harness. The shortest viable adapter is a supervised `opencode run --format json` subprocess for each turn, with a per process inline provider config, `--model diffusion/<router-model-id>`, an explicit `--dir`, and `--session <id>` for continuation. This can use both native OpenCode tools and Diffusion Router's open weight models without changing the user's global OpenCode config. After proving the router's actual tool calling, structured output, interruption, and concurrency behavior, consider a long lived `opencode serve` plus HTTP/SSE adapter to avoid cold starts. OpenCode explicitly documents both scripting mode and attach to a server to avoid repeated MCP startup. [OpenCode CLI](https://opencode.ai/docs/cli/), [OpenCode server](https://opencode.ai/docs/server/).

OpenCode is currently absent from Gimbal's code: `ephemeral/legacy/program/runtime.go` creates only Codex, Claude, and agy adapters; `ephemeral/legacy/internal/modelalias/modelalias.go` routes only `openai`, `anthropic`, and `gemini`; `ephemeral/legacy/harness/contract.go` requires `CreateSession`, `RunTurn`, `Steer`, `Interrupt`, and `Compact`. `Codergen` creates a fresh session per call in `ephemeral/legacy/program/codergen.go`. An OpenCode adapter and model route are therefore both necessary.

## Portal guide defect and tested v1 configuration

The [portal OpenCode guide](https://connect.diffusion.io/connect?guide=opencode) (archived in `portal/coding/opencode.md`, marked beta and reviewed 2026-09-20) names endpoint `https://router.diffusion.io/v1`, environment variable `DIFFUSION_API_KEY`, and model `deepseek-4.1-flash`. It puts model definitions under singular `provider` while using plural `providers` for the npm package, URL, and key. **That combined shape is invalid in installed OpenCode 1.2.27**: passing it through `OPENCODE_CONFIG_CONTENT` to `opencode models diffusion` exits 1 with `Unrecognized key: "providers"`. The official v1 provider schema puts `npm`, `options`, and `models` under one singular `provider.<id>` object. [OpenCode v1 providers](https://opencode.ai/docs/providers/).

Corrected **v1** shape (model ID and URL are supported by the portal guide and its fresh catalog snapshot; a live inference request remains untested):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "diffusion": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Diffusion Router",
      "options": {
        "baseURL": "https://router.diffusion.io/v1",
        "apiKey": "{env:DIFFUSION_API_KEY}"
      },
      "models": {
        "deepseek-4.1-flash": {
          "name": "DeepSeek 4.1 Flash"
        }
      }
    }
  }
}
```

With a dummy key and this config in `OPENCODE_CONFIG_CONTENT`, `opencode models diffusion` returned `diffusion/deepseek-4.1-flash` on 1.2.27. This validates schema parsing and configured model visibility only. The portal's 2026-09-25 fresh catalog snapshot (`portal/catalog-2026-09-25.json`) lists additional coding candidates `glm-5.3` and `glm-5.3-flash`; OpenCode's custom provider shows models explicitly placed in its `models` map, so `opencode models` is not evidence that it dynamically discovered every Diffusion model. The portal's [OpenAI Chat Completions guide](https://connect.diffusion.io/connect?guide=openai-chat-completions) explicitly documents `/v1/chat/completions`, and the v1 OpenCode docs say `@ai-sdk/openai-compatible` targets that API. The portal also documents `/v1/responses`, which would use a different OpenCode package. The docs allow `options.apiKey`, `{env:NAME}` substitution, and model `limit.context`/`limit.output`; do not guess limits or tool capability from a model name. [OpenCode v1 providers](https://opencode.ai/docs/providers/), [OpenCode config](https://opencode.ai/docs/config/).

For Gimbal, construct that JSON in memory for each child process or a short lived config file, and pass the secret only in the child environment. Do not write the key to a tracked file or operation log. `OPENCODE_CONFIG_CONTENT` is an **override**, not a clean profile: OpenCode merges remote, global, project, and inline config, retaining nonconflicting settings. `OPENCODE_CONFIG` loads before project config, while `OPENCODE_CONFIG_DIR` is for agents, commands, modes, and plugins. Thus an isolated child process gives an independent provider override but still inherits the user's other OpenCode settings, tools, and plugins. If true full isolation is required, separately test XDG config/data home isolation; the OpenCode docs do not promise it through these three variables. [OpenCode config](https://opencode.ai/docs/config/).

## Turn transport choices

| Path | Evidence | Gimbal fit |
| --- | --- | --- |
| `opencode run` per turn | `--model provider/model`, `--dir`, `--format json`, `--session`, `--fork`, and `--variant` are documented and available in the installed binary. The JSON stream is newline delimited. [CLI](https://opencode.ai/docs/cli/) | Best first adapter. Process lifetime aligns with one `RunTurn`; context cancellation can terminate the process group. Parse `sessionID` and events; persist the ID for resume. Startup and plugin/MCP initialization repeat. |
| `opencode run --attach` | Official CLI says attach to `opencode serve` to avoid MCP cold starts. [CLI](https://opencode.ai/docs/cli/) | Transitional option: same CLI event parser, shared long lived server. The server's config is fixed at startup, so start one server per configuration profile if profiles differ. |
| `opencode serve` HTTP/SSE | `POST /session`, `POST /session/:id/message` (wait), `POST /session/:id/prompt_async` (204), `GET /event` (SSE), `GET /session/:id/message`, `GET /session/status`, `POST /session/:id/abort`, and `POST /session/:id/summarize` are documented. [Server](https://opencode.ai/docs/server/) | Better lifecycle/event control and lower per turn overhead, but requires event correlation, reconnect/recovery, permission handling, and server supervision. A Go client can use HTTP directly; the official SDK is JS/TS. |

The official SDK starts and owns a server, or connects to an existing one, and exposes `session.create`, `session.prompt`, `session.abort`, `session.summarize`, and `event.subscribe`. Structured JSON output is documented for `session.prompt` via a schema and a model tool with validation retries. **That capability must be tested with each router model**, because open weight model tool call formatting is the likely failure mode. `opencode run` has no documented `--output-schema` flag, so Gimbal's existing exact schema contract cannot be assumed from the CLI; use validation/retry in Gimbal or the server API after testing. [SDK](https://opencode.ai/docs/sdk/), [CLI](https://opencode.ai/docs/cli/).

Local mock proof: `opencode run --format json --model gimble-mock/fake-coder --dir /tmp 'say hello'` emitted `step_start`, `text`, and `step_finish` JSON records with one `sessionID`, final text `mock reply`, and usage counts. A second invocation with `--session <same-id>` emitted the same session ID. The mock received `/v1/chat/completions` requests with provider model ID `fake-coder`. It also received an additional title generation request on the first turn, so account for auxiliary model calls and spend. This is a transport proof, not a router integration proof.

For the CLI adapter, map `text` to assistant events, `reasoning` to thinking, `tool_use` to tool state, and `step_finish` to usage; preserve raw event JSON and unknown types because event sets evolve. Final assistant text should be assembled from completed text parts, avoiding duplicates. Check the terminal event and process exit status, and handle permission prompts and `error` events even if exit status is zero. `Steer` may need queueing until the next turn or server API semantics; do not claim midturn steering until tested. `Compact` maps most directly to server `/session/:id/summarize`; CLI might require a slash command or explicit prompt. [OpenCode CLI](https://opencode.ai/docs/cli/), [OpenCode server](https://opencode.ai/docs/server/).

## Gimbal design changes

1. Add `harness/opencode` and register it in `program.NewRuntime`. Keep the existing neutral five method contract, but capture the OpenCode session ID on the initial turn since CLI `run` creates it as it starts. `CreateSession` can allocate a local placeholder and bind the native ID when `RunTurn` emits its first event, or a server adapter can call `POST /session` immediately.
2. Extend model resolution: current `providerForNative` rejects `deepseek-4.1-flash`, and `providerHarness` has no `diffusion` entry. Preserve a separate configured transport/harness and upstream provider/model ID; avoid assuming every `gpt-*` must use Codex or every router model must be called through one harness. OpenCode selects `diffusion/deepseek-4.1-flash` but sends only `deepseek-4.1-flash` upstream.
3. Scope server pools by config profile (router endpoint, credential identity, model catalog, project config), not by model alone. A single OpenCode server process can serve many sessions for one profile and multiple workspaces, but it cannot be reconfigured per incoming turn merely by setting a caller's environment. Validate the desired workspace on every request; CLI `--dir` already provides it.
4. Treat `ReasoningEffort` carefully. `opencode run --variant` is provider specific; Gimbal's current default `high` is not guaranteed to be a valid variant for a router model. A missing supported variant should mean omission, not an invented flag. Get limits, modalities, reasoning options, and actual model IDs from the router catalog and confirm by live calls.
5. Test real tool roundtrips: read, edit, shell, streamed tool result, long context/compaction, structured output, abort, retry, resume, parallel sessions, and key rotation. Check that router replies conform to OpenAI compatible tool call deltas with IDs, arguments, and corresponding tool messages. A model listing alone is insufficient.

## Version boundary

OpenCode v2 documentation is published separately, and the currently installed binary remains v1.2.27. V2 changes `provider` to `providers`, `npm` to `package`, `options` to `settings`, and calls its server API an intentional breaking change. V2 has a native `@opencode/ai/providers/openai-compatible` package and different JS client/SDK packages. Generate version specific config and protocol handling; never combine keys from both schemas. [V2 migration guide](https://opencode.ai/v2/docs/migrate-v1), [V2 providers](https://opencode.ai/v2/docs/providers), [V2 client](https://opencode.ai/v2/docs/build/client).

## Remaining evidence gap

The live router was not called in this subtask. The portal catalog was freshly retrieved by the parent research task, and its Chat Completions guide is marked "locally-tested," but these are not a successful authenticated turn from Gimbal or OpenCode. Before committing implementation, verify router `/v1/models`, `/v1/chat/completions` response and streaming behavior, model tool use, actual context/output limits, and whether its `/v1/responses` implementation behaves with OpenCode's corresponding package. No unverified numeric model limit appears in the recommended config.
