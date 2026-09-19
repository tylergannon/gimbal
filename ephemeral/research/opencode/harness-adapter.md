# OpenCode harness adapter: CLI versus managed server

Research for Gimble maintainers · 2026-09-19 · OpenCode **1.18.27**

## Decision

**Prefer a Gimble-managed foreground OpenCode server, accessed through a small explicit Go HTTP/SSE client.** Direct `opencode run` loses capabilities central to `HarnessAdapter`: caller-supplied JSON Schema, a separate active-turn steering control, live native events, and replay. Attaching to an existing server preserves the HTTP capabilities but gives Gimble less authority over version, configuration, execution ownership, and cleanup.

This is a direction to validate, **not a demonstrated complete adapter mapping**. The installed server exposes two API generations with complementary capabilities:

| Surface | What makes it useful | What prevents using it alone |
| --- | --- | --- |
| Legacy `/session` and `/event` | Native fork; schema-bearing synchronous prompt; final assistant/message projections | No distinct steer operation; instance-wide live stream without replay |
| Experimental `/api/session` and `/api/event` | Durable prompt admission, steer/queue distinction, per-session durable history/replay, wait and interrupt | No fork endpoint or caller output-schema field; session stream excludes live deltas and control events |

The first decision gate is whether these generations can operate on **the same native conversation**. In a captured no-model probe, a session created through `/api/session` appeared in its list but legacy `/session?directory=...` returned `[]`. That is concrete evidence against assuming aliases; it does not by itself prove a direct legacy get/fork of that ID fails. A mixed-route adapter is only a candidate until create/read/prompt/fork compatibility is established. A successful probe of one generation cannot establish the other's semantics. Verify both directions by exact ID, then check that legacy schema prompting is visible to v2 history/control and that the legacy fork is readable and independently promptable through the intended family. Different lists alone cannot distinguish storage incompatibility from filtering/projection differences. [Routes][Route-observation][V2-session][Legacy-handler]

The upstream Go SDK **does exist**. Its observed generated coverage omits the operations that would justify this integration: fork, `prompt_async`, schema format, and `/api` routes/session replay. Favor an explicit client on those coverage grounds, not on a claim that the SDK is abandoned. Details and pinned evidence appear below. [Go-readme][Go-session][Go-event]

### Evidence boundaries

This report was synthesized through the supplied semantic index and linked local artifacts. No new OpenCode model turn was run for this report. “Observed” below means a saved installed-binary observation in the corpus; it does not mean the author reran it. The report keeps four levels separate:

- **Observed:** installed version/help, startup/health/auth, generated `/doc`, a stream opening, session create/list, and empty-prompt CLI failure.
- **Pinned source/schema:** released contracts and implementation at the resolved tag; these establish available shapes and source behavior, not live success of every operation.
- **Moving documentation/history:** official pages and issue reports provide context and race examples; they do not establish that an issue reproduces or is fixed in 1.18.27.
- **Unobserved:** model output, schema completion, per-turn accounting, same-active-turn steer landing, live cancellation, permission/question timing, reconnect during inference, and descendant cleanup.

The evidence ledger supplies at least three artifacts for each of the fifteen research topics. Three artifacts are not necessarily three independent experiments: generated types and OpenAPI often derive from the same implementation. No evidence count turns the explicit unknowns into facts.

## 1. Research baseline and Gimble contract

### Product, release, and build identity

