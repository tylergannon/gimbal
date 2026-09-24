# Classification using confidence

## Purpose

This cookbook classifies SEC filing text into one of 75 industry groups, then uses Jev `Choice.confidence` to choose between the specific group and its deterministic parent division. The supervision pattern is graceful specificity: return an actionable narrow diagnosis when confidence is high, a broader safe category when it is not, and reserve human review for applications where broad routing is insufficient.

## Key concepts and evidence

- A single `Choice` carries all 75 group options. The answer returns the winning label, the full 75-way distribution, and a separate `confidence`; the cookbook warns not to substitute winner probability because the same winning mass can have very different runner-up structure. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- The taxonomy is built in code from 444 SEC SIC industries into 75 major groups and ten divisions. Parent fallback requires no second model call. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- At confidence `>=0.9`, code emits the major group; below it, code maps the same selected group to its division. If the broader answer is still unsafe, the cookbook explicitly places human handoff in this branch. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- Option descriptions matter: 33 of 75 groups lack umbrella titles, so criteria are synthesized from representative constituent industries rather than bare codes or incomplete names. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)

## Measured examples

- Dataset: 60 Item 1 “Business” sections, spanning 1993-2024 and 700-2,200 words. The SIC target is self-reported and can be stale, so the set was filtered to filings whose text supports the label. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- With cutoff `0.9`, 30 filings were high-confidence: 27/30 (`90%`) had the right group. The other 30 had only 12/30 (`40%`) correct groups; falling back to division made 21/30 (`70%`) useful. Overall useful answers rose from 39/60 forced groups to 48/60 confidence-broadened labels. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- The three lowest-confidence filings scored `0.22`, `0.23`, and `0.29`; two described intended future businesses and one had recently sold one of two segments. These are structurally ambiguous examples, not random failures. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)

## Citation bookmarks

- Confidence-based taxonomy fallback: [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- Taxonomy construction and option descriptions: [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- Choice distribution semantics: [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)
- Evaluation results: [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)

## Themes for continuous supervision

- **Hierarchical diagnoses:** classify a run into a narrow coaching type (`repeat-without-new-evidence`, `tool-interface-confusion`) when sure; fall back to a broader intervention family (`needs-direction`, `needs-verification`) when unsure.
- **Confidence controls specificity, not truth:** low confidence need not mean discard; it can select a safer coarser action.
- **One semantic call, deterministic roll-up:** keep parent maps and intervention consequences in Gimble code.
- **Criteria-rich labels:** describe each supervision class in observable terms; short label names alone are insufficient.

## Gotchas and failure modes

- The label set itself can be wrong or stale. SIC ground truth was self-reported and required manual filtering; a supervision taxonomy learned from historical agent interventions can encode reviewer habits instead of genuine benefit.
- The `0.9` cutoff is demonstrated on 60 selected examples, not a universal confidence calibration.
- Parent fallback only works when labels have a legitimate hierarchy and the selected child’s parent is still useful. Some coaching decisions have no safe broad action and must escalate.
- Accuracy improved under a changed target granularity; `70%` division correctness is not comparable to `40%` group correctness as the same task.
- A `Choice` can handle roughly 240 options according to this cookbook, but large flat taxonomies are harder to describe and audit. [classification_using_confidence](https://docs.typesafe.ai/cookbooks/classification_using_confidence.md)

## Task recipes

1. **Coarse-to-specific coaching:** define a two-level taxonomy of intervention family and concrete tactic; ask one Choice over concrete tactics; emit the tactic above a validated confidence cutoff, otherwise emit the parent family or human review.
2. **Audit confusion:** retain runner-up probabilities and examine low-confidence traces. Cluster them into true ambiguity, missing state, taxonomy overlap, and noisy labels before changing thresholds.
3. **Evaluate honestly:** score narrow and broad outcomes separately, publish coverage at each specificity, and include a no-action/human-review cost model rather than presenting coarser answers as equivalent correctness.
