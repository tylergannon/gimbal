# Knowledge graph entity alignment

## Purpose

This cookbook routes 450 candidate entity pairs using one three-level Jev `Score`: different product, related/uncertain, or same product. Three companion Nouls explain field agreement for curator review. The key supervision pattern is to put the review state inside an ordered semantic rubric rather than retrofit a numerical uncertainty threshold later.

## Key concepts and evidence

- Asymmetric risk motivates three outcomes: a false merge contaminates every attached fact and is costly to undo, while a missed match leaves a duplicate. The middle level exists for cases unsafe to merge or drop. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:14-30`
- `Score` preserves the ordered relationship of different → related → same and attaches semantic criteria directly to each outcome. The document argues a Noul would require fitted thresholds and a Choice would discard order. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:26-44`
- Three Nouls for same name, brewery, and style ride in the same request and provide curator diagnostics. ABV comparison is deliberately left to arithmetic in code. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:147-167`
- The route rounds the continuous Score to the nearest of three semantic levels; the middle criterion explicitly covers variants, special editions, and ambiguous naming. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:152-176` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:195-220`

## Measured examples

- Candidate corpus: 450 pairs from the Magellan Beer benchmark, with raw noisy text left unprocessed; each entity has name, brewery, style, and ABV. One request is made per preselected pair. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:107-130`
- Clear same pair `c446` scored `1.94`, confidence `0.92`; clear different `c427` scored `0.03`, confidence `0.95`. Ambiguous `c100` scored `1.30`, confidence `0.27`, and `c428` scored `1.10`, confidence `0.77`; both went to curator review. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:239-269`
- Overall routing: 40/450 (`8.9%`) asserted same, 50/450 (`11.1%`) went to the curator, and 360/450 (`80.0%`) stayed unlinked. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:325-340`
- Boundary density was asymmetric: nine pairs within `0.1` of the merge cut at `1.5`, but 47 within `0.1` of the lower review cut at `0.5`. The middle-level wording controls this distribution. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:342-352`

## Citation bookmarks

- Risk model and three outcomes: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:9-44`
- Candidate data caveats: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:107-120`
- Score/Noul specification and route: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:147-220`
- Distribution and cut-point interpretation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:271-352`

## Themes for continuous supervision

- **Semantic middle outcome:** define “possibly needs coaching” directly rather than forcing intervention/no-intervention and bolting on an arbitrary confidence threshold.
- **Asymmetric cost:** false steering can be more harmful than a missed opportunity; ordered levels should reflect that asymmetry.
- **Explain review with companions:** the route may be one Score, while Nouls identify which observable dimensions disagree.
- **Candidate generation remains separate:** cheap deterministic or embedding methods narrow the set; Jev performs the expensive judgment on candidates.

## Gotchas and failure modes

- The cookbook reports route counts but no precision/recall against `known_same_as`, despite the benchmark labels being present. It therefore does not demonstrate alignment accuracy or false-merge safety.
- “No threshold to fit” is only partly true: rounding creates fixed cut points at `0.5` and `1.5`. They follow from rubric geometry, but outcome quality still depends heavily on level wording.
- Score confidence is not used in routing. `c100` at confidence `0.27` and `c428` at `0.77` both receive the same curator route, which may be acceptable but loses review-priority information.
- Raw data noise is tolerated here but could generate systematic false positives; deterministic normalization/arithmetic should be evaluated separately.
- Public endpoint concurrency is limited: the cookbook uses six workers and notes rate limiting above roughly eight. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/entity_alignment.md:94-104`

## Task recipes

1. **Three-level intervention score:** write observable criteria for `no coaching needed`, `ambiguous/collect more evidence`, and `coaching warranted`; round Jev Score to the corresponding route; keep intervention consequences in Gimble code.
2. **Add diagnostic Nouls:** in the same request ask whether the run is making progress, repeating, ignoring evidence, blocked on external input, or violating scope; show these only to the curator/supervisor for middle cases.
3. **Prove safety before automation:** label a representative run corpus, measure false-intervention and missed-intervention rates at both Score cut points, prioritize examples near the action boundary, and use Score confidence only as an additional review-priority signal after calibration.
