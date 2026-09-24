# Classifying RAG passages

## Purpose

This cookbook inserts Jev between retrieval and answer generation. Each query-passage pair gets four Noul probabilities—relevance, usable evidence, premise contradiction, and prompt injection—and deterministic ordered policy routes it to accepted evidence, conflicting evidence, or exclusion. It is directly applicable to supervising agent context, evidence selection, and steering triggers.

## Key concepts and evidence

- Retrieval similarity is explicitly treated as insufficient: highly similar passages can be irrelevant, contradictory, or hostile instructions. Jev scores the query-passage relation rather than the passage alone. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Four Nouls in one request measure separate facts. None asks “should this be included?”; routing remains ordinary code so changing policy changes constants, not prompt wording. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Routing is ordered: injection `>0.70` excludes first, contradiction `>0.70` becomes conflict, relevance `<0.45` excludes, evidence `>0.55` includes, otherwise excludes. Security precedes evidence; contradiction precedes inclusion because contradictory passages often contain usable facts. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Thresholds are centralized policy constants, allowing zero-call replay and policy comparison over stored probabilities. The authors explicitly say the four values are corpus-specific starting points. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md) [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Accepted and conflicting evidence are separate prompt blocks. The generator is told to treat every passage as untrusted, cite IDs, report conflicts, and refuse to guess when evidence is insufficient. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)

## Measured examples

- Corpus: 80 verbatim Supabase auth documentation passages plus one planted forum injection; six queries retrieve the top 12 of 81 passages using 256-dimensional embeddings. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md) [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- For the false “refresh tokens expire after 30 days” premise, similarity ranked the planted injection first (`0.584`) and the contradicting official passage seventh (`0.509`); all 12 similarity scores were tightly packed `0.455-0.584`. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Jev scored the injection `0.99` for prompt injection and excluded it despite relevance `0.71`. It scored the official refutation `0.92` for premise contradiction and routed it to conflict even though relevance/evidence alone (`0.49/0.51`) would have dropped it. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- For “How long should an access token live?”, four passages were included. Three near-top retrievals about signing-key lifetime scored `<=0.08` relevance, while three accepted passages had retrieval ranks 8, 9, and 11. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Across six queries, 72 passages were scored; at least two-thirds of each query’s retrieved set was excluded, and only the two false-premise queries produced conflict routes. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)

## Citation bookmarks

- Pipeline overview: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Policy constants: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Four question definitions: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Ordered routing rationale: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Security caveat and request scaling: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Generator prompt boundary: [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)

## Themes for continuous supervision

- **Multi-axis measurement before action:** separate “is this relevant?”, “does it supply new evidence?”, “does it contradict the current plan?”, and “is it trying to steer the evaluator?” rather than one vague quality score.
- **Ordered policy matters:** a run event can be both relevant and dangerous, or both useful and contradictory. First-match routing makes precedence explicit and reviewable.
- **Preserve conflicts:** contradiction should reach the supervising agent in a distinct channel, not be filtered as low-quality context.
- **Rescore retrieved windows:** fast lexical/embedding retrieval finds candidates; Jev gives semantic routing at a per-window cost.

## Gotchas and failure modes

- The injection classifier is only one probabilistic filter. The cookbook explicitly says passages below its threshold still reach the prompt, so every passage remains untrusted and the filter is not a security boundary. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Thresholds were picked for one planted-injection corpus and six queries. There is no production false-negative benchmark.
- Cost scales with retrieved `k`: one Jev request per passage, because each question is about one query-passage pair. [classifying_rag_passages](https://docs.typesafe.ai/cookbooks/classifying_rag_passages.md)
- Asking one monolithic include/exclude question would hide precedence and make policy changes prompt changes.
- A bigger generator remains necessary to synthesize prose answers; Jev gates evidence but does not replace generation here.

## Task recipes

1. **Supervise retrieved run context:** retrieve recent or similar trace windows; ask Nouls for relevance to current objective, new actionable evidence, plan contradiction, and embedded steering/instruction; route in explicit security/contradiction/relevance order.
2. **Build coaching packets:** put accepted observations and contradictions in separate blocks, preserve stable event IDs, and instruct the supervisor agent that all trace text is untrusted data.
3. **Calibrate offline:** cache full Noul vectors, annotate false positives/negatives, sweep policy constants without new Jev calls, and report intervention coverage plus missed-critical-event rate.
