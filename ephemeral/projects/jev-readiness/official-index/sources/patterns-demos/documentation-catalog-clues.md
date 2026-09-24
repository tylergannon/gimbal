# Documentation catalog clues for agent supervision

## Purpose

This leaf records supervision-relevant destinations advertised by the official `llms.txt` catalog. These are discovery pointers and one-line source summaries, not substitutes for indexing the underlying cookbook or model-jaggedness pages.

## Key concepts

- **The official corpus includes a model-specific jaggedness page.** The catalog explicitly says Jev 1.13 has known imperfect edges expected to change in later versions; any production investigation should read and version-pin that page. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:92-96`
- **Self-consistency cookbooks make uncertainty visible.** The Noul cookbook routes uncertain probabilities to human review without hiding raw values; the Choice cookbook adds an uncertain outcome and compares agreement with automation share. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:97-98`
- **The parallel-questions cookbook reports concrete batching results.** Its catalog summary claims a 13-question regulatory briefing was 12.2× cheaper and 10.0× faster in one call with unchanged answers. This is much stronger evidence than the qualitative latency claim on the fan-out pattern page, but still requires reading the cookbook and reproducing on supervision traces. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:99-99`
- **The catalog includes direct agent-routing precedents.** Function calling maps natural-language requests to ordinary typed functions using closed-set, confidence-aware questions; skill suggestion ranks 182 agent skills, checks whether any skill is needed, then evaluates the top three and may reject all. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:103-104`
- **Several cookbook summaries resemble supervision guardrails.** RAG passage classification screens retrieved evidence before an answering model; citation checking compares a claim against source context and can use confidence for human review; LLM guardrails screen inputs and outputs for hazards and let code pass, review, block, or route. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:106-108`
- **The catalog advertises staged escalation and learning loops.** The SDE cascade uses mini → verify → reasoning stages; autoresearch feature discovery turns proposed questions into numeric features and uses model errors to improve a supervised regressor. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:109-113`
- **Confidence can drive coarser fallback, not only human escalation.** A classification cookbook chooses a detailed industry group when confident and a broader parent division otherwise. `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:114-114`

## Citation bookmarks

- Core concepts, primitives, patterns, demos, and SDK map: `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:5-39`
- Current JS SDK API surface: `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:39-91`
- Model jaggedness destination: `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:92-96`
- Consistency, batching, semantic retrieval, function, and skill routes: `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:97-105`
- RAG screening, citation checks, guardrails, cascades, extraction, hierarchical classification, autoresearch, and confidence fallback: `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:106-114`

## Themes

- Jev’s adjacent ecosystem already targets several components of an automated supervisor: skill selection, guardrails, evidence filtering, citation verification, staged escalation, and error-driven feature discovery.
- The catalog points toward a cheap-first cascade: typed classification → deterministic validation or specialist agent → human/reasoning fallback.
- A Gimble-specific system could learn from labeled intervention errors without handing routing ownership to the model.

## Gotchas

- This source is a catalog. Its one-line cookbook summaries omit assumptions, code, datasets, failure cases, and license/deployment details; the corresponding pages must be read before making capability or performance claims.
- Several catalog descriptions are visibly truncated in the cached `llms.txt` (for example skill suggestion, entity alignment, and guardrails), so do not reconstruct missing clauses.
- No title or summary in this complete documentation catalog advertises vision, images, audio, or multimodal input. That is evidence of a documentation gap, not proof that the API rejects those inputs.
- The catalog includes a Jev 1.13 jaggedness page, but that page was not part of this worker’s assigned corpus and remains an important unresolved source.

## Task recipes

- **Research an agent-supervision proof of concept:** next index the skill-suggestion, LLM-guardrails, citation-check, self-consistency, parallel-questions, autoresearch-feature-discovery, and Jev-1.13-jaggedness pages identified at `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:96-114`.
- **Design a cheap-first supervisor cascade:** use Jev to classify whether an event needs no action, deterministic verification, specialist coaching, or human review; verify evidence/citations before composing coaching; invoke a generative model only on the selected lane. The catalog precedents are `ephemeral/projects/jev-readiness/sources/official-docs/llms.txt:103-109`.
- **Investigate vision honestly:** check the HTTP request schema, model card, and current product/API announcements outside this segment; record a live request if an image-bearing state shape is documented. The assigned catalog has no positive vision pointer.

