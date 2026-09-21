# Jev core interface, modalities, and operating envelope

## Purpose

This leaf records what Jev 1.13 can accept, return, and sustain operationally. Use it before proposing a Gimble integration, especially when the proposal involves images, long traces, high-frequency supervision, model pinning, privacy, or cost.

## Key concepts

- **Jev is a structured decision model, not a generative model.** It evaluates one `state` against typed questions and returns values and probability distributions intended for direct use by code; it does not return prose, code, or explanations. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction.md:7-11`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/system-one.md:19-29`
- **There is no native vision or other non-text modality in Jev 1.13.** Input is text only: strings or JSON structures containing text. Images, audio, and video must first be converted into text or structured fields by some other system. This means Jev can judge OCR, captions, detected objects, or a vision model's transcript, but it cannot inspect pixels and its probability cannot recover evidence omitted by the upstream conversion. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:11-21`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/state.md:29-33`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/system-one.md:13-17`
- **The HTTP operation is `POST https://api.typesafe.ai/v1/systemone`.** It requires bearer authentication and JSON. The required top-level fields are `state`, `model`, and `questions`; each answer is returned under the caller-chosen question ID. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:11-39`
- **State is shared, questions are isolated.** One request carries a string, object, or array as `state`; all questions see that same state and are evaluated independently. Related evidence should be grouped into one structured object, while irrelevant trace material should be removed before the call. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/state.md:9-29`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/state.md:35-59`
- **Question IDs are application metadata, not model context.** The chosen keys only correlate request and response; the underlying model does not see them. The complete semantic instruction must therefore be in `instructions`, not implied by a key such as `needs_coaching`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:31-37`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/primitives.md:254-277`
- **Instructions can carry structured, code-supplied context.** Objects and arrays can keep a question, rubric, examples, schemas, or comparison records in separately named fields, which the instruction can reference by backticked field name. Structured option and level descriptions can make boundaries contrastive instead of relying on a long prose prompt. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:54-71`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/primitives/advanced.md:240-258`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/primitives/advanced.md:341-355`
- **The response identifies the actual version and reports token usage.** `model` is the version that handled the request, `answers` is keyed like the request, and `usage` includes input and output token counts. This supports logging model-version provenance beside Gimble supervision events. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:180-218`
- **Jev 1.13's advertised context is constrained in two ways.** A request has a 64k-token budget across state and all questions, and a 32k-token budget for state plus the single longest question. The docs also warn that accuracy declines when a large state contains irrelevant material, so fitting in the context window is not evidence that a whole run transcript is a good state. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:11-20`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/model-jaggedness/jev-1.13.md:94-102`
- **Current list price is input-only billing.** The Jev 1.13 table says `$42/Btok` or `$0.042/Mtok`; output tokens are free. Listed limits are 250,000 tokens/second and 1,200 requests/minute, but TypeSafe warns that limits can change without notice and advertises higher custom/enterprise limits. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:9-24`
- **Aliases trade convenience for reproducibility.** `jev-latest` and `jev-preview` currently resolve to `jev-1.13.0`, but aliases move when releases ship. Thresholds tuned against one version should use the pinned version and migrate deliberately; log the response's versioned `model` either way. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:27-40`
- **Errors are conventional and retryable failures are explicit.** `401` means authentication failure, `422` request validation, `429` rate limiting, and `529` overload. Direct integrations should use exponential backoff for `429`/`529`; the client SDKs do this by default and can honor `retry-after`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:325-338`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:18-20`
- **Customization is in-request, not account-specific training.** Jev is not fine-tuned or LoRA-adapted with customer data; the same weights serve all accounts. Domain behavior comes from state, instructions, criteria, and code-side composition. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:42-48`
- **English is the strongest supported language.** Other languages including CJK are accepted but are described as less accurate, so multilingual supervision needs its own evaluation and routing thresholds. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:50-52`
- **Published data-handling claims are limited but useful.** TypeSafe states that customer requests and responses are not used for training. The local corpus links a DPA, customer agreement, and privacy policy; zero data retention is an enterprise offering rather than the documented default. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:54-56`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/legal.md:9-17`

