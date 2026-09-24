# Composite scoring

## Purpose

Composite scoring decomposes a fuzzy overall judgment into independent Jev Score questions, normalizes their outputs, and combines them using weights explicitly owned by application code.

## Key concepts

- **Decompose before aggregating.** The pattern evaluates independent dimensions separately and combines them only after the model returns. [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md) [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- **One state can serve several role-specific composites.** The resume example scores Python depth, team leadership, system design, and generalism once, then applies one weight vector for senior IC ranking and another for engineering-manager ranking. [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- **Rubrics are descriptive and ordered.** Each dimension uses concrete levels from absence through increasingly strong evidence, rather than asking the model for an opaque overall number. [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- **Normalization and weights are deterministic code.** The example divides four-level-index scores by four to reach 0–1 and calculates weighted sums in Python. [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- **Inspectability enables policy correction.** Because each subscore and each weight is visible, unexpected rankings can be diagnosed and weights adjusted rather than rewriting an opaque prompt. [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)

## Citation bookmarks

- Pattern premise: [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- Parallel dimensions and role-specific weight diagram: [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- Four complete rubrics: [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- Normalization and two weight vectors: [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)
- Inspectability rationale: [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md)

## Themes

- Model output is evidence by dimension; code defines the objective.
- The same atomic evaluation can support multiple policies without another model call.
- Review and tuning become localized to rubric wording and weight constants.

## Gotchas

- Weighted sums can hide one catastrophic subscore. For supervision safety, pair the composite with hard code gates for dimensions that must never be averaged away.
- Normalization depends on rubric cardinality and score semantics; copying `/4` is only correct for a five-level rubric indexed 0–4.
- The page discusses inspecting scores and tuning weights but does not describe training/evaluation data, confidence treatment for individual scores, or how to avoid post-hoc overfitting.

## Task recipes

- **Build an observable run-health score:** ask separately about progress, repetition, instruction adherence, evidence quality, and risk; keep all dimension values visible; use Go-owned weights only for prioritization. Start at [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md).
- **Support several supervisor policies:** reuse the same atomic scores with one weight vector for “needs coaching soon” and another for “needs immediate human attention.” The two-role analogue is [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md).
- **Debug a bad intervention:** inspect the dimension scores first; if those match labels, tune code weights; if not, revise the affected rubric and reevaluate. The source rationale is [composite scoring](https://docs.typesafe.ai/patterns/composite-scoring.md).

