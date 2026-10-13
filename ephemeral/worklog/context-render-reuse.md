# Context rendering reuse

decision: Optimize Gimbal-owned redundant rendering work, without tokenizer benchmarks, global caches, new public configuration, or reduced context/completeness assertions.
friction: Aggregate rendering searched the full 32 KiB byte range even when all immutable artifact previews were shorter, and each excerpt binary-searched when its largest permitted candidate already fit. Bound the actual search range and validate the largest candidate first.
friction: Template/planner overflow handling counted the same full input in both budgetRenderedText and fitExcerpt. The already-proven overflow now goes directly to bounded excerpt rendering.
evidence: At base 4b7342cd, the same six uncached behavioral tests passed before and after: package execution 3.952 s to 3.043 s; aggregate overflow 0.88 s to 0.40 s; aggregate completeness 0.60 s to 0.26 s. Index fallback remained about 1.4 s. These measurements cover application behavior, not tokenizer performance.
