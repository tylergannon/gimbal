# JavaScript Score question and response semantics

## Purpose

This leaf covers the ordered-rubric primitive end to end. A Score is not a selected integer label: Jev returns an expected numeric score that may lie between rubric levels, the distribution over levels, the rubric legend, and reported confidence.

## Question contract

- `ScoreQuestion<T>` is a question that assigns a score using an ordered rubric; `T` extends `ScoreCriteria`. [ScoreQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreQuestion.md)
- `criteria: T` holds descriptions of available outcomes. [ScoreQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreQuestion.md)
- `instructions` is optional/nullable `EntryType`, and the discriminator is `type: "score"`. [ScoreQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreQuestion.md)

## Response contract

- `ScoreResponse<T>` is documented as an expected score with its rubric and probabilities, parameterized by the originating `ScoreCriteria`. [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- `confidence` is a readonly number described as reported confidence in the score. [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- `legend: ScoreLegend<T>` preserves rubric descriptions keyed by score, enabling stored results to remain interpretable even if local rubric code later changes. [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- `probabilities` maps numeric or numeric-string score keys to probability values. [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- `score` is the expected score and may fall between integer rubric levels; the response is discriminated by readonly `type: "score"`. [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)

## Citation bookmarks

- Score question shape: [ScoreQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreQuestion.md)
- Expected score and confidence: [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md) [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- Legend and probability distribution: [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)
- Response discriminator: [ScoreResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ScoreResponse.md)

## Themes

- **Ordered semantic continuum:** use Score only when moving from level N to N+1 has coherent meaning.
- **Expected value, not hard class:** downstream code can retain nuance instead of rounding immediately.
- **Self-describing stored result:** persist `legend` with score/probabilities so historical evaluations retain their rubric context.
- **Distribution-aware policy:** expected score and confidence summarize different aspects; full probabilities reveal bimodality or boundary mass.

## Gotchas and gaps

- The response permits numeric and numeric-string probability keys; normalize keys before ordered iteration or arithmetic.
- A fractional expected score can hide a split between distant rubric levels. Always inspect/store the distribution for consequential routing.
- The docs do not define confidence calculation, calibration, or its mathematical relationship to probability spread.
- The question interface makes instructions optional, even though an ordered rubric without a clear question can be type-correct but semantically unusable.
- The assigned docs do not state maximum rubric length, runtime validation, normalization tolerance, or whether missing probability keys are possible.
- v0.6.0 changed Score criteria representation; see the compatibility leaf before porting older examples.

## Task recipes

1. **Model intervention severity:** define ordered observable levels such as `no intervention`, `watch`, `collect evidence`, `steer`, `stop/escalate`; preserve expected score and distribution; route with explicit application code.
2. **Detect ambiguous extremes:** flag distributions with meaningful mass on non-adjacent levels even when expected score sits calmly in the middle.
3. **Store the legend:** include returned legend and authored rubric version with every durable supervision sample.
4. **Validate locally:** require nonblank instructions, at least two ordered descriptions, finite probabilities, and a finite expected score within the rubric range.
