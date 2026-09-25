# Structure recovery (`autoformat`)

## Purpose

This cookbook recovers Markdown structure from stripped, hard-wrapped plain text without asking a generator to rewrite the document. Jev answers narrow boundary and block-type questions; deterministic code preserves source characters, groups blocks, and renders markup. This is a strong pattern for supervision systems that need semantic judgment without letting the judge mutate the underlying transcript.

## Key concepts and evidence

- The pipeline deliberately separates semantic judgment from mutation: Jev decides whether a line continues a sentence and what type a block is, while code performs merging and rendering, preserving every input character. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Two sequential requests cover the whole document: a `Noul` per eligible adjacent line pair, then a `Choice` per merged block plus companion questions for heading level, sequence order, and callout type. Blank lines and explicit markers remain deterministic evidence. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Stable short IDs (`L014`, `B003`) are embedded in state so questions can point to exact units. This is directly reusable for run events, messages, tool calls, and supervision observations. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- The stitch question is intentionally factual and local: “does this line pick up mid-sentence?” Blank-line-separated pairs are not asked at all. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Routing combines Jev output with deterministic punctuation: merge at `>=0.2` after dangling text and `>=0.5` after terminal punctuation. The direct signal and semantic signal are complementary, not competitors. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Companion questions are asked eagerly in the same second request and read only when the block type makes them relevant. The stated tradeoff is that state dominates token volume, while a third round trip adds latency. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Rendering is fully deterministic: consecutive list items become ordered only when their mean step probability is at least `0.5`; code, heading, quote, and callout marks are fixed mappings. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)

## Measured examples

- On 28 nonblank memo lines, the first request asked 16 pair questions in `0.32s`; deterministic merging produced 17 blocks and healed 11 line breaks. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md) [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- The second request asked 62 questions about 17 blocks in `0.51s`. It correctly separated three unordered team items (step probabilities `0.12-0.16`) from three ordered migration steps (`0.86-0.90`) and identified the warning callout at confidence `0.65`. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- The cost appendix reports 10,211 tokens and `0.8s`. Its generated calculation prints `$0.0003`, but surrounding prose says `$0.0015`; do not quote a cost without resolving this internal inconsistency. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Broad wording caused a concrete failure: “same paragraph” gave unmarked list transitions probabilities `0.77-0.91`, collapsed lists, and produced 12 blocks instead of 17. The narrow “mid-sentence” wording kept the same transitions at `0.05-0.22`. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- The lowest-confidence block was a genuinely multi-role list introduction: confidence `0.43`, with probabilities paragraph `0.53`, list item `0.24`, callout `0.19`. The cookbook suggests review below type confidence `0.55`. [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)

## Citation bookmarks

- Architecture and non-rewriting contract: [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Stitch question and thresholds: [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Block taxonomy and companion questions: [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Deterministic renderer: [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)
- Wording ablation: [autoformat](https://docs.typesafe.ai/cookbooks/autoformat.md)

## Themes for continuous supervision

- **Annotate, do not rewrite:** ask Jev whether a run fragment exhibits a coaching-relevant property; preserve the original event stream and let Gimbal decide what to show or do.
- **Stable object IDs:** label messages, tool calls, stalls, review findings, and plan revisions before asking batched questions.
- **One state, many questions:** carry likely companion diagnostics in the same request, but ignore irrelevant answers downstream.
- **Hybrid evidence:** deterministic facts such as exit status, elapsed time, file paths, and explicit error markers should bypass Jev; Jev should answer only semantic questions code cannot derive.

## Gotchas and failure modes

- Vague category wording measures topic continuity instead of the operational fact. The “same paragraph” ablation is a direct warning against broad prompts such as “does this agent need help?”
- Thresholds are corpus-specific. The `0.2/0.5/0.55` values are demonstrated on one memo, not calibrated defaults.
- A winning label can still be weak (`0.43` type confidence). Always retain the full distribution for review and tuning.
- Asking all companions cheaply is useful only when the state dominates request cost; validate that assumption for long Gimbal traces and the deployed API.
- Published dollar cost is internally inconsistent (`$0.0003` computed versus `$0.0015` prose).

## Task recipes

1. **Segment an agent trace without rewriting it:** assign stable IDs to events; use narrow Nouls for “continues the same attempt?” and Choices for event role; use explicit timestamps, process exits, and blank/phase boundaries in code; assemble spans deterministically.
2. **Detect coaching moments:** classify each span as progress, repetition, uncertainty, blocked dependency, or verification; ask companion Nouls such as “is user input required?” and “is there new evidence?” in the same request; route low-confidence or conflicting spans to an agent/human reviewer.
3. **Tune wording before thresholds:** replay a labeled corpus with paired formulations of each question, inspect distribution separation, then select thresholds in code. Do not compensate for an ambiguous question by endlessly tuning a cutoff.
