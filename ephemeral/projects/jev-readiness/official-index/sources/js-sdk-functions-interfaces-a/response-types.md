# JavaScript response typing: Choice and Noul

## Purpose

This leaf describes the response surfaces most relevant to continuous supervision: a typed selected label plus confidence/distribution for `Choice`, and a direct P(yes) value for `Noul`. These should be stored as measurements; application routes should be derived separately so thresholds can be replayed without another Jev call.

## Response types

### `ChoiceResponse<T>`

- The interface represents a selected label and its probabilities, parameterized by the originating `ChoiceCriteria` type `T`. [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- `choice` is typed as `keyof T & string`, preserving the criteria object’s string labels in the winning value. [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- `confidence` is a readonly number described as reported confidence in the selected label. [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- `probabilities` is readonly and maps labels to numbers, but the rendered type is broad—`{ readonly [label in string | number | symbol]: number }`—rather than visibly keyed by `keyof T`. [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- The discriminator is readonly `type: "choice"`. [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)

### `NoulResponse`

- `noul` is a readonly number documented as the probability of a yes answer from zero to one. [NoulResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulResponse.md)
- The discriminator is readonly `type: "noul"`. [NoulResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulResponse.md)

## Citation bookmarks

- Choice selected label: [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- Choice confidence: [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- Choice distribution: [ChoiceResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceResponse.md)
- Noul probability: [NoulResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulResponse.md)

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