The studied executable is `/Users/tyler/.opencode/bin/opencode`, identifying itself as `1.18.27`. Its saved `/global/health` response agrees. The upstream project is [anomalyco/opencode](https://github.com/anomalyco/opencode), with [release v1.18.27](https://github.com/anomalyco/opencode/releases/tag/v1.18.27). The resolved Git tag is:

```text
v1.18.27 → 4b7e19e315cca414121ba1d61523fef74bb3ae8b
```

The saved tag ref and tag package version independently corroborate that mapping. GitHub release metadata separately gives `target_commitish=b04697366f05419e9bd7a92f841813dd976161c9`; that commit's package says 1.18.26. It is **not** the tag source pin and establishes no installed/source mismatch. The compiled version module consumes injected `OPENCODE_VERSION`, with `local` fallback. Neither the binary version output nor health exposes a build SHA, so byte-for-byte provenance to the tag remains unproven. [Version][Health][Tag][Package][Release][Version-source]

The installed and pinned generated OpenAPI documents each contain 162 paths, with OpenAPI `3.1.0`, title `opencode`, and `info.version=1.0.0`. That last value identifies the API document, not the product release or build. Do not use it to reject the installed server as the wrong product version. Current official CLI/server pages are moving documentation; release source and installed help take precedence for this report. [Routes][Installed-schema][CLI-docs][Server-docs]

### Required behavior, rather than adapter precedent

The local contract is [harness.go][Harness], corroborated by [go doc -all .][Godoc], the session runtime, and existing adapters/tests. These are the actual acceptance boundaries:

| Operation/data | Gimble requirement | OpenCode decision consequence |
| --- | --- | --- |
| `CreateSession(ctx, model, effort, workdir)` | Reserve an adapter session ID. Native creation may be lazy. Bind model, requested effort, and workdir. | A returned OpenCode ID is only part of identity; retain server and location too. Empty effort leaves harness defaults. |
| `RunTurn(ctx, sessionID, prompt, schema, onEvent)` | Block for one turn; forward events as they arrive; interrupt native execution on cancellation and return `ctx.Err()`. | Prompt admission, HTTP success, and session idle are different facts. Need native turn correlation and active cancellation. |
| `TurnResult.Output` | With schema: raw structured JSON. Without schema: final message encoded as a JSON string. `Generate` validates/decodes. | JSONL events or JSON-looking prose do not satisfy schema input/output. |
| `TurnResult.Usage` | Optional per-model map reporting this turn's usage/cost. Nil when no harness turn report; Gimble then accounts from step events. | Session cumulative totals are not a turn report; repeated settlement projections must not be added repeatedly. |
| `Steer` | Report whether input landed in the running turn. No active turn or a finish race is `false,nil`; failure to reach harness is an error. | Native admission or promotion alone is not proof of landing. Boundary delivery within the same active `RunTurn` can qualify; mid-token interruption is not required. |
| `Fork` | New native session containing conversation so far, independent afterward. | A later prompt on the same mutable ID is not a fork. |
| `Close` | Idempotently release adapter-held process/subscription/map state when its owning scope ends. | Do not equate cleanup with deleting persisted conversation history. Stop a service only at its ownership scope. |

`AgentEvent` supplies `Type`, `Data`, optional `Metadata`, and `NativeRef`; Gimble assigns event ID/timestamp. The runtime requires `Data` to be a JSON object with a native `sessionID`, rewrites it to a canonical Gimble ID, and canonicalizes relevant message identities. A valid `NativeRef` has a nonempty `provider`, a matching native session identity, and only these allowed fields:

```text
provider, sessionID, turnID, messageID, responseID, itemID,
parentToolUseID, normalizedMessageID, normalizedSessionID
```

**`NativeRef` is not an unrestricted raw-event bag**, despite its `json.RawMessage` type. Unknown keys are rejected. Preserve additional native payload, sequence, location, and event-family data in suitable `Data`/`Metadata`, with compatible IDs in `NativeRef`; do not widen the contract in this research. Global heartbeats and other events without a session are connection diagnostics, not directly valid session `AgentEvent`s. [Events][Session-runtime][Persistence]

Relevant precedents explain flexibility, not additional requirements:

- **Codex:** persistent native thread/app-server; sends raw output schema; steer has an expected turn ID; close removes local routing and archives without starting/restarting a stopped shared daemon. Its tests protect idempotence and no-start cleanup. No harness usage report; step events supply accounting. [Codex][Codex-tests]
- **Claude:** local UUID, one process per turn, `--session-id` then `--resume`, deferred native fork via `--fork-session`, schema and effort per process. Live stream steering; bounded cancellation; final per-model usage. Session close forgets state once the turn process is gone. [Claude][Claude-tests]
- **Antigravity:** lazy native conversation, resumable print process, schema-bearing output and explicit unsupported fork. Steer interrupts/resumes inside the active `RunTurn`; owns a process group and cleans lingering descendants. This demonstrates that landed steering need not be provider mid-token mutation. [Agy][Agy-tests]

Normalized names such as `session.step.*` are Gimble vocabulary. They neither prove that OpenCode emits those names nor that an OpenCode adapter already exists. Model binding chooses harness/native model/effort; it does not make every provider's variant vocabulary interchangeable. [Binding][Harness][Session-runtime]

## 2. Direct CLI: convenient execution, insufficient adapter boundary

### Invocation and input

A representative unattended text invocation is:

```sh
/Users/tyler/.opencode/bin/opencode run \
  --format json --model provider/model --variant low \
  --dir /absolute/project 'Do the requested work'
```

`provider/model` and `low` are illustrative; use an available model and its supported variant. Positional `message..` arguments are joined with spaces, including trailing arguments after `--`. Non-TTY stdin is read **to EOF once**; it becomes prompt text or is appended to the positional prompt with a newline. An open pipe can therefore delay prompt admission. Stdin is not a later permission/question response protocol. Empty noninteractive input without `--command` fails before inference. [Run-help][Run-source][CLI-docs]

Installed help exposes `--command`, `--continue/-c`, `--session/-s`, `--fork`, `--share`, `--model/-m`, `--agent`, `--variant`, `--thinking`, `--format default|json`, `--file/-f`, `--title`, `--attach`, `--password/-p`, `--username/-u`, `--dir`, `--port`, `--interactive`, `--auto`, `--pure`, and logging controls. The parsed port option does not cause ordinary noninteractive `run` to publish a listener. The default path constructs an SDK client for `http://opencode.internal` whose fetch calls `Server.Default().app.fetch(...)`: **an in-process HTTP application, not a spawned `opencode serve`.** `--attach URL` instead uses an existing server. [Run-help][Run-source][Serve-source]

Local `--dir` resolves from the initial root and changes cwd before file resolution. Attached `--dir` denotes the server's directory; local cwd still resolves attachments. Attached regular files become data URLs with a 10 MiB/special-file guard; local directory attachments are rejected because the remote server cannot see them. This distinction matters for reproducible project context and avoiding accidental local/remote path confusion. [Run-source][Run-help][Routes]

### JSON output is a selected JSONL projection

`--format json` selects newline-delimited records, not one JSON result and not a model output schema. This illustrative record is derived from source, not captured inference:

```json
{"type":"text","timestamp":1700000000000,"sessionID":"ses_1","part":{"id":"prt_1","sessionID":"ses_1","messageID":"msg_1","type":"text","text":"done","time":{"start":1,"end":2}}}
```

The outer timestamp is the CLI's `Date.now()`. Records have either `part` or `error`; they do not all contain both. [Run-source][CLI-types][Routes]

| CLI type | Native material selected | Important loss |
| --- | --- | --- |
| `text` | Text part once `time.end` exists | Live deltas and intermediate snapshots |
| `reasoning` | Ended reasoning part, only with `--thinking` | JSON mode alone does not enable reasoning |
| `tool_use` | Tool part in `completed` or `error` state | Pending/running/progress lifecycle |
| `step_start`, `step_finish` | Corresponding native parts | Full message/provider context and unprojected events |
| `error` | Streamed session error or immediate prompt/command error | Preflight errors need not be JSON |

The CLI consumes selected-session `session.status=idle` to stop, but does **not** print that terminal status. There is no `final`, `done`, `turn_end`, or dedicated usage record. `step_finish` may include tokens/cost; the successful prompt response itself is not printed. Raw event IDs, outer project/directory scope, message updates, permission/question events, and most subagent activity are omitted. Child sessions are tracked for permission handling, not exposed as a full child event stream. CLI timestamps cannot reconstruct native ordering or omissions. [Run-source][CLI-types][Event-schema]

The saved empty-input JSON-mode probe exited 1, wrote zero stdout bytes, and printed `Error: You must provide a message or a command` on stderr. Source sets exit code 1 for streamed/immediate errors; normal completion leaves the code unchanged, ordinarily 0. Successful inference, provider-error envelopes in practice, signal-specific exit codes, broken stdin, and timeout/descendant behavior were not observed. A parser must handle process errors without expecting an `error` JSON line. [CLI-empty][CLI-empty-out][CLI-empty-err]

### Continuation, fork, and model selection

`--session ses...` fetches and reuses an explicit persisted session. `--continue` lists sessions and chooses the first without `parentID`; it is a current-instance/list-order heuristic, unsuitable as an adapter's stable handle. With `--fork`, the selected session is forked before prompting; without a session/continue flag, fork is rejected. This supports CLI conversation branching but provides no standalone reservation/fork command that returns an ID without beginning the run flow. No live continuation or forked model conversation was observed. [Run-source][Session-schema][Run-help]

`--model provider/model` splits at the first slash, retaining the rest as model ID. `--agent` selects a primary agent; unknown/subagent selections warn and fall back. `--variant` is the explicit provider-specific effort choice; there is no `--reasoning-effort`. Interactive variant preference precedence—flag, saved choice, history—and `~/.local/state/opencode/model.json` must not be mistaken for a guaranteed noninteractive binding. The noninteractive prompt carries model/variant; whether an override persists into subsequent unspecified prompts is not proven. [Run-source][Variant][Run-help]

Configuration affects both CLI and server. The pinned loader reads global `config.json`, `opencode.json`, `opencode.jsonc`; searches project JSON/JSONC and `.opencode` directories up toward the worktree; includes home `.opencode` and `OPENCODE_CONFIG_DIR`; supports `OPENCODE_CONFIG`, inline `OPENCODE_CONFIG_CONTENT`, `OPENCODE_DISABLE_PROJECT_CONFIG`, and JSON `OPENCODE_PERMISSION`. Managed/account state can also contribute. These are merge/discovery mechanisms, not a fully verified provider-credential precedence table. Discovery may seed global configuration. Bind directory and environment intentionally; do not infer isolation from a session ID alone. [Config][Config-paths][Run-source]

Server Basic-auth credentials authorize HTTP access; they do not provide model-provider credentials. Provider authentication comes from OpenCode's environment, credential stores, and provider/config layers. Exact per-provider precedence remains outside the verified evidence.

### Permissions, questions, cancellation, and ownership

Fresh noninteractive sessions install deny rules for `question`, `plan_enter`, and `plan_exit`. For received `permission.asked`, the CLI replies `once` under `--auto`, otherwise `reject`. The hidden dangerous-skip alias uses the same auto mechanism; it is not proof that explicit denies are bypassed. Stdin has already been consumed. The loop does not implement machine question answering, and unexpected questions bypassing the deny rule remain an untested case. Continuing an existing session must not be assumed to reinstall every fresh-session rule. [Run-source][Run-help][Routes]

`run.ts` provides no caller context, timeout option, separate steer channel, or demonstrated signal-to-native-abort protocol. Terminating an attached CLI may leave remote work alive; terminating an in-process run may still require descendant cleanup. The attached `finish()` path does not await its background event loop, further weakening the CLI as a durable stream supervisor. None of those unknowns is resolved by process exit alone. [Run-source][SDK-launcher][Serve-source]

Direct CLI is reasonable for one-shot text automation. It cannot meet the current complete harness contract by merely parsing JSONL, adding “respond with JSON” to a prompt, or calling a second invocation a landed steer.

## 3. Managed server and HTTP contract

### Launch, readiness, authentication, and lifetime

The studied launch shape is:

```sh
/Users/tyler/.opencode/bin/opencode serve --hostname 127.0.0.1 --port 0
```

`serve` runs in the foreground indefinitely. Help exposes hostname (default `127.0.0.1`), port (default `0`), mDNS/domain, CORS, logging, and `--pure`. No project-directory, config-file, daemon/background, shutdown/restart, or auth-username flags are exposed. Network configuration may supply defaults unless explicitly overridden. Port 0 means **try 4096, then an OS-selected port**, not necessarily a random port immediately. An explicit unavailable nonzero port fails startup. [Serve-help][Serve-source][Server-source]

After binding, stdout prints `opencode server listening on http://<host>:<port>`. Saved observations show that line and HTTP 200 for `/global/health`, `/api/health`, and `/doc`. `/global/health` returns `{healthy:true,version:"1.18.27"}`; `/api/health` returns `{healthy:true}`. Readiness should combine the reported bound address with authenticated health; a printed line alone does not validate credentials or the needed API generation. The JavaScript launcher waits for that line with a default five-second startup timeout, stops its child on startup failure/abort/close, and illustrates process supervision rather than constituting a Go client. [Server-observation][Health][SDK-launcher]

`OPENCODE_SERVER_PASSWORD` enables HTTP Basic auth; username defaults to `opencode` or `OPENCODE_SERVER_USERNAME`. Without a password the command warns and serves unsecured. Captured authenticated `/api/health` succeeds for the configured identity; missing/wrong identity produces 401, `WWW-Authenticate: Basic realm="Secure Area"`, and a typed error. Middleware also accepts query `auth_token`; use the Authorization header. OpenAPI's `security:[]` annotations do not override the observed middleware authentication. CORS is separate. [Auth-observation][Auth-source][Routes]

Location is request/session state, not a `serve` flag. Legacy calls use `directory`/`workspace`; v2 catalog calls accept deep-object `location[directory]`/`location[workspace]` or `x-opencode-directory`/`x-opencode-workspace`. The v2 **body** `LocationRef` is `{directory,workspaceID?}`—do not copy query key `workspace` into it. Default directory is server cwd; session-specific v2 middleware resolves from stored session location. Retain exact resolved location for create, reads, controls, and any cross-generation call. One server can serve multiple directories; that is context routing, not a filesystem sandbox. [Location][Session-location][Routes]

The listener's `stop()` closes its effect scope; forced stop additionally closes HTTP sockets/WebSockets and removes mDNS publication. `serve` does not expose that listener object or a shutdown RPC to its parent. A managed integration must retain process/process-group ownership and bounded shutdown. Listener cleanup is **not evidence** that provider/tool descendants are always reaped. Per-session `Close` releases local resources; shutting a shared server belongs to the server owner's scope, not the first session closed. An attached mode must not stop/restart the server or delete unrelated transcripts. [Server-source][SDK-launcher][Harness]

### Reading the route tables

`sid`, `mid`, `rid` stand for path IDs, not literal strings. Examples use illustrative `ses_`, `msg_`, `per_`, `que_` values. Legacy session/message schemas commonly require prefixes `^ses`/`^msg`; newer message/event schemas use `^msg_`/`^evt_`. Do not invent stricter ID parsing than the selected route requires.

Unless marked otherwise, successful JSON responses are HTTP 200; no-content controls are 204. Common v2 errors are 400 `InvalidRequestError`, 401 `UnauthorizedError`, and typed 404s for absent session/message/request. Session list also has invalid-cursor errors; prompt declares 409 `ConflictError`; wait/compact can report 503 `ServiceUnavailableError`. Legacy routes declare operation-specific 400/404 errors, and model failures can appear inside assistant/error events after HTTP acceptance. These layers must remain distinct. [Routes][Legacy-group][V2-session][V2-handler]

### Legacy sessions, prompting, and discovery

| Method and path | Request | Response and meaning |
| --- | --- | --- |
| `GET /session` | Directory/workspace; optional `scope=project`, `path`, `roots`, `start`, `search`, `limit` | `Session[]`; location-sensitive listing |
| `POST /session` | Optional parent, title, agent, model `{id,providerID,variant?}`, metadata, `workspaceID`, permission rules | `Session`; stable native ID |
| `GET /session/{sid}` | ID plus location routing | `Session` |
| `DELETE /session/{sid}` | ID | Boolean; destructive session removal, not ordinary close |
| `GET /session/{sid}/children` | ID | Child sessions |
| `POST /session/{sid}/fork` | Optional `{messageID}` | New `Session` with own ID and copied conversation; cutoff semantics below |
| `GET /session/{sid}/message` | Optional `limit`, `before` | Array of `{info,parts}`; persisted message projections |
| `GET /session/{sid}/message/{mid}` | Session and message | One `{info,parts}` |
| `POST /session/{sid}/message` | Required `parts`; optional `messageID`, model `{providerID,modelID}`, agent, tools, format, system, variant, `noReply` | Synchronous assistant `{info,parts}`; waits for prompt service |
| `POST /session/{sid}/prompt_async` | Same prompt shape | 204 before completion; background failure later publishes `session.error` |
| `POST /session/{sid}/abort` | No correlation body | Boolean, handler returns true after cancellation; not a landed-steer result |
| `GET /session/status` | Location | Map from session IDs to busy/idle/retry statuses |

A legacy `Session` includes `id`, slug, project/directory, title, version, timestamps, and optional parent/workspace/model/agent/permission/cost/tokens fields. Model creation/reference uses `id`; prompt model selection uses `modelID`. These similarly named shapes are not interchangeable. [Routes][Legacy-group][Legacy-handler]

The fork handler accepts an empty body or `{messageID}` and returns 200 `Session`; malformed input is 400, missing source session 404. The pinned implementation copies messages **before** the selected message, excluding that message. Omitted or unrecognized `messageID` copies all available messages. It allocates new message/part IDs and remaps assistant parent-message IDs; a cutoff is therefore not an inclusive transcript marker. Crucially, `createNext` in this path receives current instance directory/path, original workspace, fork title, and cloned metadata, but **no session `parentID`, model, agent, or permission**. The schema allows parent IDs, yet this fork implementation does not set one. Preserve Gimble's parent relationship separately and verify native settings after fork; issue every fork in the original directory. Do not infer model/permission inheritance or atomic snapshot behavior while the source session is running. [Legacy-handler][Session-publisher][Routes]

Legacy synchronous prompt uses an HTTP stream internally to deliver **one final JSON `{info,parts}` object**, after awaiting the prompt service. It is not SSE token streaming; concurrent events require a separate subscription. Handler-level prompt-service failures map to 400, while assistant errors may remain in the returned message. Message pagination requires `limit` when `before` is supplied, validates the opaque cursor, and returns `Link: ...; rel="next"` plus `X-Next-Cursor` when another page exists; omitted/zero limit takes the unpaginated path. [Legacy-handler][Routes]

The schema-bearing request is concrete:

```json
{
  "messageID": "msg_user1",
  "model": {"providerID":"provider","modelID":"model"},
  "variant": "low",
  "parts": [{"type":"text","text":"Return the requested answer."}],
  "format": {
    "type":"json_schema",
    "schema":{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false},
    "retryCount": 2
  }
}
```

`retryCount` is optional, nonnegative; no default is asserted here. The pinned/installed schema defines optional assistant `structured`, and `StructuredOutputError` with `{message,retries}`. Thus **`response.info.structured` is the schema-defined candidate for `TurnResult.Output`**, while final text remains in parts. This is stronger than saying structured output is absent, but still not a live completion proof. Validate presence, schema conformance, exhaustion behavior, and whether the chosen model/path actually populates it. No dedicated structured-output event need exist for the field to arrive in an assistant message update. [Routes][Installed-schema][Legacy-handler]

The topic evidence contains generated-client drift: the inspected v2 `PromptInput` is text/files/agents only, and a v1 generated request view omitted `format`; the generated release OpenAPI and legacy request handler schema expose it. Preserve the exact route and generator generation in client selection. `run --format json` is unrelated to this prompt-body `format`.

| Discovery/configuration route | Useful response |
| --- | --- |
| `GET /project`, `GET /project/current`, `GET /path` | Known/current project and resolved path context |
| `GET /config`, `PATCH /config` | Effective configuration / configuration mutation |
| `GET /config/providers` | `{providers,default}`; provider catalogs and default model IDs |
| `GET /provider` | `{all,default,connected}` |
| `GET /provider/auth` | Provider-keyed arrays of supported authentication methods |
| `GET /agent` | Agent catalog |
| `GET /api/model`, `GET /api/provider` | `{location,data:[...]}` catalogs at explicit location |
| `GET /api/location` | `LocationInfo` directly, not a `{location,data}` wrapper |

Catalogs expose available models/variants and connected providers, not a guarantee of successful inference or a universal effort mapping. Existing provider authentication should be reused deliberately; broad config mutations are unnecessary for a scoped turn. No `/api/config` replacement appears in the reviewed inventory. [Routes][Provider-group][Config-group][Location]

### V2 durable sessions and controls

| Method and path | Request | Response and contract |
| --- | --- | --- |
| `POST /api/session` | Optional `id`, `agent`, `model:{id,providerID,variant?}`, `location:{directory,workspaceID?}` | `{data:SessionV2Info}` |
| `GET /api/session` | Optional directory, project/subpath, workspace, search, order, limit, opaque cursor | `{data:SessionV2Info[],cursor:{previous?,next?}}`; default newest 50; cursor is not event sequence |
| `GET /api/session/{sid}` | ID | `{data:SessionV2Info}` |
| `GET /api/session/active` | None | `{data:{sid:{type:"running"}}}`; foreground drains owned by this OpenCode process |
| `POST /api/session/{sid}/agent` | `{agent}` | 204; changes subsequent provider turns |
| `POST /api/session/{sid}/model` | `{model:{id,providerID,variant?}}` | 204; changes subsequent provider turns |
| `POST /api/session/{sid}/prompt` | `{id?,prompt:{text,files?,agents?},delivery?:"steer"\|"queue",resume?}` | `{data:SessionInputAdmitted}`; durable acceptance, not final answer |
| `POST /api/session/{sid}/wait` | None | 204 when agent loop idle; 503 if operation unavailable |
| `POST /api/session/{sid}/interrupt` | None | 204; interrupt execution owned by this process; idle is no-op |
| `GET /api/session/{sid}/history` | `after` exclusive nonnegative sequence; `limit` 1–100 | `{data:SessionDurableEvent[],hasMore}`; finite page |
| `GET /api/session/{sid}/event` | Optional `after` | SSE: replay durable session events, then new durable events |
| `GET /api/session/{sid}/message/{mid}` | IDs | `{data:SessionMessage}` projection |
| `GET /api/session/{sid}/context` | ID | `{data:SessionMessage[]}` active context after last compaction |
| `POST /api/session/{sid}/compact` | None | 204; 503 if unavailable |

No v2 fork or session DELETE is defined in the reviewed session group. A schema's optional `parentID` is not evidence of a callable fork operation. `SessionV2Info` carries ID, project, title, location, time, cumulative cost/tokens, and optional parent/model/agent/subpath/revert. The routes are annotated **experimental**, even though shipped in the studied release. [V2-session][V2-handler][Routes]

An admission response has this shape; it returns **`id`**, while event payloads use **`messageID`**:

```json
{"data":{"admittedSeq":12,"id":"msg_user1","sessionID":"ses_1","prompt":{"text":"Do the work"},"delivery":"steer","timeCreated":1700000000000}}
```

Optional `promotedSeq` records promotion. `resume:false` admits without scheduling execution; it supports no-model inspection of durable admission. `prompt` rejects additional properties, so sending `format` inside it cannot smuggle in JSON Schema. File inputs use `uri` with optional name/description/source, rather than legacy file-part syntax. [Routes][V2-session][Input-store]

The API declares 409 for prompt ID conflicts with existing durable records. The linked input store also returns an already admitted input when it finds the ID. Therefore **do not generalize “every repeated POST returns 409,” or assume generic idempotent retry**: pending admission, already-promoted message, differing payload, and cross-session reuse need distinct validation. After a lost response, reconcile the caller ID against durable history before issuing a new ID. [V2-handler][Input-store][Routes]

### Steering, concurrency, and control acknowledgments

The pinned input store separates `promoteSteers` up to an execution cutoff from `promoteNextQueued` in input order. The per-session coordinator starts/joins one execution and coalesces wakes into follow-up work; different session keys can run concurrently. This establishes real admission and scheduling semantics. It does not establish that `promotedSeq` means the provider has consumed the input in the current Gimble turn. [Input-store][Coordinator][V2-handler]

A useful steering proof must associate the admitted ID, promotion/consumption, active execution, and resulting assistant behavior. Delivery at the next model/tool boundary **within the same active `RunTurn`** qualifies; delivery only in a later separately queued turn does not. Keeping a client method artificially blocked across an unrelated later turn is not proof of landing. An admission after a finish race must not leave an unintended future prompt while claiming `false,nil`; handling that race is an unresolved protocol-fit question. Legacy async prompt plus abort is not an established native steer operation.

`wait` is session-idleness, not a final-answer response for one caller ID. Concurrent prompts can join/coalesce at the coordinator, so serializing one Gimble `RunTurn` per native session is the smallest sensible initial constraint. A 204 interrupt says that the serving process handled its own active execution; it neither reports landed steering nor proves cleanup of work owned by another OpenCode process. Attached mode still supports interruption when that same server process owns the turn—the ownership limit is not a blanket ban on remote cancellation. [Coordinator][V2-session][Harness]

### Permission and question APIs

Permissions and questions are request/response controls correlated by request ID; they are not extra prompt text. Pending lists allow recovery when a live notification was missed. The response must target the owning session/request, not whichever turn is most recently visible. [Permission-group][Question-group][Permission-handler][Question-handler][Routes]

| Surface | Exact operation | Body/result |
| --- | --- | --- |
| Legacy permission | `GET /permission` | Pending requests across the resolved instance |
| Legacy permission | `POST /permission/{rid}/reply` | `{reply:"once"\|"always"\|"reject",message?}` → boolean |
| Deprecated legacy response | `POST /session/{sid}/permissions/{rid}` | `{response:"once"\|"always"\|"reject"}` → boolean; current spec does **not** expose `remember` |
| Legacy question | `GET /question`; `POST /question/{rid}/reply`; `POST /question/{rid}/reject` | Pending list; `{answers:string[][]}` → boolean; reject → boolean |
| V2 location lists | `GET /api/permission/request`; `GET /api/question/request` | Location query → `{location,data:[requests]}` |
| V2 permission | `GET /api/session/{sid}/permission`; `GET /api/session/{sid}/permission/{rid}` | `{data:[requests]}` or `{data:request}` |
| V2 permission creation | `POST /api/session/{sid}/permission` | Required `{action,resources}`; optional id/save/metadata/source/agent → `{data:{id,effect}}`; evaluates policy and creates a request when needed |
| V2 permission reply | `POST /api/session/{sid}/permission/{rid}/reply` | `{reply,message?}` → 204 |
| V2 saved rules | `GET /api/permission/saved?projectID=...`; `DELETE /api/permission/saved/{id}` | `{data:[rules]}`; delete → 204 |
| V2 question | `GET /api/session/{sid}/question` | `{data:[requests]}` |
| V2 question reply/reject | `POST /api/session/{sid}/question/{rid}/reply` or `/reject` | `{answers:string[][]}` or no body → 204 |

V2 reply handlers verify ownership; missing/cross-session IDs produce typed 404s (`PermissionNotFoundError`, `QuestionNotFoundError`, or session-not-found). Answers preserve question order; each inner array contains selected labels. Questions include question text/header/options and optional `multiple`/`custom`; control requests may identify `{messageID,callID}`. No observed inference established reply timing, behavior after cancellation, or all races between wait/interrupt/reply. Source/schema/handler agreement supports shapes and checks, not a claim of race-free interaction.

### Go SDK availability and coverage

The caller's 2026-09-19 check pins [anomalyco/opencode-sdk-go](https://github.com/anomalyco/opencode-sdk-go) main at `1f8f6faf2bec1f8c3e606df1af37f301da634914`. The latest observed release is **v0.19.2, published 2025-12-18T04:01:28Z**; the main commit is dated the same day. README says Stainless-generated, Go 1.22+, with import/module **`github.com/sst/opencode-sdk-go`**, confirmed by `go.mod`. Repository owner and module path differ. These observations establish release activity, not formal abandonment or future maintenance policy. [Go-ref][Go-release][Go-commit][Go-readme][Go-module]

Pinned `session.go` implements legacy create/get/list/messages, synchronous `Prompt`, `Abort`, and other older operations. It has no generated fork, `PromptAsync`, schema-format prompt field, or v2 `/api` session operations. `event.go` implements streaming `GET /event`, not durable session `after` replay. The API listing independently describes that older surface. Adopting it would still require bypassing it for core adapter capabilities. [Go-session][Go-event][Go-api]

**Recommendation:** write only the required pinned HTTP calls and SSE decoding explicitly. Current OpenAPI is available if generation later becomes worthwhile, but generated names, auth annotations, two event envelopes, SSE framing, and replay semantics still require inspection. The existing SDK is an available dependency with insufficient demonstrated coverage for this task; a custom client creates a small maintenance obligation, not a claim of upstream support.

## 4. Events, output, usage, and completion

### Streams: scope and delivery are separate choices

| Endpoint | Scope and JSON envelope | Delivery/recovery |
| --- | --- | --- |
| `GET /event?directory=&workspace=` | Legacy instance/project; `{id,type,properties}` | Live-only; initial connected event, 10-second heartbeat; no declared replay cursor |
| `GET /global/event` | Server-wide legacy; `{directory,project?,workspace?,payload:{id,type,properties}}` | Live/global; no proven replay cursor; do not infer identical framing/lifetime details to `/event` |
| `GET /api/event` | Server-wide native `{id,type,data,metadata?,durable?,location?}` | Live-only; connected first, 15-second comment heartbeat; bounded subscriber capacity 256 |
| `GET /api/session/{sid}/event?after=N` | One session's `SessionDurableEvent` | Durable history after exclusive sequence, then new durable events; excludes transient deltas and permission/question/status events outside that union |

All protected streams inherit configured Basic auth. The saved installed `/api/event` observation returned 200 and `text/event-stream`; the deliberate client timeout only establishes an open stream, not inference delivery. [Legacy-events][V2-events][Server-observation][Routes][V2-session]

Legacy `/event` registers its listener before `server.connected`, writes SSE `event: message`, and emits JSON `server.heartbeat` every ten seconds; matching `server.instance.disposed` ends it. V2 `/api/event` also installs its bounded listener before connected, writes `event: message` with JSON data, omits a transport SSE `id`, and merges `: heartbeat` comments every fifteen seconds. The JSON event's `id` is distinct from an SSE `id:` line. No Last-Event-ID protocol is established for either global feed. [Legacy-events][V2-events][Server-docs]

The session SSE route is declared with Effect `StreamSse`: OpenAPI describes `id`, `event`, and string `data` containing a `SessionDurableEvent`, and an `effect/httpapi/stream/failure` failure-event mechanism. Its handler delegates the durable stream to that framework. Do not copy the global handler's connected/heartbeat/framing behavior into this route without a capture. A stream can fail after HTTP 200; transport EOF or a framework failure is not successful turn completion. [Routes][V2-handler][V2-session]

**Complete live observation and durable recovery require different feeds.** A session durable stream can reconstruct settled text/tool/step boundaries, but cannot deliver live text/reasoning/tool-input deltas or permission/question/status notifications absent from its union. Those belong on a live feed filtered by session/location, with pending-control list reconciliation. The catalog includes both native and legacy projections; do not count both as separate work. This is a material cost of choosing v2, not merely a decoder detail. [Event-schema][Routes][V2-events]

### Wire shape: event names do not determine envelope

The same `session.next.step.ended` name can appear as a legacy bus projection under `properties` or as a v2 native event under `data`. Payload fields are **not** all top-level fields beside `type`. The following are schema-derived examples, not live captures; IDs and values are illustrative.

Legacy snapshot:

```json
{"id":"evt_1","type":"message.part.updated","properties":{"sessionID":"ses_1","time":2,"part":{"id":"prt_1","sessionID":"ses_1","messageID":"msg_1","type":"text","text":"done","time":{"start":1,"end":2}}}}
```

Legacy delta is a separate event in the pinned OpenAPI, rather than an optional `delta` on that snapshot:

```json
{"id":"evt_2","type":"message.part.delta","properties":{"sessionID":"ses_1","messageID":"msg_1","partID":"prt_1","field":"text","delta":"done"}}
```

V2 durable settlement, with optional durability/location populated to show replay correlation:

```json
{"id":"evt_3","type":"session.next.step.ended","durable":{"aggregateID":"ses_1","seq":19,"version":2},"location":{"directory":"/project"},"data":{"timestamp":2,"sessionID":"ses_1","assistantMessageID":"msg_1","finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":1,"cache":{"read":2,"write":0}}}}
```

The schema permits optional `metadata`, `location`, and `durable` in native envelopes; the definition determines whether an event is actually persisted. Durable step settlement has schema version 2, while most other durable definitions use version 1. The version is not the product/API generation. The generated spec also contains internal/versioned `sync` shapes; they are not a reason to decode public SSE as a raw storage record. [Routes][Installed-schema][Event-schema][Event-api]

### Reference catalog: messages and parts

For legacy events the table describes `properties`; the native v2 feed may present corresponding payloads under `data`. Optional fields are marked `?`. Generated types are a contract catalog, not proof that every path/provider emits every optional field. [Routes][CLI-types][Session-publisher]

| Event or object | Relevant fields and interpretation |
| --- | --- |
| `session.created`, `.updated`, `.deleted` | `sessionID`, `info:Session`; parent relationship is in `info.parentID` |
| `session.status` | `sessionID`, status `{type:"busy"}`, `{type:"idle"}`, or retry `{type:"retry",attempt,message,next,action?}` |
| `session.idle` | Deprecated standalone idle event; do not create a second completion from it |
| `session.error` | Session ID may be optional in error schema; structured `error`; cannot blindly stamp an unscoped error as a session event |
| `message.updated` | `sessionID`, `info:UserMessage\|AssistantMessage`; replace state for this message |
| `message.removed` | `sessionID`, `messageID`; remove projection |
| User message | ID/session, role, creation time, agent/model; format/variant and other input metadata depend on schema |
| Assistant message | ID/session, role, `parentID` of causing user message, model/provider, agent/mode/path, creation time, optional completion time/error/finish/structured/variant, cost/tokens |
| `message.part.updated` | `sessionID`, complete `part`, `time` in current pinned OpenAPI |
| `message.part.delta` | `sessionID`, `messageID`, `partID`, `field`, `delta` |
| `message.part.removed` | Session/message/part IDs |
| Text part | ID/session/message, `type:"text"`, `text`, optional synthetic/ignored/time/metadata; time when present has start and optional end |
| Reasoning part | Same identity, text, required time with optional end, metadata? |
| Step start/finish parts | Start marks a model step; finish has reason, cost, tokens/cache, optional snapshot |
| Other parts | File, agent, subtask, snapshot, patch, retry, compaction; preserve identity/type without interpreting them as final text |

Tool parts share `id`, `sessionID`, `messageID`, `type:"tool"`, `callID`, `tool`, and a discriminated `state`:

| State | Required core; optional detail |
| --- | --- |
| `pending` | `input` object and raw input string |
| `running` | Input and `time.start`; title/metadata optional |
| `completed` | Input, output string, title, metadata, start/end; optional attachments and compacted time |
| `error` | Input, error string, start/end; optional metadata |

A completed tool is not a completed model turn. A tool-error part can be followed by agent recovery. Pending/running updates are the key observability lost by the CLI. [Routes][CLI-types][Run-source]

### Reference catalog: durable lifecycle

All names below have prefix `session.next.`. Their `data` contains timestamp and session ID. Unless marked live-only, the pinned definition is durable; durability gives replayable facts, not exactly-once callback delivery. [Event-schema][Routes][Event-core]

| Suffix | Additional payload fields | Meaning |
| --- | --- | --- |
| `prompt.admitted`, `prompted` | `messageID`, prompt, delivery | Durable input admission versus prompt promotion/projection; distinguish acknowledgment from execution |
| `agent.switched`, `model.switched` | Message ID and agent/model | Subsequent execution context |
| `moved` | Location, optional subdirectory | Session location transition |
| `context.updated`, `synthetic` | Message ID, text | Context changes; not automatically user-visible final output |
| `step.started` | `assistantMessageID`, agent, model, snapshot? | Provider/model identity for a step |
| `step.ended` | Assistant ID, finish string, cost, full tokens/cache, snapshot?, files? | Settled model step; may be followed by more steps |
| `step.failed` | Assistant ID, structured error | Failed step; not equivalent to global stream failure |
| `text.started` / `text.ended` | Assistant ID, text ID; ended adds full text | Recoverable text boundaries |
| `text.delta` | Assistant ID, text ID, delta | Live-only fragment |
| `reasoning.started` / `.ended` | Assistant ID, reasoning ID, provider metadata?; ended adds text | Recoverable reasoning boundaries |
| `reasoning.delta` | Assistant ID, reasoning ID, delta | Live-only fragment |
| `tool.input.started` / `.ended` | Assistant ID, call ID; started adds name, ended adds raw text | Recoverable tool input boundaries |
| `tool.input.delta` | Assistant ID, call ID, delta | Live-only fragment |
| `tool.called` | Assistant/call IDs, tool, parsed input, provider `{executed,metadata?}` | Native tool call |
| `tool.progress` | Assistant/call IDs, structured state, content array | Durable bounded progress checkpoints, not every stdout chunk |
| `tool.success` | Same IDs, structured/content, provider, outputPaths?, result? | Successful tool completion |
| `tool.failed` | Same IDs, structured error, provider, result? | Tool failure |
| `retried` | Attempt, retry error including message/isRetryable and optional HTTP metadata | Retry lifecycle, not a new successful turn |
| `compaction.started` / `.ended` | Message ID, reason `auto\|manual`; ended adds text/recent | Context compaction; ended is recoverable |
| `compaction.delta` | Message ID, text | Live-only; field is `text`, not `delta` |
| `shell.started` / `.ended` | Call ID; started includes message ID/command, ended includes output | Explicit shell activity |
| `revert.staged` / `.cleared` / `.committed` | Revert state / base fields / message ID | History boundary changes; not new final model output |

The durable session union does **not** contain `session.status`, permission/question events, or these live-only deltas. Seeing them in the larger generated `V2Event` union does not put them in `/api/session/{sid}/event`.

### Permission/question event correlation

Legacy `permission.asked` carries request `id`, session, permission name, patterns, metadata, and optional tool `{messageID,callID}`; `permission.replied` carries session/request ID and reply `once|always|reject`. V2 `permission.v2.asked` instead uses `action`, `resources`, optional save/metadata/source; source can be `{type:"tool",messageID,callID}`. V2 replied uses the same reply choices. [Permission-schema][Routes][Permission-handler]

Legacy `question.asked/replied/rejected` and v2 `question.v2.asked/replied/rejected` correlate by question request ID. Asked contains ordered questions with headers/options and optional multiple/custom; replied has `answers:string[][]`; rejected identifies the request. Example native v2 request:

```json
{"id":"evt_4","type":"permission.v2.asked","data":{"id":"per_1","sessionID":"ses_1","action":"bash","resources":["echo ok"],"source":{"type":"tool","messageID":"msg_1","callID":"call_1"}}}
```

The outer event ID and inner permission ID are different identities. A reply to `per_1` is not a reply to `evt_4`, `call_1`, or the latest user message. Neither an HTTP 204 reply nor a disappearance from the pending list proves the associated model turn succeeded. [Routes][Question-group][Question-handler]

### Output and accounting authority

An OpenCode session contains many inputs, assistant messages, model steps, and tools. The reviewed APIs supply no universal native turn ID equivalent to Gimble's whole `RunTurn`. Reconstruct its boundary from the submitted user/admission ID, assistant parentage or prompt/step sequence, active execution, and final projections. Do not substitute the latest event in the session or the largest timestamp.

For plain text, ended/full text values or final text-part snapshots are the stable result; reasoning and synthetic/ignored text need their own treatment. For legacy schema output, the declared final assistant `structured` field is the candidate. A missing structured result is a failure/unsupported-path condition, not permission to silently return ordinary text. A successful synchronous HTTP response can contain an assistant error. [Routes][Legacy-handler][Harness]

For usage, the best native sources are assistant accounting, legacy `step-finish`, or durable `step.ended`: cost and `tokens.input/output/reasoning/cache.read/cache.write`. Associate model/provider from the assistant or step start. A step settlement is stronger than a partial delta, but **one step is not necessarily the whole turn**. Multiple tool/model steps, retries, and compaction can lie within one `RunTurn`. Account each settled step once, group by native model, and do not add the assistant projection of that same settlement again. Session totals are cumulative and unsafe as an unqualified per-turn report, especially with outside writers. [Routes][Event-schema][Session-publisher][Godoc]

Repeated snapshots replace state; replay repeats existing facts; late corrected projections need reconciliation rather than arithmetic accumulation. Do not infer zero cost from a missing record or recalculate provider pricing as if OpenCode reported it. Cache accounting and whether input includes cached tokens are not universally established by field names. Preserve native components until provider semantics are checked. If no reliable harness turn report is available, the contract allows `Usage:nil` and step-event accounting—but that still requires correct event projection and must avoid double counting. [Harness][Godoc][Event-schema]

Finish strings are open strings in the schema; preserve them. A tool-call finish or completed text part may precede another model step. A terminal success needs current-turn final output, no terminal native error, and a confirmed end of that execution. Idle alone says no work is active, not that the requested work succeeded. A retry or compaction event is not final completion. [Routes][Coordinator][Legacy-handler]

Assistant errors include provider auth, unknown, output-length, aborted, structured-output, context-overflow, content-filter, and API errors. `APIError` carries message/isRetryable and optional status/headers/body/metadata; schema-output exhaustion has retries. Durable step failures use their own structured error schema. The pinned retry implementation has retryable network/provider/rate-limit conditions, jittered backoff, a five-retry cap, and no automatic context-overflow retry. These facts explain busy/retry/idle transitions, not observed provider behavior. Cancellation must still return the caller's `ctx.Err()` rather than a nominally successful partial message. [Routes][Retry][Harness]

### Ordering, replay, and concrete race classes

Use legacy `(sessionID,messageID,partID)` for part state, `callID` for tools, assistant `parentID` for its causing user input, and session `parentID` for forks/child sessions. They are different relationships. For v2, preserve assistant/text/reasoning/call IDs, event ID, and durable `(aggregateID,seq,version)`. Session-list cursors, event sequences, and wall-clock timestamps are different domains. There is no reviewed guarantee of a total order across sessions. [Routes][Event-schema][Event-core]

The event core installs its aggregate subscription before reading history, reads durable rows by sequence, then rereads after each wake. This closes the source-level subscribe/read gap without depending on ephemeral notifications as the durable payload. Its initial sequence is `-1` when `after` is omitted; sequence allocation begins at 0. **Omit `after` for a full initial replay**: `after=0` excludes sequence 0, and the public query does not accept `-1`. The next finite history page uses the last processed durable sequence; `hasMore` describes that page, not permanent session completion. Reconnect using the last **processed durable sequence**, with `after` exclusive; deduplicate by session/aggregate sequence and event ID. Final full-value boundaries reconstruct settled text/reasoning/input after missed fragments. They do not recover every transient delta or exact interleaving with global control events. Global streams have no replay cursor, and bounded subscription does not establish infinite buffering or exactly-once delivery. [Event-core][V2-session][V2-events]

| Failure class | Why it matters | Evidence boundary and response |
| --- | --- | --- |
| Old idle or child idle finishes the new/main turn | Status is session-scoped, not caller-turn-scoped | Historical issue #30043 describes child/main confusion; filter exact session and require current input/execution evidence |
| Replayed full text plus appended deltas duplicates output | A snapshot is replacement state, not another suffix | Schema separates deltas/full values; deduplicate identities and reconcile, rather than concatenate every record |
| Missed events despite healthy connection | Heartbeats only prove transport liveness | Issue #46733 reports 1.18.25 connected/heartbeat-only `/event` and `/global/event`; not proof of the same defect/fix in 1.18.27 |
| HTTP acknowledgment precedes failure | Admission/204 is earlier than model execution | Legacy async handler publishes later errors; v2 admission returns a durable record, not result |
| Reconnect after start or lost final event | Client can mistake an incomplete view for completion | Durable history repairs persisted boundaries; global/legacy streams cannot promise recovery |
| Cancel/reply/prompt race | Reply may target an ended request; a wake can schedule more work | Ownership checks and coordinator exist; full linearization/race outcomes remain unobserved |
| Busy async scheduling loses expected follow-up | A recorded prompt need not prove it executed | Moving-branch issue #46842 is a risk example, not a pinned-release reproduction |

Pinned schema, handlers, and event/coordinator source independently explain these race classes; historical issues add actual reports for particular versions. The record does not contain three independent confirmations of every race in installed 1.18.27. Do not label that missing evidence as successful validation. [Issue-status][Issue-sse][Issue-busy][Legacy-handler][Coordinator][Event-core]

## 5. Capability decision and bounded validation

### Mapping the three options

| Harness concern | Direct CLI | Managed local HTTP server | Attached existing HTTP server |
| --- | --- | --- | --- |
| Session reservation | Lazy implicit creation; ID obtained during invocation | Explicit native create; retain model/effort/location | Same API; also bind URL/auth/version/location |
| `RunTurn` | One process and selected JSONL; no native final envelope | Legacy sync result or v2 admission plus correlated event/projection completion | Same protocol, with outside-writer and owner risks |
| Schema output | No schema input flag; unsupported directly | Legacy format/assistant structured field; v2 lacks input schema | Same if version exposes it |
| Live `AgentEvent` | Omits deltas, progress, controls, native IDs | Native live stream plus durable recovery; normalize to Gimble contract | Same streams; network/lifecycle controlled elsewhere |
| `NativeRef` | Preserve available IDs only; cannot recover omissions | Allowed provider/session/message/item IDs; raw extras in Data/Metadata | Same restrictions |
| Usage | Step-finish data, incomplete context; no native turn report | Unique step/assistant settlement; model-keyed per-turn projection still unproven | Same, cumulative session deltas especially unsafe |
| `Steer` | No separate channel | V2 steer admission is promising; active-turn landing/race unresolved | Same semantics if serving process owns execution |
| `Fork` | Pre-prompt flag; awkward standalone operation | Legacy native fork; absent v2; cross-family gate | Legacy if exposed and authorized |
| Cancellation | Kill CLI; remote/descendant behavior unknown | Interrupt active native work, return context error; own process lifecycle | Interrupt caller's turn when owned by serving process; never kill external server |
| `Close` | End/reap owned turn process and forget local state | Release session resources; stop service only at owner scope; no implicit transcript DELETE | Release local subscription/state only; no takeover/start/stop |

This mapping is grounded in the harness contract, pinned routes/CLI, and generated schema plus observed server behavior. It does not claim that aggregating the available endpoints yields a tested implementation. [Harness][Run-source][Routes][V2-session][Route-observation][Server-observation]

### Material constraints and recommendation gates

1. **Route-generation compatibility comes first.** Prove the same session is addressable across legacy schema/fork and v2 control/event routes. If not, a mixed-route adapter cannot provide the combined contract. A legacy-only adapter would lack demonstrated steering/replay; a v2-only adapter lacks schema input/fork. Report that blocker or explicitly negotiate reduced support rather than conceal it behind local transcript copying.
2. **Full event fidelity needs live and durable channels.** Session durable replay does not replace global/live delta and control observation. Reconcile duplicates and pending requests; reconnect restores durable state, not everything the client missed.
3. **One native session is not a parallel-turn container.** Coordinator joining/coalescing does not provide independent turn results. Keep one active `RunTurn` per native session initially; different sessions may run concurrently. A shared workdir still means shared files.
4. **Steer admission is not landed steering.** Establish same-active-turn consumption and the finish race. A later queued message must neither be counted as landed nor accidentally survive a claimed drop.
5. **Ownership has two levels.** Session cleanup must not delete transcript history; server shutdown belongs only to the process owner. Attached mode cannot guarantee cleanup of an independently owned execution.
6. **Provider evidence remains necessary.** Schema population, effort/variant semantics, cost/cache population, and cancellation final state vary by path/provider. Pin the binary and test the chosen cheapest provider rather than claim universal behavior.

These are compatibility and behavioral gates, not an adapter architecture expansion. Start with the existing library contract and just the required HTTP operations/event projection. The Go SDK's missing core routes makes an explicit client smaller conceptually than combining generated methods with extensive bypasses. Defer general SDK generation, arbitrary server takeover/restart, concurrent turns in one session, and any claim of replaying transient deltas. [Harness][V2-session][Input-store][Coordinator][Go-session][Go-event]

### Small validation sequence

No new inference is required to accept this research. Before implementing/releasing an adapter, use the following bounded observations in order; stop at a contract blocker instead of spending on later probes. Existing captures already support the startup/auth/empty-prompt baseline. Repeat only as needed for the exact implementation under test. Save any future probe output in the supplied `.gimble/research/opencode` area, not tracked research notes.

| Stage | Observation | Pass evidence that changes the decision |
| --- | --- | --- |
| 1 · no model | Launch owned foreground server on loopback/port 0; parse URL; health and `/doc`; repeat with custom Basic-auth user/password | Correct bound address/version; expected 401 versus authenticated success on both families; no assumption that schema version equals product version |
| 2 · no model | Create sessions in two temporary directories; list/get/location/active checks; legacy get/fork of v2 session and reciprocal access | Exact directory/ID separation and concrete acceptance or rejection of the same native conversation across families; this is the first integration gate |
| 3 · no model | Subscribe to live and durable routes, admit valid text with `resume:false`, read history after known sequence; resend caller ID before/after promotion where possible | Actual envelope/framing, admission `id`/event message ID correlation, cursor boundary, duplicate/conflict semantics; do not assume every stream sends connected |
| 4 · cheapest model | One schema-bearing legacy turn with a tiny object schema; capture assistant result, live events, durable/projection settlement and usage | Schema-valid `info.structured`, truthful final error handling, provider/model identity, one-turn accounting; compare v2's different admission behavior without pretending it accepts the schema |
| 5 · cheapest model | Continue same session, fork, then send distinct follow-ups | Stable parent continuation; new native fork containing prior context and independent subsequent histories; separately verified directory/model/variant and Gimble parent linkage |
| 6 · cheapest model | Long enough tool/text turn, cancel context and interrupt; then close with cancellation already active | `RunTurn` returns `context.Canceled`, native activity ends, late settlement counted once, descendants actually exit; attached server remains alive after local close |
| 7 · cheapest model | Steer during active execution and at a finish boundary; compare with explicit queue | Admitted/promoted input consumed within same active turn versus separate later turn; truthful landed/drop result and no unintended queued residue |
| 8 · controlled controls/reconnect | Trigger one permission and one question; reply and race one with cancellation; drop SSE after known durable sequence and reconnect | Correct request ownership/answers, late-reply behavior, pending-list recovery, no duplicate output/usage; full settled output recovered despite missing live deltas |

Use no-model control creation where available before asking a model to trigger a tool. For inference, choose the cheapest configured suitable model and report its exact provider/model/variant; the collected evidence does not select one or authorize a spend requirement. The requested report is research only, so none of the listed future results is claimed here. Existing process/auth tests and the saved observations support the probe order; no green schema or HTTP check substitutes for these runtime outcomes. [Validation-plan][Serve-test][Auth-test][Harness]

## Evidence ledger and local source map

The report's 15 topics are covered below. Each row has at least three linked artifacts; shared schema evidence is intentionally reused instead of duplicating downloads. Original upstream links follow the ledger. All local references remain under the supplied research cache; Gimble snapshots originated at the workspace paths named in the brief. The index was the only research entry point.

| Research topic | Principal artifacts and evidence limits |
| --- | --- |
| 001 · identity | [Installed version][Version], [tag ref][Tag], [tag package][Package], [release metadata][Release], [health][Health]; no installed build SHA |
| 002 · Gimble contract | [Harness][Harness], [Godoc][Godoc], [session runtime][Session-runtime], [Codex tests][Codex-tests]; adapters are precedent |
| 003 · CLI execution/output | [Help][Run-help], [pinned CLI][Run-source], [generated types][CLI-types], [empty-input status][CLI-empty]; no successful model capture |
| 004 · continuation/config | [CLI][Run-source], [session schema][Session-schema], [config loader][Config], [variant logic][Variant], [moving docs][CLI-docs] |
| 005 · CLI controls/ownership | [CLI][Run-source], [serve command][Serve-source], [SDK launcher][SDK-launcher], [help][Run-help]; signals/descendants unobserved |
| 006 · server lifecycle | [Serve help][Serve-help], [listener source][Server-source], [auth middleware][Auth-source], [installed auth][Auth-observation], [pinned docs][Server-docs] |
| 007 · session/model/config routes | [Generated OpenAPI][Routes], [legacy handler][Legacy-handler], [v2 protocol][V2-session], [route observation][Route-observation], [provider group][Provider-group] |
| 008 · prompts/controls/steer | [OpenAPI][Routes], [v2 handler][V2-handler], [input store][Input-store], [permission handler][Permission-handler], [question handler][Question-handler]; active-turn races unobserved |
| 009 · SSE | [Legacy handler][Legacy-events], [v2 handler][V2-events], [session protocol][V2-session], [installed schema][Installed-schema], [event core][Event-core] |
| 010 · event catalog | [Generated types][CLI-types], [durable definitions][Event-schema], [session publisher][Session-publisher], [OpenAPI][Routes], [permission schema][Permission-schema] |
| 011 · output/usage/terminal | [OpenAPI][Routes], [durable definitions][Event-schema], [retry source][Retry], [session publisher][Session-publisher]; live finality/accounting unobserved |
| 012 · ordering/replay/differences | [CLI][Run-source], [event core][Event-core], [session protocol][V2-session], [SSE issue][Issue-sse], [status issue][Issue-status]; issues are version-specific reports |
| 013 · capability comparison | [Harness][Harness], [OpenAPI][Routes], [CLI][Run-source], [v2 protocol][V2-session], [installed session observation][Route-observation] |
| 014 · concurrency/isolation | [Coordinator][Coordinator], [input store][Input-store], [session-location middleware][Session-location], [event core][Event-core], [busy issue][Issue-busy] |
| 015 · recommendation/validation | [Harness][Harness], [serve process test][Serve-test], [authorization test][Auth-test], [installed route observation][Route-observation], [Go session coverage][Go-session] |

Original sources: [OpenCode release](https://github.com/anomalyco/opencode/releases/tag/v1.18.27), [exact source commit](https://github.com/anomalyco/opencode/tree/4b7e19e315cca414121ba1d61523fef74bb3ae8b), [pinned CLI run](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/opencode/src/cli/cmd/run.ts), [serve command](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/opencode/src/cli/cmd/serve.ts), [server listener](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/opencode/src/server/server.ts), [OpenAPI](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/sdk/openapi.json), [v2 session protocol](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/protocol/src/groups/session.ts), [v2 session handler](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/server/src/handlers/session.ts), [durable event definitions](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/schema/src/session-event.ts), [v2 event handler](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/server/src/handlers/event.ts), [legacy event handler](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/opencode/src/server/routes/instance/httpapi/handlers/event.ts), [permission handler](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/server/src/handlers/permission.ts), [question handler](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/server/src/handlers/question.ts), [JS launcher](https://github.com/anomalyco/opencode/blob/v1.18.27/packages/sdk/js/src/v2/server.ts). Current moving documentation: [CLI](https://opencode.ai/docs/cli/), [server](https://opencode.ai/docs/server/). Issue history: [#46733](https://github.com/anomalyco/opencode/issues/46733), [#30043](https://github.com/anomalyco/opencode/issues/30043), [#46842](https://github.com/anomalyco/opencode/issues/46842).

Go SDK originals: [pinned tree](https://github.com/anomalyco/opencode-sdk-go/tree/1f8f6faf2bec1f8c3e606df1af37f301da634914), [session.go](https://github.com/anomalyco/opencode-sdk-go/blob/1f8f6faf2bec1f8c3e606df1af37f301da634914/session.go), [event.go](https://github.com/anomalyco/opencode-sdk-go/blob/1f8f6faf2bec1f8c3e606df1af37f301da634914/event.go), [release v0.19.2](https://github.com/anomalyco/opencode-sdk-go/releases/tag/v0.19.2), [latest-release metadata endpoint](https://api.github.com/repos/anomalyco/opencode-sdk-go/releases/latest), [main-commit metadata endpoint](https://api.github.com/repos/anomalyco/opencode-sdk-go/commits/main). “Latest” describes the caller's saved check, not an ongoing freshness guarantee.

[Version]: ../../../.gimble/research/opencode/topic-001/sources/installed-version.txt
[Health]: ../../../.gimble/research/opencode/topic-001/sources/installed-server-health.json
[Tag]: ../../../.gimble/research/opencode/topic-001/sources/upstream-tag-ref.txt
[Package]: ../../../.gimble/research/opencode/topic-001/sources/tag-opencode-package.json
[Release]: ../../../.gimble/research/opencode/topic-001/sources/release-v1.18.27.json
[Version-source]: ../../../.gimble/research/opencode/topic-001/sources/tag-version-source.ts
[Harness]: ../../../.gimble/research/opencode/topic-002/sources/harness.go
[Godoc]: ../../../.gimble/research/opencode/topic-002/sources/go-doc-all.txt
[Events]: ../../../.gimble/research/opencode/topic-002/sources/events.go
[Session-runtime]: ../../../.gimble/research/opencode/topic-002/sources/session.go
[Persistence]: ../../../.gimble/research/opencode/topic-002/sources/event-persistence.go
[Binding]: ../../../.gimble/research/opencode/topic-002/sources/model-binding.go
[Codex]: ../../../.gimble/research/opencode/topic-002/sources/codex-adapter.go
[Codex-tests]: ../../../.gimble/research/opencode/topic-002/sources/codex-tests.go
[Claude]: ../../../.gimble/research/opencode/topic-002/sources/claude-adapter.go
[Claude-tests]: ../../../.gimble/research/opencode/topic-002/sources/claude-tests.go
[Agy]: ../../../.gimble/research/opencode/topic-002/sources/antigravity-adapter.go
[Agy-tests]: ../../../.gimble/research/opencode/topic-002/sources/antigravity-tests.go
[Run-help]: ../../../.gimble/research/opencode/topic-003/sources/installed-opencode-run-help.txt
[Run-source]: ../../../.gimble/research/opencode/topic-003/sources/upstream-v1.18.27-cli-run.ts
[CLI-types]: ../../../.gimble/research/opencode/topic-003/sources/upstream-v1.18.27-generated-event-types.ts
[CLI-docs]: ../../../.gimble/research/opencode/topic-003/sources/official-cli-docs.html
[CLI-empty]: ../../../.gimble/research/opencode/topic-003/clips/run-format-json-no-message.status
[CLI-empty-out]: ../../../.gimble/research/opencode/topic-003/clips/run-format-json-no-message.stdout
[CLI-empty-err]: ../../../.gimble/research/opencode/topic-003/clips/run-format-json-no-message.stderr
[Session-schema]: ../../../.gimble/research/opencode/topic-004/sources/upstream-v1.18.27-session-schema.ts
[Config]: ../../../.gimble/research/opencode/topic-004/sources/upstream-v1.18.27-config.ts
[Config-paths]: ../../../.gimble/research/opencode/topic-004/sources/upstream-v1.18.27-config-paths.ts
[Variant]: ../../../.gimble/research/opencode/topic-004/sources/upstream-v1.18.27-variant-selection.ts
[SDK-launcher]: ../../../.gimble/research/opencode/topic-005/sources/upstream-v1.18.27-sdk-server.ts
[Serve-help]: ../../../.gimble/research/opencode/topic-006/sources/installed-serve-help.txt
[Serve-source]: ../../../.gimble/research/opencode/topic-006/sources/serve.ts
[Server-source]: ../../../.gimble/research/opencode/topic-006/sources/server.ts
[Server-observation]: ../../../.gimble/research/opencode/topic-006/sources/installed-server-observation.txt
[Auth-observation]: ../../../.gimble/research/opencode/topic-006/sources/installed-auth-observation.txt
[Auth-source]: ../../../.gimble/research/opencode/topic-006/sources/release-authorization-middleware.ts
[Server-docs]: ../../../.gimble/research/opencode/topic-006/sources/release-server-docs.mdx
[Routes]: ../../../.gimble/research/opencode/topic-007/sources/release-sdk-openapi.json
[Installed-schema]: ../../../.gimble/research/opencode/topic-007/sources/installed-openapi.json
[Route-observation]: ../../../.gimble/research/opencode/topic-007/sources/installed-session-route-observation.txt
[Legacy-group]: ../../../.gimble/research/opencode/topic-007/sources/release-session-group.ts
[Legacy-handler]: ../../../.gimble/research/opencode/topic-007/sources/release-session-handler.ts
[Provider-group]: ../../../.gimble/research/opencode/topic-007/sources/release-provider-group.ts
[Config-group]: ../../../.gimble/research/opencode/topic-007/sources/release-config-group.ts
[Location]: ../../../.gimble/research/opencode/topic-007/sources/release-location.ts
[Session-location]: ../../../.gimble/research/opencode/topic-007/sources/release-v2-session-location.ts
[V2-session]: ../../../.gimble/research/opencode/topic-008/sources/release-v2-session-group.ts
[V2-handler]: ../../../.gimble/research/opencode/topic-008/sources/release-v2-session-handler.ts
[Permission-group]: ../../../.gimble/research/opencode/topic-008/sources/release-permission-group.ts
[Question-group]: ../../../.gimble/research/opencode/topic-008/sources/release-question-group.ts
[Permission-handler]: ../../../.gimble/research/opencode/topic-008/sources/release-v2-permission-handler.ts
[Question-handler]: ../../../.gimble/research/opencode/topic-008/sources/release-v2-question-handler.ts
[Legacy-events]: ../../../.gimble/research/opencode/topic-009/sources/release-event-handler.ts
[V2-events]: ../../../.gimble/research/opencode/topic-009/sources/release-v2-event-handler.ts
[Permission-schema]: ../../../.gimble/research/opencode/topic-010/sources/upstream-schema-permission.ts
[Session-publisher]: ../../../.gimble/research/opencode/topic-010/sources/upstream-session-implementation.ts
[Retry]: ../../../.gimble/research/opencode/topic-011/sources/upstream-retry.ts
[Issue-sse]: ../../../.gimble/research/opencode/topic-012/sources/issue-46733-sse.json
[Issue-status]: ../../../.gimble/research/opencode/topic-012/sources/issue-30043-status.json
[Event-schema]: ../../../.gimble/research/opencode/topic-014/sources/opencode-v1.18.27-session-event.ts
[Event-api]: ../../../.gimble/research/opencode/topic-014/sources/opencode-v1.18.27-event-api.ts
[Event-core]: ../../../.gimble/research/opencode/topic-014/sources/opencode-v1.18.27-event.ts
[Input-store]: ../../../.gimble/research/opencode/topic-014/sources/opencode-v1.18.27-input.ts
[Coordinator]: ../../../.gimble/research/opencode/topic-014/sources/opencode-v1.18.27-run-coordinator.ts
[Issue-busy]: ../../../.gimble/research/opencode/topic-014/sources/issue-46842.json
[Validation-plan]: ../../../.gimble/research/opencode/topic-015/INDEX.md
[Serve-test]: ../../../.gimble/research/opencode/topic-015/sources/opencode-v1.18.27-serve-process.test.ts
[Auth-test]: ../../../.gimble/research/opencode/topic-015/sources/opencode-v1.18.27-httpapi-authorization.test.ts
[Go-ref]: ../../../.gimble/research/opencode/go-sdk-check/ref.txt
[Go-release]: ../../../.gimble/research/opencode/go-sdk-check/latest-release.json
[Go-commit]: ../../../.gimble/research/opencode/go-sdk-check/main-commit.json
[Go-readme]: ../../../.gimble/research/opencode/go-sdk-check/README.md
[Go-module]: ../../../.gimble/research/opencode/go-sdk-check/go.mod
[Go-session]: ../../../.gimble/research/opencode/go-sdk-check/session.go
[Go-event]: ../../../.gimble/research/opencode/go-sdk-check/event.go
[Go-api]: ../../../.gimble/research/opencode/go-sdk-check/api.md
