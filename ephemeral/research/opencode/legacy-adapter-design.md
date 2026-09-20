# Legacy OpenCode adapter: verified direction and open choices

September 19, 2026. User direction: build against the legacy API, keep one shared OpenCode server across projects and runs, add `gimble opencode start|stop`, put configurable runtime state under `~/.gimble/` by default, and generate only needed types. Handwrite the small HTTP client and SSE handling. This note records source verification and questions before implementation; no adapter or process-management code has been written.

The three source audits target OpenCode 1.18.31, commit `014614d35b397775e5d397a490fc72368c894ec2`. They reused the exact generated OpenAPI and downloaded relevant pinned primary source. No model inference or server mutation was performed in this phase.

## Shared process and event routing

Use one managed `opencode serve` process for the user across projects. Stock serve supports HTTP TCP, not a Unix socket CLI option. Bind loopback, retain its actual address and process identity in private state, and serialize start/stop across Gimble processes. Start is idempotent. This does not need a generic daemon framework. The same configured state directory must be used by all callers to discover the same server.

Every project request carries the correct absolute directory. `/event` is scoped to directory/workspace. `/global/event` is process-wide and wraps each legacy event in a location envelope. There is no legacy `/session/{sessionID}/event`; the similarly named `/api/session/{sessionID}/event` is the newer durable stream.

One global SSE reader in each Gimble process can route by native session ID to the active turn callback. Independent Gimble processes may each have a subscription to the same OpenCode server; no cross-process event broker is needed. Gimble's existing callback wrapper already records events with the correct run, scope, session and turn. Normalize the selected native events before dispatch and ignore unrelated sessions; do not add another persistence layer.

Closing a session releases its local routing/resources, not the shared process or project instance. OpenCode caches project instances; there is no observed idle eviction timer. Explicit server stop provides memory reclamation. `/instance/dispose` and `/global/dispose` are not substitutes for session cleanup or process stop.

Sources: [server audit](../../../.gimble/research/opencode-legacy-design/server/findings.md), [event audit](../../../.gimble/research/opencode-legacy-design/events/findings.md), [Gimble callback](../../../session.go).

## Completion requires more than idle

There is no legacy `message.completed` event. A `message.updated` snapshot with assistant `time.completed` completes one message/model step. Tool work can cause further steps. Normal completion combines a correlated completed assistant with a later `session.status` idle. Deprecated `session.idle` duplicates the status notification and must not produce a second completion.

Source-derived cases the adapter must distinguish:

- Structured output may be added in a later message update after the completed timestamp. Use the final snapshot, not the first completed snapshot.
- A fatal processor error can publish error and idle before final part/message cleanup; a later idle follows that cleanup.
- Context overflow can publish `session.error` and then compact/retry successfully.
- Missing structured output can appear as `assistant.error` without `session.error`.
- An asynchronous failure before the runner starts can emit `session.error` without any assistant or idle. Some nonfatal diagnostics use the same event channel.

Prefer a short `prompt_async` POST plus the shared stream, with short message/status reads when needed. The 204 response only confirms background dispatch. Track caller message IDs and assistant parent IDs before submitting. Do not equate first error, first idle, or stream EOF with a successful turn.

The async protocol has no universal request-correlated terminal acknowledgment. Source inspection supports the common event-driven paths but does not settle every pre-run/plugin/exception path. A status read alone cannot prove a pre-run creation coroutine ended. Preserve this as a concrete live-proof/design question rather than invent a debounce timeout. The CLI also waits for a synchronous prompt response; copying its idle handler alone omits that signal.

## Small client and generated type surface

Core calls: create session, async prompt, abort, fork, message-history read, global health, and the handwritten global SSE reader. Do not generate every legacy endpoint. Model/provider catalogs are unnecessary when the workflow supplies the model explicitly.

A mechanical reference traversal of the five core nonstream session operations plus eight selected event schemas reaches 60 named component schemas, compared with 472 in the full spec. It avoids the two previously observed component-name collisions. This new subset has not yet been generated or compiled. Use a components-only oapi-codegen input; handwritten request/envelope structs can further exclude unused attachment inputs. Large message/part unions remain useful generated types. Do not generate the entire Event union merely to switch on a handful of tags.

Fork copies conversation data but does not inherit all model/agent/permission settings. Retain adapter settings and send model/variant on prompts; restore explicit permission rules if used. Legacy schema output uses the StructuredOutput tool and assistant `structured` field. Do not assume the exposed retryCount is implemented by the inspected prompt loop.

Source: [minimal surface audit](../../../.gimble/research/opencode-legacy-design/surface/findings.md).

## Steering and decisions still open

Ordinary legacy prompt submission can join an active loop, but a finish race can instead start a new run or leave an input unconsumed. A promising boundary-delivery approach inserts the steer with `noReply:true`, which cannot itself start execution, then checks assistant-parent correlation. If the current run ends without consuming it, delete the unconsumed message before returning a dropped steer. This needs serialized local completion/steering, exclusive ownership of the native session, and live race proof. It adds only short message-create/delete calls. Immediate abort-and-continue inside the same Gimble RunTurn is another option, but interrupts native work.

The user suggested newer API steering for legacy sessions. Two independent source traces found that both APIs share SessionTable, correcting any inference that different list results prove separate session identities. However, newer admission writes SessionInputTable, and newer prompt promotion writes SessionMessageTable. The legacy loop reads MessageTable/PartTable. Newer wake invokes a separate coordinator/runner; the inspected projector does not bridge its prompts into the legacy execution. Therefore newer `delivery:"steer"` is not a supported shortcut for this legacy adapter. An accepted session ID or prompt does not establish active-run interoperability. See the mixed-generation follow-up in both audits.

Questions presented to the user:

1. Automatically start the shared server on first use, or require explicit `gimble opencode start`? Recommendation: automatic start, with explicit start available for prewarming. Cleanup never starts or stops it.
2. Does explicit `gimble opencode stop` interrupt all active work, or refuse while busy? Recommendation for the simple command: stop means stop, and affected runs report interruption.
3. The user proposed newer-API steering instead of selecting boundary delivery or abort/continue. Source inspection rules out treating this as a supported bridge. Recommend proving legacy `noReply:true` boundary delivery before opting for interruption.
4. Would one synchronous prompt-completion request per active turn, alongside the shared SSE stream, be acceptable for a definite result, or must the adapter remain async-only? This is an explicit tradeoff against the user's preference for fewer long-running requests. If async-only is required, resolve the exceptional completion cases before implementation; do not silently weaken completion semantics.

Live validation must establish the selected completion and steering behavior, concurrency across sessions/directories, actual schema output, fork independence, cancellation, and shared process survival after session Close. Tests and probes remain delegated; no proof is claimed from this source audit alone.
