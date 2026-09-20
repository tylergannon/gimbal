# OpenCode newer API and generated Go client: decision research

Research date: September 19, 2026. Target: OpenCode **1.18.31**, release commit `014614d35b397775e5d397a490fc72368c894ec2`. This report answers the follow-up to the [initial harness research](harness-adapter.md): whether the newer API can satisfy Gimble, which Go generators actually work on OpenCode's specification, and whether generated structs can be retained while building only the SDK methods ourselves.

## Decision

**Prefer the newer `/api` surface and stable oapi-codegen component models, with a small custom HTTP/SSE layer. Do not yet treat a complete new-API-only Gimble adapter as feasible without upstream capability work.** The code-generation problem has a promising bounded solution; the API functionality problem remains the larger risk.

The decisive generator experiment supports the user's previous approach. **oapi-codegen v2.8.0 generated a compiling, 477,364-byte model package from the 243 component schemas used by `/api`, with one naming annotation.** Actual newer `V2Event` and `SessionDurableEvent` payloads round-tripped, and typed `From`/`As` conversions worked. Removing operation paths avoids the broken response wrappers while preserving the reusable component models. This is not a complete SDK: inline request/response types, HTTP methods, SSE framing, and explicit union dispatch remain to be supplied. It is also not schema validation: unknown and missing tags survive because the generated unions preserve raw JSON. [Model trial][oapi-result], [independent evidence review][evidence-review], [actual model tests][model-tests].

No tested generator produced a compiling **complete** SDK for all 51 newer-API paths. Ogen's successful output omits 35 of 58 operations and lacks the needed event unions. OpenAPI Generator's Go client is roughly 6 MB and does not compile. Rebuilding all structs with libopenapi would therefore be premature; use it, if helpful, to inspect the specification and generate the narrower missing SDK surface. [Generator results][ogen-result], [alternative results][alternatives-result], [metric audit][metrics].

The newer API has no native fork and no caller-supplied structured-output schema input. Its `wait` operation deliberately returns 503. More significantly, source inspection found unresolved served-runner and tool-registration wiring: a real runner exists, but the evidence does not establish that `opencode serve` binds everything it needs to execute a coding turn. All runtime probes used `resume:false`; **no OpenCode model turn was observed**. Those facts put a small execution-capability investigation ahead of substantial SDK work. [Capability source][session-core], [serve wiring audit][serve-wiring], [tool coverage audit][tools-audit], [no-model transcript][runtime-probe].

## Evidence and reproducibility boundaries

The research used a freshly generated local OpenAPI document, current pinned generator versions, actual generation and isolated Go compile attempts, downloaded released source, a pinned upstream comparison, and bounded no-model server probes. Source inspection, generated-code compilation, semantic probes, and live API observations are distinguished throughout.

| Artifact or tool | Pin and observed scope |
|---|---|
| Installed OpenCode | `1.18.31`; local `opencode generate` exited 0 |
| Released source | `v1.18.31` → `014614d35b397775e5d397a490fc72368c894ec2` |
| Moving upstream snapshot | `fee476bb90043a1012abda156dd9af9e5c71b19d`, 32 commits ahead of release at collection time |
| Specification | OpenAPI `3.1.0`; 162 paths, 188 operations, 472 component schemas |
| Newer API path filter | 51 `/api/` paths, 58 operations; initially retains all 472 schemas |
| Closed model input used by successful trial | 243 component schemas, then **zero paths** to isolate model generation |
| Stable / experimental oapi-codegen | `v2.8.0` / `v0.1.0` |
| ogen | `v1.24.0`, tag commit `0d865e7e568f1b36e5e6788e39aa5cd14e02999f` |
| OpenAPI Generator Go target | `7.25.0`; npm launcher `2.41.0`; Temurin Java 21 |
| Contiamo generator / libopenapi | `v0.19.0` / `v0.38.7` |

The original CLI document is 1,061,286 bytes with SHA-256 `00502bd13e9c86f3ca9e765e99a57e06fa9f434ca16f2a714766d1444f8d37f3`. Retained generator outputs and logs live under `.gimble/research/opencode-next`; they are experimental cache, not proposed application code. The original input was preserved. All transformations described below produced separate files. [Input provenance][input-readme], [spec comparison][spec-provenance].

Some initial research summaries overstated passing outcomes or invented plausible route names. This report uses the final generated artifacts and reviews: `/api/version` and `/api/instance/dispose` do not exist; older passing oapi behavior logs do not establish the later failing full SDK; the alternative-generator size columns require the corrected audit. [Evidence review][evidence-review], [runtime review][runtime-review], [metrics][metrics].

## 1. New API coverage against Gimble

Gimble's actual adapter contract reserves a session with `CreateSession(ctx, model, effort, workdir)`, runs a blocking `RunTurn(ctx, sessionID, prompt, schema, onEvent)`, steers the same active turn, forks a native session with the conversation so far, and closes owned resources idempotently. Model/effort are session-creation inputs; they are not additional `RunTurn` parameters. Cancellation must interrupt native work and return the caller's context error. [HarnessAdapter contract][gimble-contract].

