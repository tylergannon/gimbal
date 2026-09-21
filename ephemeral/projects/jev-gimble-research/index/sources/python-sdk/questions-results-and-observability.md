# Python SDK question, result, and observability types

## Purpose

Reference for translating supervision state and decision batteries into Python SDK inputs, consuming primitive-specific answers without conflating their meanings, and retaining model/usage/request metadata for Gimble evaluation and audit trails.

## Key concepts

- `state` is text, a JSON object, or a JSON array; top-level `None` is rejected, while nested values may be `None`. The common aliases distinguish `JSONValue` (recursive scalar/sequence/mapping, allowing nested `None`) from `JSONContent` (top-level string/mapping/sequence). [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:38-48] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/common.md:38-60]
- Questions are keyed by answer name and may freely mix Pydantic `Noul`/`Choice`/`Score` objects with typed/raw dictionaries discriminated by `type`. Those stable names are the join keys between a supervision policy and its returned answers. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:909-987] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:989-1003]
- `Noul` asks yes/no and can optionally describe the true and false outcomes with structured content. Its answer is a single probability: near 1 favors true, near 0 favors false, and near 0.5 is uncertain. It does **not** have a separate confidence field. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:50-58] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:140-150] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:790-872]
- `Choice` selects among named labels. Criteria map each label to a text/object/array description or `None`; `ChoiceAnswer` provides the winning label, confidence, and a probability distribution that sums approximately to 1. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:412-422] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:552-674] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:886-1077]
- `Score` uses a **nonempty ordered sequence**, one description per integer level starting at zero. The returned `score` is a probability-weighted expectation and may be fractional; the response also carries confidence, an integer-keyed legend, and integer-keyed probabilities. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:676-686] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:809-855] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1091-1213]
- `SystemOneResponse` is a frozen, strict Pydantic model that ignores unknown extra fields. It contains returned model name, usage, and a discriminated answer map, plus cached type-filtered views (`nouls`, `choices`, `scores`). [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:42-52] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:273-283] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:393-607]
- `Usage.input_tokens` and `output_tokens` are optional integers: the usage object exists, but either count may be `None` when the API does not report it. Cost accounting must therefore support unknown rather than coercing missing counts to zero. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:609-668] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:682-784]
- Both system and model-list responses expose `request_id` from `x-typesafe-request-id` and the underlying HTTP response. `SystemOneResponse.model` is the model actually used, while model discovery returns a tuple of metadata containing accepted name/alias, description, and `YYYY-MM-DD` release date. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:285-363] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1333-1441] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1507-1638]

## Important citation bookmarks

- Structured input aliases: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/common.md:36-60]
- Primitive object fields: Noul [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:302-410], Choice [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:552-674], Score [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:809-907]
- Answer union discriminator: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1321-1331]
- Response metadata and raw-response access: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:279-363]
- Choice probability semantics: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:969-1077]
- Score output semantics: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1179-1319]

## Themes

- **Named parallel judgments:** one state plus a keyed battery yields independently addressable primitive answers.
- **Probability is not one universal field:** Noul's value is itself the true probability; Choice/Score expose confidence and full distributions.
- **Operational provenance is in the response:** request ID, actual model, usage, and raw HTTP response should travel with supervisory decisions.
- **Strict known data, tolerant future data:** recognized response fields are strictly typed and immutable while unknown extra fields are ignored.

## Gotchas and version limitations

- A `Score` value is an expectation, not necessarily one of the rubric's integer indices. Code that casts it to an enum or integer changes its meaning. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1187-1213]
- Choice probabilities sum only approximately to 1; avoid exact floating-point equality checks. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:1025-1077]
- Question instructions are optional in the Python models, and criteria descriptions may be `None`. Type validity therefore does not imply a sufficiently specified semantic question. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:302-410] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/questions.md:552-674]
- Typed response models are frozen. Build a separate Gimble decision/event object instead of trying to annotate the returned model in place. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/types/responses.md:325-335]
- Extra recognized-response fields are ignored and unknown answer kinds may be omitted from typed views; retaining raw response bytes is the only documented way to preserve the complete forward-version payload. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:325-345]

## Task recipes

### Encode a supervision battery

1. Make `state` a compact JSON object containing the transcript slice, last tool event, declared task state, and any deterministic evidence.
2. Use Noul for binary predicates such as “needs intervention,” Choice for a closed coaching/action taxonomy, and Score for an ordered severity or progress rubric.
3. Give every question a durable name suitable for metrics and specify descriptions for every ambiguous outcome/label/level.
4. Record the whole answer distribution, not only the winner, so thresholds can later be recalibrated.

### Convert a response into an auditable Gimble event

1. Iterate `result.answers` and branch on each answer's `type`; use filtered `nouls`, `choices`, and `scores` only for convenience.
2. Preserve Noul probability or Choice/Score confidence plus distribution without normalizing them into an invented common confidence.
3. Attach `result.model`, nullable token counts, `request_id`, and the configured question-set version.
4. Emit a separate action record stating whether the result caused observe-only logging, coaching, interruption, escalation, or no action.

### Add a typed domain response without losing provenance

1. Subclass `SystemOneResponse` and add fields named after the question keys, typed as `NoulAnswer`, `ChoiceAnswer`, or `ScoreAnswer`.
2. Pass the subclass as `response_model`.
3. Confirm both the named field and base `answers` view resolve to the same object, as in the documented example. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:79-98]
4. Avoid a standalone `BaseModel` unless intentionally giving up the standard response surface or explicitly re-declaring needed metadata.

## Gaps

- No schema version appears in the response, so a stored payload cannot identify its wire-contract version by itself.
- Token usage has no cached/read/write breakdown, monetary cost, latency, or retry-attempt count.
- The SDK types do not provide calibrated policy thresholds or a shared uncertainty abstraction across Noul, Choice, and Score.
- No documented validation connects returned Choice labels or Score legends back to the exact submitted criteria beyond ordinary response validation.
