# Task-first routes

## Which Go SDK should Gimbal use?

Start with the [seven-client comparison and live SDK smoke result](../sdk-and-supervision.md). TypeSafe does not publish a Go SDK; use this index for the official [HTTP API and model contract](sources/core-foundations/interface-capabilities.md), then inspect the selected Go SDK's source and tests before integration.

## Can Jev supervise or coach Gimbal agents?

Start with [programmatic supervision](sources/core-foundations/programmatic-supervision.md), [decision primitives and confidence](sources/core-foundations/primitives-confidence.md), [tool routing and guardrails](sources/cookbooks-b/tool-routing-and-guardrails.md), and [cascades and bounded extraction](sources/cookbooks-b/cascades-and-bounded-extraction.md). For learned coaching signals, continue to [autoresearch feature discovery](sources/cookbooks-a/autoresearch_feature_discovery.md). For cheap multi-question evaluation, see [skill routing and batching](sources/cookbooks-b/skill-routing-and-batching.md) and [speculative fan-out](sources/patterns-demos/speculative-fan-out.md).

Interpretation boundary: Jev can detect, classify, score, or select among predefined interventions. It cannot generate novel coaching text. Gimbal code or a generative supervisor must own that step.

## Does Jev support vision or other media?

Open [interface capabilities](sources/core-foundations/interface-capabilities.md). The official model and state pages establish text-only input; an OCR, accessibility-tree, captioning, or vision system must first produce text or structured fields.

## How should confidence, abstention, and calibration work?

Use [decision primitives and confidence](sources/core-foundations/primitives-confidence.md), [confidence-gated routing](sources/patterns-demos/confidence-gated-routing.md), [classification using confidence](sources/cookbooks-a/classification_using_confidence.md), and both consistency studies: [Choice](sources/cookbooks-a/consistency_choice_cookbook.md) and [Noul](sources/cookbooks-a/consistency_noul_cookbook.md). For downstream learned features, see [autoresearch](sources/cookbooks-a/autoresearch_feature_discovery.md).

## Can Jev verify evidence, outputs, or tool calls?

Use [citation checking](sources/cookbooks-a/citation_check.md), [tool routing and guardrails](sources/cookbooks-b/tool-routing-and-guardrails.md), [cascades and bounded extraction](sources/cookbooks-b/cascades-and-bounded-extraction.md), and [composite scoring](sources/patterns-demos/composite-scoring.md). Keep deterministic checks such as exit codes, schema validation, file existence, hashes, and arithmetic in Go.

## Can Jev retrieve context, rank evidence, or route skills?

Use [retrieval and hierarchies](sources/cookbooks-b/retrieval-and-hierarchies.md), [classifying RAG passages](sources/cookbooks-a/classifying_rag_passages.md), [skill routing and batching](sources/cookbooks-b/skill-routing-and-batching.md), and [intent routing](sources/patterns-demos/intent-routing.md).

## How do cost, context, rate limits, privacy, and model drift affect use?

Start with [interface capabilities](sources/core-foundations/interface-capabilities.md). For SDK failure behavior, use the [JavaScript route](routes/javascript/README.md) or [Python route](routes/python/README.md). Record requested and returned model IDs; a moving alias can change tuned thresholds without a code change.

## Where are all official pages covered?

- [Foundations and official patterns](routes/foundations/README.md)
- [Cookbooks and measured examples](routes/cookbooks/README.md)
- [JavaScript SDK](routes/javascript/README.md)
- [Python SDK](routes/python/README.md)
