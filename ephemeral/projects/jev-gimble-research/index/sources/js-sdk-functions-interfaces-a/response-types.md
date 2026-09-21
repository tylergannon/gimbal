# JavaScript response typing: Choice and Noul

## Purpose

This leaf describes the response surfaces most relevant to continuous supervision: a typed selected label plus confidence/distribution for `Choice`, and a direct P(yes) value for `Noul`. These should be stored as measurements; application routes should be derived separately so thresholds can be replayed without another Jev call.

## Response types

### `ChoiceResponse<T>`

- The interface represents a selected label and its probabilities, parameterized by the originating `ChoiceCriteria` type `T`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:5-14`
- `choice` is typed as `keyof T & string`, preserving the criteria object’s string labels in the winning value. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:15-25`
- `confidence` is a readonly number described as reported confidence in the selected label. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:27-38`
- `probabilities` is readonly and maps labels to numbers, but the rendered type is broad—`{ readonly [label in string | number | symbol]: number }`—rather than visibly keyed by `keyof T`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:39-49`
- The discriminator is readonly `type: "choice"`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:51-59`

### `NoulResponse`

- `noul` is a readonly number documented as the probability of a yes answer from zero to one. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulResponse.md:5-20`
- The discriminator is readonly `type: "noul"`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulResponse.md:21-29`

## Citation bookmarks

- Choice selected label: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:19-25`
- Choice confidence: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:29-38`
- Choice distribution: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceResponse.md:41-49`
- Noul probability: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulResponse.md:11-20`

## Themes

- **Discriminated unions:** response `type` fields let consumers narrow heterogeneous answer collections by primitive.
- **Retain more than the winner:** store Choice probability vectors and confidence, not only `choice`; they support abstention, diagnostics, and later policy changes.
- **Noul is already continuous:** do not coerce P(yes) to boolean at the SDK boundary.
- **Readonly transport objects:** derive policy results into separate records rather than mutating responses.

## Gotchas and gaps

- The docs define `confidence` only descriptively; they do not state its formula, calibration population, or relationship to the winning probability. Do not assume equivalence.
- Probability normalization, missing-label behavior, NaN handling, and runtime range enforcement are not documented in these interfaces. Validate before policy use.
- `ChoiceResponse.probabilities` appears less precisely typed than `choice`; callers may need to index through known criteria keys and runtime-check completeness.
- Noul has no separate confidence property. Its P(yes) is a semantic probability, not evidence that an automatic threshold decision is safe.
- The assigned interface set omits `ScoreResponse`, usage/token metadata, request IDs, response model/version, and top-level answer-map typing.

## Task recipes

1. **Persist measurements:** save primitive type, question key/version, selected label, full probability vector or Noul value, request/model metadata from the enclosing response, and timestamp.
2. **Derive policy separately:** calculate `automatic`, `uncertain`, or `review` from stored values in application code; never discard the raw response.
3. **Validate at the boundary:** assert finite values in `[0,1]`, all expected Choice labels present, and a tolerable distribution sum before triggering supervision.
4. **Compare model upgrades:** replay a frozen set and diff raw distributions, confidence, and derived actions independently.
