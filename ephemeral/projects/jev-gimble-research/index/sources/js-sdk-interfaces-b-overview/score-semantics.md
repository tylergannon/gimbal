# JavaScript Score question and response semantics

## Purpose

This leaf covers the ordered-rubric primitive end to end. A Score is not a selected integer label: Jev returns an expected numeric score that may lie between rubric levels, the distribution over levels, the rubric legend, and reported confidence.

## Question contract

- `ScoreQuestion<T>` is a question that assigns a score using an ordered rubric; `T` extends `ScoreCriteria`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreQuestion.md:5-14`
- `criteria: T` holds descriptions of available outcomes. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreQuestion.md:15-25`
- `instructions` is optional/nullable `EntryType`, and the discriminator is `type: "score"`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreQuestion.md:27-47`

## Response contract

- `ScoreResponse<T>` is documented as an expected score with its rubric and probabilities, parameterized by the originating `ScoreCriteria`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:5-14`
- `confidence` is a readonly number described as reported confidence in the score. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:15-26`
- `legend: ScoreLegend<T>` preserves rubric descriptions keyed by score, enabling stored results to remain interpretable even if local rubric code later changes. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:27-38`
- `probabilities` maps numeric or numeric-string score keys to probability values. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:39-50`
- `score` is the expected score and may fall between integer rubric levels; the response is discriminated by readonly `type: "score"`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:51-71`

## Citation bookmarks

- Score question shape: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreQuestion.md:5-47`
- Expected score and confidence: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:5-26` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:53-62`
- Legend and probability distribution: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:29-50`
- Response discriminator: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ScoreResponse.md:63-71`

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