“Observed” below means exercised against the installed server without inference. “Source-supported” means implementation exists in pinned source but was not demonstrated in a served model turn. “Missing” means the required public operation/input was absent from the inspected newer surface, not a prediction about future releases.

| Gimble need | Newer API surface | Evidence and remaining limitation |
|---|---|---|
| Reserve native session | `POST /api/session` | **Observed.** Accepts optional ID, agent, model, and location; returns session under `data`. |
| Bind work directory | `location: {directory, workspaceID?}` | **Source-supported and observed location storage.** Explicit absolute directory avoids dependence on server cwd. |
| Select model / reasoning effort | Creation `model: {providerID,id,variant?}`; `POST /api/session/{sessionID}/model` | **Source-supported.** Variant is available; Gimble effort-to-variant mapping is provider/model specific and unproven. |
| Select agent | Creation `agent`; `POST /api/session/{sessionID}/agent` | **Source-supported.** Switch publishes a durable event. Agent tool/runtime parity is separate. |
| Continue a conversation | Prompt existing native ID; `context`, `message`, and history reads | **Durable storage observed; inference continuity unproven.** |
| Admit a turn | `POST /api/session/{sessionID}/prompt` | **Observed with `resume:false`.** HTTP 200 acknowledges admission, not execution/completion. |
| Run a coding turn to completion | Prompt defaults to waking execution; `/api/session/active`, event/history surfaces (resume is an internal service method, not a separate public route) | **Source-supported runner; served execution unproven.** Runner binding and tool registration require investigation. |
| Blocking completion | `POST /api/session/{sessionID}/wait` | **Unavailable.** Explicit source failure and observed HTTP 503. Adapter must own completion detection if other execution works. |
| Steer active turn | Prompt with `delivery:"steer"` | **Source-supported.** Runner promotes steers inside continuation loop. Admission alone cannot establish `landed=true`. |
| Queue later turn | Prompt with `delivery:"queue"` | **Source-supported.** Outer loop promotes queue after current turn settles. This does not satisfy Gimble `Steer`. |
| Cancel native work | `POST /api/session/{sessionID}/interrupt` | **Idle HTTP 204 observed.** Active model/tool interruption and settlement unproven. Closing HTTP/SSE alone is insufficient. |
| Native fork with existing history | No `/api` fork/branch/clone or create-parent/context input | **Missing.** Context export, fresh prompt, and revert do not provide native independent branching. |
| Caller JSON Schema output | No schema/format field in newer prompt input | **Missing.** Generated `OutputFormat` types do not imply a usable newer prompt capability. |
| Live events and controls | `GET /api/event`; permission/question routes | **Transport observed; execution-dependent events source-supported.** Global live feed has no replay cursor. |
| Durable events/recovery | `GET /api/session/{sessionID}/event?after=…`; `/history` | **Admission replay observed.** Durable records exclude transient deltas and controls. |
| Native per-turn usage | `session.next.step.ended` tokens/cost | **Source-supported.** Aggregate unique steps belonging to the turn; session totals are cumulative. |
| Release session resources | Cancel subscriptions, remove adapter state; owned process lifecycle separately | **Adapter responsibility.** Gimble `Close` does not require transcript deletion or a remote disposal endpoint. |

Sources for the matrix are the [exact newer specification][new-spec], [session service][session-core], [route handlers][session-handler], [runner][runner], [runtime transcript][runtime-probe], and [wiring audit][serve-wiring].

### Prompt admission is a useful protocol, not a completion API

The request accepts `id?`, `prompt`, `delivery?`, and `resume?`. `prompt` contains text and optional file/agent attachments. It does not accept per-request model, JSON Schema, or imported conversation history. The source defaults delivery to steer and wakes execution unless `resume:false`. An admission response contains the input ID, session ID, admitted sequence, delivery, prompt, and creation time; a later durable `session.next.prompted` represents promotion into conversation context. [Prompt schema][prompt-schema], [admission implementation][input-source], [session service][session-core].

The no-model probe established exact sequential duplicate handling: repeating the same ID and payload returned the original `admittedSeq`; changing text under the same ID returned 409. That supports retaining an explicit ID and exact request body across a retry. It does not prove all concurrent or lost-response cases safe. The service can call `execution.wake` after obtaining an admission, so “deduplication never wakes the runner” would overstate the implementation. Never retry an uncertain prompt with a fresh ID merely because the HTTP response was lost. [Observed duplicates][runtime-probe], [prompt implementation][session-core].

Steering needs particular care. Gimble asks whether the message landed **in the active `RunTurn`**, including the race where that turn ends during delivery. The runner's steer/queue separation fits that intention: a steer may be consumed at the next provider/tool boundary within the active turn; mid-token interruption is not required. But HTTP admission is not enough to report success, and promotion into a newly started turn after a race is not the required outcome. The adapter will need correlation against its active turn and observed promotion/settlement. No live race test was performed. [Gimble contract][gimble-contract], [runner loop][runner].

### Native fork and schema output are genuine gaps