## Important citation bookmarks

- Native modality and preprocessing boundary: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:11-21`
- Full top-level request contract: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:19-52`
- Response envelope and usage: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:180-218`
- Structured state and textual modality caveat: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/state.md:13-33`
- Version aliases and threshold pinning: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:27-40`
- Cost, rate, and context table: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:9-24`
- Privacy and ZDR statements: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/legal.md:9-17`

## Themes

- **Code-owned control:** Jev is an inference component in an ordinary program, not an orchestration framework.
- **Textual sensor:** non-text observations require a separate perception step whose omissions become Jev's blind spots.
- **Batch economics:** shared-state questions amortize state-token cost and latency, while the 64k/32k and relevance limits favor small trace windows.
- **Operational reproducibility:** version pinning, response-version logging, retry policy, and per-version threshold calibration belong in the integration contract.
- **Data governance:** “not trained on customer data” and optional enterprise ZDR are different claims; sensitive agent traces still require a retention and access decision.

## Gotchas, version notes, and legal limitations

- `jev-1.13` limitations were last reviewed 2026-09-17, so this leaf should be refreshed when `jev-latest` moves. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/model-jaggedness/jev-1.13.md:5-13`
- The API reference types `instructions` as `string | object | array` and required, while the advanced structure page includes `null` in its `EntryType` table. The same tension appears for some criteria values. For portable integrations, send non-null instructions and reserve `null` only for Choice option descriptions, where the API explicitly documents it. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:54-95`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:114-132`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/primitives/advanced.md:240-249`
- The corpus gives an approximate “about 100 ms” statement and a 150 ms use-case claim, not a latency SLA or percentile distribution. Do not size a supervision control loop from those claims alone. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/how-to-build-with-system-one.md:274-302`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/use-case-map.md:18-20`
- Rate limits are explicitly dynamic. Capacity planning needs a live account check and graceful degradation, not constants copied from this leaf. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:18-25`
- The local legal page summarizes and links policies but does not contain retention periods, subprocessors, residency, security controls, or ZDR terms. Those questions remain external diligence, especially before sending proprietary run transcripts. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/legal.md:9-17`
- The docs do not specify batch-level limits on number of questions, beyond Choice's 255 options and Score's 10 levels; empirically validate large supervision rubrics before depending on them. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:124-132`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:162-164`

## Task recipes

### Evaluate a text-only supervision slice

1. Construct a small object with the current agent goal, latest assistant/tool events, relevant policy, and available actions; exclude older irrelevant transcript material. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/concepts/state.md:21-29` and `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/model-jaggedness/jev-1.13.md:94-102`.
2. Put semantic meaning in each question's instructions, because IDs are invisible to the model. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/primitives.md:254-277`.
3. Pin `jev-1.13.0` during evaluation and log the response model plus token usage. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:27-40` and `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:180-218`.

### Add visual evidence without pretending Jev has vision

1. Run a separate OCR, image-captioning, or vision-model stage.
2. Preserve the extracted text, source image identifier, regions/pages, and upstream confidence in structured state.
3. Ask Jev only semantic questions answerable from that representation; keep a distinct failure path for “upstream perception insufficient.” The hard capability boundary is documented at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:16-21`.

### Estimate a continuous-monitoring budget

1. Measure actual input tokens for representative state windows and question sets from the returned `usage.input_tokens`; do not estimate from characters alone. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/api.md:198-218`
2. Multiply by the documented input price, then model retry and headroom separately. Output is currently free. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:11-20`
3. Recheck live limits before rollout because the docs say they are dynamic. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/models.md:23-25`

## Gaps left by this source segment

- No native vision capability, roadmap date, or documented image-token interface.
- No latency percentiles, uptime SLA, concurrency contract, or exact large-question-count limit.
- No detailed privacy/security terms in the downloaded page itself; linked legal documents require separate indexing.
- No tokenizer specification or live account-specific pricing/rate-limit discovery endpoint is documented here.

