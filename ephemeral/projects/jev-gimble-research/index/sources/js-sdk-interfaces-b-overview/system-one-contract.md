# `systemOne` request, payload, result, and usage contract

## Purpose

This leaf covers the typed boundary for one Jev evaluation: authored request, wire payload after model resolution, typed answer map, returned model identity, and token usage. It is the core contract for attaching Jev measurements to Gimble run state while preserving enough metadata to reproduce or audit a decision.

## Authored request

- `SystemOneRequest<Q>` packages state and named questions for `systemOne`; `Q` extends `Questions`, enabling answer types to be derived from the supplied question primitives. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:5-19`
- `model` is optional and inherits the client’s `defaultModel` when omitted. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:21-32`
- `questions` is required and documented as nonempty; keys are the names later used to identify answers. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:33-44`
- `state` is required `EntryType`: text, JSON object, array, or `null`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:45-55`
- Additional properties on a request variable are forwarded, including properties whose values are `null`. This is broader than the three declared fields and matters for exact wire-shape auditing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:7-10`

## Resolved wire payload

- `SystemOneRequestPayload` is the body for `POST /v1/systemone` after the model has been resolved. It extends `SystemOneRequest`, but `model` is now required `string`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequestPayload.md:5-27`
- The wire payload carries a nonempty `Questions` map and the same `EntryType` state. Unlike authored `SystemOneRequest<Q>`, the shown payload interface is not generic over the specific question map. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequestPayload.md:29-59`

## Typed result and usage

- `SystemOneResult<Q>` returns answers keyed by question name; each answer type is computed as `ResultFor<Q[K]>`. The prose explicitly promises types inferred from supplied questions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneResult.md:5-25`
- `model` is a readonly string naming the model actually used. Store this, especially when the request used an alias or inherited default. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneResult.md:27-38`
- `usage` is readonly request token usage. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneResult.md:39-49`
- `Usage` contains readonly `input_tokens` and `output_tokens`, both numbers. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/Usage.md:5-31`

## Citation bookmarks

- Authored request and forwarding behavior: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequest.md:5-55`
- Resolved payload: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneRequestPayload.md:5-59`
- Typed answers and model identity: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/SystemOneResult.md:5-38`
- Token fields: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/Usage.md:5-31`

## Themes

- **One state, many typed measurements:** batch related supervision questions while preserving primitive-specific response types.
- **Distinguish requested from actual model:** authored model may be omitted or aliased; result model is the runtime identity available for evidence.
- **Persist durable cost units:** input/output token counts are safer long-lived evidence than derived dollar cost.
- **Wire shape matters:** forwarded extra properties and explicit `null` values can affect cache keys, server validation, or behavior.

## Gotchas and gaps

- “Nonempty questions” is documented in prose, but the shown TypeScript property is only `Q`; compile-time nonemptiness is not evident from this interface.
- Additional properties are forwarded, including nulls. Accidental fields, secrets, UI-only metadata, or cache-buster values may reach the service; construct a narrow request object rather than forwarding broad application state.
- The payload’s required `model` confirms resolution, but its copied property prose still says omitted values inherit `defaultModel`; the type and interface-level description are more precise than that inherited sentence.
- `Usage` contains no cached-token breakdown, latency, request count, dollar cost, retry count, or model pricing/version date.
- `SystemOneResult.model` is just a string; it does not say whether it is immutable version identity or an alias. Preserve request model and result model separately.
- The assigned docs do not specify answer behavior for missing/extra keys, partial failures within a batch, or server-side limits on state/questions.

## Task recipes

1. **Build an auditable call record:** persist the exact authored state/questions, requested model/default snapshot, returned `result.model`, raw typed answers, usage, SDK version, and request ID when available.
2. **Keep state minimal:** pass only the bounded run window and deterministic metadata each question needs; reject unexpected request keys before calling.
3. **Batch coherent questions:** put independent signals about the same trace window in one request; split unrelated states so failure/retry and provenance remain intelligible.
4. **Price after collection:** store input/output tokens and join them to a dated price table later instead of persisting only a derived charge.