The only fork route in the full specification is legacy `POST /session/{sessionID}/fork`. The newer create input has no parent, source-session, imported context, or event-log seed. Newer context/history routes export information; they do not import it into a new native session. Revert changes an existing conversation rather than producing a second independent one. Sending the exported transcript as prompt text loses native message/tool identity and is not Gimble's native fork contract. [Full spec][full-spec], [newer session service][session-core], [fork comparison][fork-comparison].

The legacy request supports `format: {type:"json_schema", schema, retryCount?}`. The newer `PromptInput` offers text/files/agents and no equivalent. A schema named `OutputFormat` being reachable through shared models is not evidence the newer prompt endpoint can use it. Likewise, requesting JSON in prose, validating after the fact, or inventing a synthetic output tool is not demonstrated native schema enforcement. The runner lists structured-output tool definitions among unfinished work; this investigation did not find a public newer operation that closes the gap. [Prompt schema][prompt-schema], [comparison][fork-comparison], [runner TODO and implementation][runner].

The pinned upstream comparison showed no relevant newer session route/service change closing those gaps. This supports a conclusion only for the inspected release and dev snapshot; it is not an upstream roadmap commitment. Choosing the newer API remains sensible, but it means either waiting for/contributing missing capabilities or explicitly accepting a reduced contract. Mixing legacy sessions into the new design should be a separate decision, not an invisible workaround. [Pinned upstream comparison][dev-compare].

### The additional material risk: served execution and tools

The server explicitly supplies `SessionExecutionLocal.node` for `SessionV2`, and that execution service calls `SessionRunner.Service`. This is a real durable-session execution design, not a legacy bridge or deliberate no-op. The runner resolves a model, advertises materialized tools to the provider, settles local calls, records results, and continues. [Serve route composition][serve-source], [local execution coordinator][execution-source], [runner][runner].

However, the retained route layer graph does not list `SessionRunner.node`; the local execution node's dependencies include session storage and location mapping, while its drain resolves the runner lazily. The source audit therefore identifies a plausible missing-service failure on the first ordinary wake. `resume:false` deliberately bypasses that path, so successful startup/admission cannot settle the question. This is a **source-supported wiring concern**, not an observed model failure. [Serve wiring audit][serve-wiring].

Similarly, the newer built-in catalog declares `apply_patch`, `bash`, `edit`, `glob`, `grep`, `question`, `read`, `skill`, `todowrite`, `webfetch`, `websearch`, and `write`, but the inspected served layer includes `ToolRegistry.node` without the built-in registration node. The checked MCP service lists tools separately without an established bridge into this v2 registry; the checked plugin host also does not expose the needed tool registration. Local tool execution exists, while served built-in/MCP/plugin coverage is incomplete or unproven. A coding harness needs more than a successful text-only inference request. [Tool coverage audit and retained sources][tools-audit].

Other declared operations also illustrate why spec presence is insufficient: released `shell`, `skill`, `compact`, and `wait` methods explicitly return `OperationUnavailableError`. Model and agent switching have implemented event publication. Do not infer every listed operation works from successful client generation. [Released session implementation][session-core].

## 2. The OpenAPI specification is available and includes both generations

The CLI command works: `opencode generate` emits the combined specification, including the newer 51-path surface. A running server also exposes `GET /doc`. Both use the same API exporter. The CLI adds JavaScript `x-codeSamples` and formatting; after removing those samples, the saved CLI and server documents are structurally identical. The server document is 478,968 bytes with SHA-256 `46db986090aae41846cd6dbe16225a1d883f0bbcb4c48814008d3f6ce140aa5c`. [Raw CLI input][full-spec], [raw server document][server-doc], [comparison and source][spec-provenance].

A third source is pinned [`packages/sdk/openapi.json`](https://github.com/anomalyco/opencode/blob/014614d35b397775e5d397a490fc72368c894ec2/packages/sdk/openapi.json); `packages/docs/openapi.json` is a symlink to it. The saved comparison found byte-identical SDK specifications at 1.18.27, 1.18.31, and the inspected dev commit. That is a useful provenance check, not a promise of future stability. Generate from the actual supported binary when refreshing the adapter, retain the original, and compare against the pinned source. [Provenance comparison][spec-provenance].

### Subsetting is worthwhile, but has three distinct meanings

1. **Path-only subset:** keep 51 `/api/` paths and all 472 schemas. This is the shared input used for fair raw generator comparisons. It removes legacy operations but not legacy model baggage.
2. **Reference-closed subset:** keep those paths and the 243 schemas retained by the tested closure traversal. This is the practical operation/model input, with zero unresolved local schema references in the saved audit.
3. **Components-only model input:** retain those 243 schemas, remove every path, and set `skip-prune:true`. This is what made stable oapi model generation compile. It intentionally omits schemas defined inline inside operations, so it cannot itself define the whole SDK contract.

The independent libopenapi traversal counted 242 rather than 243 reachable components; `SessionActive` is the extra schema in the tested 243-schema document. This discrepancy was not resolved by assuming either count proves a uniquely minimal closure. Use **243 as the measured input to the successful experiment**. Neither closure count is a measured custom-generator code size. [Closure manifest][closure-manifest], [runtime review][runtime-review], [components-only transform][components-transform].

Subsetting does not remove all complexity. The saved 243-schema audit counts 61 `anyOf` occurrences and one 28-branch `oneOf` for `SessionDurableEvent`; it has no explicit `discriminator` or `const` keys. Literal tags appear as singleton enums on properties such as `type`, `name`, or `_tag`. The audit found no type arrays or `nullable:true`; the exporter strips optional null arms. These are syntactic counts, not a claim that every union has one uniform discriminator strategy. Shared message/event models can remain reachable even when their names resemble older API types. [Schema audit][polymorphism].

### What the spec does not describe adequately

| Concern | Spec representation | Runtime/integration consequence |
|---|---|---|
| SSE | Session stream models a string `data` field using `contentSchema` for durable event JSON; global stream advertises `V2Event` | Both are `text/event-stream` at runtime. Need SSE framing plus a separate JSON decode; the two schema representations are not identical. |
| Cursor | Session endpoint has `after` query parameter | Sequence is `durable.seq` inside JSON, not an SSE `id:` line. Generic `Last-Event-ID` handling does not establish OpenCode replay. |
| Basic authentication | Security schemes removed; newer operations declare empty security | Inject auth manually despite apparently unsecured generated methods. |
| PTY connection | Ordinary GET/JSON shape | Actual endpoint upgrades to WebSocket and uses a connect ticket; exclude from harness SDK scope unless needed. |
| Implemented capability | Operation appears with request/response/error schemas | `wait` still returns 503; generation cannot establish execution readiness. |
| Turn boundaries | Durable steps and message/event schemas | No demonstrated universal terminal-turn event or working wait call; client must correlate settlement. |

All 188 operations have operation IDs, so names are available for custom method generation. Errors include typed 400/401/404/409/503 variants, but generated error wrappers themselves triggered a stable oapi collision. The complete spec has 185 declared 400 responses, 67 404s, 58 401s, five 409s and five 503s; these counts describe declarations, not tested runtime coverage. [Fidelity census][fidelity], [runtime source][runtime-source], [wire transcript][wire-probe].

## 3. Generator comparison: actual output, not advertised compatibility

Grades below are specific to this OpenCode input: **A** means demonstrated fit for the stated scope; **B** means usable with explicit residual work; **D** means partial output that misses required coverage; **F** means blocked generation/compilation. An A for components does not mean A for a complete SDK. Configurability is described separately because extensive options do not compensate for missing event types.

Sizes count generated, non-test Go code only: number of Go files, raw newline count, and bytes. They exclude docs, test files, specification copies, logs, binaries, module files, and dependencies. Rows have different explicit scopes and should not be read as equivalent functionality.

| Generator / tested scope | Generation | Compile | Generated Go files / lines / bytes | Union result | Grade |
|---|---|---|---:|---|---|
| **oapi-codegen 2.8.0, 243 components / zero paths / one name annotation** | Pass | Pass | **1 / 14,448 / 477,364** | Newer event raw roundtrips and two typed helper conversions pass; permissive, no automatic tag dispatch | **A models; B hybrid candidate** |
| oapi-codegen 2.8.0, 51 paths / 243 schemas / one name annotation, models+client | Pass | **Fail** | 1 / 29,868 / 1,006,106 | Event types emitted; package prevents runtime probe | F SDK |
| oapi-codegen 2.8.0, same 51-path input, models only | Pass | **Fail** | 1 / 16,921 / 569,424 | Still emits colliding operation response wrappers | F exact models scope |
| oapi-codegen 2.8.0, raw full / path-only newer input | **Fail** | Not reached | No valid models+client artifact | Duplicate type names block generation | F raw SDK |
| **oapi-codegen-exp 0.1.0, same 243 components / zero paths / annotation** | Pass | Pass | **1 / 63,390 / 1,893,908** | Same newer raw roundtrip and typed-helper checks pass; permissive unions | **B models; larger alternative** |
| oapi-codegen-exp 0.1.0, raw full / newer input | **Fail** | Not reached | No file emitted | Invalid Go during formatting | F raw SDK |
| ogen 1.24.0, raw full / newer input | **Fail** | Not reached | No code | Numeric `exclusiveMinimum` parser blocker | F raw SDK |
| ogen 1.24.0, transformed newer input, client only | Pass with omissions | Pass | 11 / 20,904 / 503,570 | 35/58 operations omitted, including both SSE endpoints; key unions absent | D partial SDK |
| ogen 1.24.0, transformed 243-schema closure, models only | Pass with omissions | Pass | 7 / 14,669 / 342,348 | Only 23/243 component names emitted; event unions absent | D models |
| OpenAPI Generator 7.25.0 Go target, raw newer path filter, validation skipped | Pass | **Fail** | 883 / 227,040 / 6,044,786 | Broken enum/helper output; no compiling real-event probe | F SDK |
| OpenAPI Generator 7.25.0 Go target, full input, validation skipped | Pass | **Fail** | 930 / 245,763 / 6,588,754 | Same class of failures | F SDK |
| Contiamo 0.19.0, raw full / newer input | **Fail** | Not reached | No code | Numeric `exclusiveMinimum` parser blocker | F direct candidate |

Sources: [oapi results][oapi-result], [ogen results][ogen-result], [ogen models-only][ogen-models], [alternative trials][alternatives-result], and authoritative corrected [size audit][metrics]. Stable oapi “client-only without models” comparison files also generated but failed on undefined parameter/body types; they are not a successful fallback.

### Stable oapi-codegen: the best tested reusable model layer

The full raw input fails with duplicate `EventTuiCommandExecute`; the newer raw path subset fails with duplicate `SessionStatus`. Pruning to 243 schemas leaves the latter collision. One annotation, `x-go-name: SessionStatusEvent` on component `session.status`, removes it without deleting schema constraints or union arms. The 51-path model+client output then fails compilation because repeated error alternatives produce duplicate `AsInvalidRequestError` methods on response-wrapper types. Disabling client generation does not avoid those wrappers. **Removing paths does.** [Generation and compile evidence][oapi-result], [failed wrapper compile log][oapi-compile], [model config][models-config].

The successful output declares `V2Event` and `SessionDurableEvent` as structs holding `json.RawMessage`, with `AsVariant`, `FromVariant`, and `MergeVariant` helpers. Raw `UnmarshalJSON` saves the JSON; `AsSessionNextTextDelta`, for example, simply runs `json.Unmarshal` into that struct. It does not check that the event's `type` matches the chosen helper, enforce required fields, or select a variant automatically. [Generated model code][models-go].

The real newer tests prove semantic JSON preservation for known, unknown, and missing `type` values and successful typed helper conversion for `SessionNextTextDelta` and `SessionNextTextStarted`. Their deliberately small payloads also illustrate permissiveness: accepting them is not full schema-validity proof. An adapter can make an explicit switch on the tag and use the appropriate helper while retaining raw JSON for unknown future variants. That is a reasonable small boundary, not a reason to regenerate every struct. [Actual tests][model-tests].

Nested/optional/nullable correctness should not be overstated. The earlier legacy event harness exercised additional nested shapes, but that is not a comprehensive newer component suite. The final newer test does not cover every union arm, nested model, nullable field, or unknown-field preservation after conversion into a typed struct. Raw union roundtrip preservation is established; typed conversion can discard fields ordinary Go structs do not declare. The exact spec's optional-null normalization also limits what any generator can infer. These are focused future model-contract checks, not evidence that the successful package is unusable.

Configuration supports separate models/client/server generation, pruning controls, naming/type extensions, configuration files, and templates/overlays. The successful approach needs only a bounded closure transformation, zero-path model input, and one naming annotation. Its module uses `github.com/oapi-codegen/runtime v1.2.0`, with indirect JSON-merge and UUID dependencies; those dependency sources are excluded from the 477 KB count. [Pinned upstream project](https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0), [config][models-config], [trial module][models-module].

### Experimental oapi-codegen: successful models, substantially larger output

A final fair-scope trial ran experimental `v0.1.0` with no client/server flags against the **same 243-schema, zero-path input and naming annotation**. It generated one file, compiled, and passed the same newer known/unknown/missing-tag roundtrips and typed helper checks. It is therefore a viable model-layer alternative despite its failed full-SDK trials. [Final experiment][oapi-result], [experimental tests][exp-tests].

Its 1,893,908 bytes and 63,390 lines are approximately four times the stable output's bytes and 4.4 times its lines. The extra output includes additional-property, form, and default-handling machinery. Both preserve raw permissive unions in this scope; the tests do not establish a compensating advantage for Gimble. Prefer stable v2.8.0 unless a specific required field behavior later justifies the larger experimental output. This corrects any inference that the experimental generator's full-client failure also means its reusable structs fail.

### Ogen: smaller compiling output achieved by losing required coverage

Ogen first rejects valid OpenAPI 3.1 numeric `exclusiveMinimum:0` because the parser expects a boolean. The bounded transformed copy removes that keyword; doing so **loses a validation constraint**. Generation then requires ignoring unsupported `complex anyOf`, `sum types with same names`, and discriminator inference. Exit zero consequently describes a deliberately partial result. [Ogen trial][ogen-result].

Disabling server generation yields a compiling 11-file client, but only 23 of the 58 newer operations survive. The two SSE operations and key event/message unions are absent. Models-only does not fix this: it emits 23 of 243 component schema names and drops `V2Event`, `SessionDurableEvent`, `Part`, `Message`, and other important unions. Its lower byte count is not evidence of a more efficient complete representation. [Models audit][ogen-models], [independent review][evidence-review].

Ogen offers useful feature switches, custom HTTP transport, context-aware calls, and upstream SSE support. Those features were not available for the skipped OpenCode operations. In particular, the research does **not** conclude that ogen cannot generate SSE in general; it concludes that this spec/configuration did not produce OpenCode's required streams. Recovering it would require further schema surgery and fresh semantic proof. The successful stable oapi component result makes that a lower-priority path. [Ogen configuration documentation](https://ogen.dev/docs/config/), [v1.24.0 source](https://github.com/ogen-go/ogen/tree/v1.24.0).

### OpenAPI Generator and other candidates

OpenAPI Generator is Java-written and emits Go; oapi-codegen, ogen, and Contiamo are Go-written. Its Go target initially rejects repeated top-level tags. `--skip-validate-spec` allows generation, but both full and newer outputs fail compilation on duplicate enum constants (`ALLOW`, `DENY`, `ASK`) and missing `NullableAnyOf`/`AnyOf` helpers. Its generated event method reads the entire response body before returning, so it is not an incremental SSE client even if compilation is repaired. [Alternative experiment][alternatives-result].

It provides rich template customization, model/API/support-file selection, naming/type mappings, and generator options. Selecting one API file works, but selecting a union model alone omits its dependencies and fails compilation. The newer output additionally contains 14 test files (25,801 bytes) and 880 documentation files (3,177,113 bytes), excluded from the 6,044,786-byte code count. Customizing a large broken client is not more attractive than keeping the successful stable oapi models. [Go generator options](https://openapi-generator.tech/docs/generators/go/), [customization](https://openapi-generator.tech/docs/customization/), [metric audit][metrics].

Contiamo's Go-native generator was actually tried and failed on numeric `exclusiveMinimum`; its observed `v0.19.0` dates to January 2023. Go-swagger `v0.36.6` was surveyed but not tried on the document because its primary model is Swagger/OpenAPI 2.0, not a direct OpenAPI 3.1 fit. Speakeasy, Stainless, and Fern were not tested; no account/signup or cloud specification upload was performed, and no success grade is assigned. They are alternatives to investigate only if a service-based SDK workflow becomes desirable. [Contiamo](https://github.com/contiamo/openapi-generator-go), [go-swagger](https://github.com/go-swagger/go-swagger), [survey evidence][alternatives-result].

### Libopenapi fits the remaining job without replacing the structs

Libopenapi `v0.38.7` parsed and built v3 models from both exact inputs, reporting 162/51 paths and 472 schemas. It is a parser/model/indexing toolkit, not an SDK generator. Its useful role here would be reading operation IDs, request/response schemas, references, and singleton tag enums to drive a small custom layer. The research's custom union demonstration is bounded proof of the technique, not a generated complete SDK. [Library](https://github.com/pb33f/libopenapi), [parser trial][alternatives-result], [libopenapi dossier][libopenapi-index].

The user's former proprietary implementation is unavailable and was not sought or reused. The demonstrated stable oapi result supports the same broad split: **reuse component structs and union holders; custom-generate only the operations/inline types or tag dispatch that are actually needed.** For a small harness route set, writing the initial HTTP methods directly may be simpler still. The choice between a few handwritten methods and a narrow method generator should follow the final required surface. There is no measured evidence here for the cost or final size of a new custom generator, and no custom SDK generator was implemented.

## 4. The small client boundary still needs deliberate behavior

**HTTP methods and inline schemas.** The successful components package omits operation-local parameters, request bodies, and response envelopes. Those must be modeled directly or lifted into named components before a second generation stage. The latter has not been tried and could reintroduce the same response-wrapper collision. The safe recommendation is to keep the proven model layer intact and handle the small required method surface explicitly. All three major generator families offer context/transport hooks, but no tested complete output eliminates this work. [Oapi client inspection][oapi-result], [components transform][components-transform].

**SSE.** The observed frames use `data: <JSON>` followed by a blank line; comment heartbeats appear on the global stream. The JSON event `id` is not an SSE `id:` line. Parse standard framing and comments, decode the event separately, and demultiplex the shared global feed by native session ID. The global handler has a 256-item bounded subscriber buffer and no cursor. Session streams use `after=<durable.seq>`; recovery restores durable facts rather than token-by-token deltas. Backoff and deduplication are client strategies, not server guarantees. [Wire probe][wire-probe], [event handler][event-handler], [session handlers][session-handler].

**Live versus durable coverage.** Live `/api/event` is needed for text/reasoning/tool-input deltas and permission/question control events. Session durable replay carries admission, promotion, settled text/tool records, step usage, and other stored state. Use both if full live projection is required. Deduplicate records visible through both paths by stable native event identity or session sequence. Do not claim a cursor makes every event recoverable. The first report's event compendium remains useful, but the newer raw envelope is `{id,type,data,metadata?,durable?,location?}` rather than the legacy `{id,type,properties}` shape. [Newer event schemas][event-schema], [initial compendium](harness-adapter.md).

**Turn output and usage.** `step.ended` exposes input/output/reasoning/cache usage and cost, but a step can be followed by tool continuation or steering. A terminal finish reason alone does not establish `RunTurn` completion. With `wait` unavailable, settlement must combine the owned prompt/assistant progression, durable history, outstanding tool/steer state, and active execution observation; it needs a real execution probe. Count unique step usage within that turn, not cumulative session totals or both live and replay copies. Preserve allowed Gimble `NativeRef` keys; store extra native metadata in `Data`/`Metadata`, not invented native-reference fields. [Runner][runner], [event schemas][event-schema], [Gimble contract][gimble-contract].

**Cancellation.** HTTP context cancellation stops the client request/stream, while server execution is detached. Gimble cancellation therefore requires a bounded explicit `/interrupt` call and cleanup in addition to closing the stream. Idle interruption succeeded; active provider and child-tool cancellation did not run. Avoid treating a 204 idle result as proof that an active tool process is stopped. [Execution coordinator][execution-source], [runtime probe][runtime-probe].

**Authentication and lifecycle.** `OPENCODE_SERVER_PASSWORD` enables Basic auth; username defaults to `opencode`. Inject the header explicitly because OpenAPI omits the security scheme. (Server middleware also accepts an undocumented `?auth_token=` query parameter, though the `Authorization` header is strongly preferred to avoid credential leakage in logs). Keep ordinary request deadlines separate from a long-lived SSE total timeout. Manage a foreground loopback server as an owned process, poll `/api/health`, and use the binary's `--version` or observed `/global/health` for version information. There is no `/api/version` or `/api/instance/dispose`; shut down an owned process with signals. An attached server is a different ownership case and should not be terminated by closing one session. [Auth source][auth-source], [runtime review][runtime-review], [no-model lifecycle][runtime-probe].

## 5. Reproducing the successful model experiment

These commands use the saved input and pinned local trial tool. They reproduce **components only**, not a complete SDK. Run from the Gimble worktree; the cache contains the exact scripts/configs and isolated model module. The named output file is experimental cache. [Trial instructions][oapi-result].

```sh
cd /Users/tyler/.codex/worktrees/c671/gimble

# The original collection command (already completed successfully):
opencode generate > .gimble/research/opencode-next/input/opencode-generate.json

# The path-only shared subset retains all component schemas:
jq '.paths |= with_entries(select(.key | startswith("/api/")))' \
  .gimble/research/opencode-next/input/opencode-generate.json \
  > .gimble/research/opencode-next/input/opencode-new-api.json

# Build the tested component closure, then add the sole naming annotation.
ruby .gimble/research/opencode-next/trials/oapi/make-api-harness.rb \
  .gimble/research/opencode-next/input/opencode-generate.json \
  .gimble/research/opencode-next/trials/oapi/api-closure.json
jq '.components.schemas["session.status"]["x-go-name"] = "SessionStatusEvent"' \
  .gimble/research/opencode-next/trials/oapi/api-closure.json \
  > .gimble/research/opencode-next/trials/oapi/api-closure-overlay.json
ruby .gimble/research/opencode-next/trials/oapi/make-components-only.rb \
  .gimble/research/opencode-next/trials/oapi/api-closure-overlay.json \
  .gimble/research/opencode-next/trials/oapi/components-only.json

.gimble/research/opencode-next/trials/oapi/bin/stable/oapi-codegen \
  -config .gimble/research/opencode-next/trials/oapi/stable-components-models.yaml \
  -o .gimble/research/opencode-next/trials/oapi/stable-components-models/models.gen.go \
  .gimble/research/opencode-next/trials/oapi/components-only.json

cd .gimble/research/opencode-next/trials/oapi/stable-components-models
go test -count=1 ./...
```

To install that exact generator rather than use the retained binary:

```sh
GOBIN=/Users/tyler/.codex/worktrees/c671/gimble/.gimble/research/opencode-next/trials/oapi/bin/stable \
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
```

The model config is deliberately small:

```yaml
package: opencode
generate:
  models: true
output-options:
  skip-prune: true
```

For the competing trials, complete versioned commands, transforms, stderr, and module/test results are retained in [ogen RESULT][ogen-result], [ogen models-only][ogen-models], [oapi RESULT][oapi-result], and [OpenAPI Generator/alternatives RESULT][alternatives-result]. The OpenAPI Generator JAR is independently pinned by SHA-256 `41ce4f6b07f196676439d710759fa1ced7a08066d06ff1bf314681470289efae`. Do not compare a transformed ogen run with a raw oapi failure without carrying the transformation/coverage qualifications forward.

## 6. What to decide next

The next useful experiment is a **small newer-API execution investigation**, delegated to a cheap-model agent, not a full adapter implementation. It should establish whether the installed served runner can execute a turn and a real coding tool, and inspect the missing-service concern if it cannot. A source-only registry declaration or an admission response is not sufficient. Only after normal execution works is it useful to demonstrate same-active-turn steering, terminal output/usage, active cancellation, and reconnection through a short real turn.

In parallel with that bounded investigation, the architectural decision can already be made: target `/api`, keep stable oapi-codegen's reusable component models, and reserve custom work for the method/inline-schema/SSE boundary. **Do not spend another long round forcing ogen or a monolithic OpenAPI Generator client to work before testing the upstream capability gate.** Neither alternative currently offers a demonstrated advantage for the needed event unions.

Native fork and schema-constrained output still need explicit resolution with upstream or a deliberate change in supported Gimble behavior. The newer API is the preferred foundation because it has the desired durable input/control design, but preference is not proof of complete contract coverage. Until those gaps and served execution are resolved, the warranted deliverable is a viable client-generation approach plus a clear API blocker list, not a production-ready harness adapter.

Finally, pin the supported OpenCode binary/spec and generator configuration together. On an upgrade, inspect path/operation/schema changes, regenerate the component package, compile, and exercise the few event/inline-schema contracts actually consumed. An unchanged published spec can coexist with runtime wiring changes; API comparison and a tiny live turn answer different questions. No OpenCode model inference was performed for this report, so the cheap live execution check remains the decisive unperformed proof.

[input-readme]: ../../../.gimble/research/opencode-next/input/README.md
[full-spec]: ../../../.gimble/research/opencode-next/input/opencode-generate.json
[new-spec]: ../../../.gimble/research/opencode-next/input/opencode-new-api.json
[server-doc]: ../../../.gimble/research/opencode-next/topic-003/sources/server-doc-1.18.31.json
[spec-provenance]: ../../../.gimble/research/opencode-next/topic-003/sources/spec-diff-and-provenance.json
[fidelity]: ../../../.gimble/research/opencode-next/topic-003/sources/spec-fidelity-analysis.json
[runtime-source]: ../../../.gimble/research/opencode-next/topic-003/sources/runtime-source-excerpts.ts
[gimble-contract]: ../../../.gimble/research/opencode-next/input/gimble-api.txt
[session-core]: ../../../.gimble/research/opencode-next/topic-002/sources/opencode-v1.18.31-session-interface.ts
[session-handler]: ../../../.gimble/research/opencode-next/topic-001/sources/opencode-v1.18.31-session-v2-handlers.ts
[prompt-schema]: ../../../.gimble/research/opencode-next/topic-002/sources/opencode-v1.18.31-prompt-input-schema.ts
[input-source]: ../../../.gimble/research/opencode-next/topic-001/sources/opencode-v1.18.31-session-input.ts
[runner]: ../../../.gimble/research/opencode-next/topic-001/sources/opencode-v1.18.31-session-runner-llm.ts
[event-schema]: ../../../.gimble/research/opencode-next/topic-009/sources/schema-session-event.ts
[event-handler]: ../../../.gimble/research/opencode-next/topic-009/sources/server-event-handler.ts
[fork-comparison]: ../../../.gimble/research/opencode-next/topic-002/sources/opencode-generate-fork-and-schema-comparison.json
[dev-compare]: ../../../.gimble/research/opencode-next/topic-002/sources/upstream-dev-session-compare.json
[serve-wiring]: ../../../.gimble/research/opencode-next/trials/ogen/SERVE-WIRING.md
[serve-source]: ../../../.gimble/research/opencode-next/trials/ogen/packages_opencode_src_server_routes_instance_httpapi_server.ts
[execution-source]: ../../../.gimble/research/opencode-next/trials/ogen/packages_core_src_session_execution_local.ts
[tools-audit]: ../../../.gimble/research/opencode-next/trials/ogen/TOOL-COVERAGE.md
[runtime-probe]: ../../../.gimble/research/opencode-next/topic-010/sources/no-model-lifecycle-probe.txt
[wire-probe]: ../../../.gimble/research/opencode-next/topic-009/sources/sse-wire-and-turn-probe.txt
[auth-source]: ../../../.gimble/research/opencode-next/topic-010/sources/server-auth-middleware.ts
[runtime-review]: ../../../.gimble/research/opencode-next/trials/ogen/RUNTIME-REVIEW.md
[evidence-review]: ../../../.gimble/research/opencode-next/trials/alternatives/EVIDENCE-REVIEW.md
[metrics]: ../../../.gimble/research/opencode-next/trials/alternatives/QA-NOTES.md
[oapi-result]: ../../../.gimble/research/opencode-next/trials/oapi/RESULT.md
[oapi-compile]: ../../../.gimble/research/opencode-next/trials/oapi/stable-api-overlay/compile.log
[ogen-result]: ../../../.gimble/research/opencode-next/trials/ogen/RESULT.md
[ogen-models]: ../../../.gimble/research/opencode-next/trials/ogen/TYPES-ONLY.md
[alternatives-result]: ../../../.gimble/research/opencode-next/trials/alternatives/RESULT.md
[models-go]: ../../../.gimble/research/opencode-next/trials/oapi/stable-components-models/models.gen.go
[model-tests]: ../../../.gimble/research/opencode-next/trials/oapi/stable-components-models/models_test.go
[models-config]: ../../../.gimble/research/opencode-next/trials/oapi/stable-components-models.yaml
[models-module]: ../../../.gimble/research/opencode-next/trials/oapi/stable-components-models/go.mod
[components-transform]: ../../../.gimble/research/opencode-next/trials/oapi/make-components-only.rb
[closure-manifest]: ../../../.gimble/research/opencode-next/topic-004/sources/closure-manifest.json
[polymorphism]: ../../../.gimble/research/opencode-next/topic-004/clips/polymorphic-constructs-analysis.md
[libopenapi-index]: ../../../.gimble/research/opencode-next/topic-007/INDEX.md

[exp-tests]: ../../../.gimble/research/opencode-next/trials/oapi/exp-components-models/models_test.go
