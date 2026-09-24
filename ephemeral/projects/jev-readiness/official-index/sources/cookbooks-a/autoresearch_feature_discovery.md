# Autoresearch feature discovery

## Purpose

This cookbook uses a larger proposer model to invent semantic questions, Jev to answer those questions across labeled free text, and CatBoost plus held-out evaluation to accept, revise, or drop features. For Gimble, this is the clearest published template for learning scalable supervision signals from examples of “this run needed this kind of coaching” without making Jev itself the policy learner.

## Key concepts and evidence

- The loop is explicit: an LLM proposes questions, Jev turns text into numeric features, CatBoost trains on them, and the next proposal reads feature importance plus the worst-predicted rows. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- `Score` questions become two numeric features—the expected rubric level and distribution spread—while `Noul` questions become one P(true) feature. The final 38-question design produced 67 model columns. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- The proposer has structured `add`, `revise`, and `drop` actions. It is instructed to favor features that are answerable from the text, vary across rows, and add information not already represented. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- After round 1, examples are selected from both the largest out-of-fold errors and the best predictions. This contrasts missing distinctions with already-covered cases instead of feeding only failures. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Every new question is answered for every row; eight requests run concurrently. Score distributions can be encoded as mean only, mean plus spread, or every probability. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Candidate additions are rejected if their encoded column is too flat. Revisions and drops are accepted only if cross-validated dev RMSE does not worsen (configured here with zero tolerance). No extra API call is needed for model refits. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md) [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- The evaluation design keeps 1,200 dev rows for loop decisions and 800 labels untouched until final scoring; five-fold, three-repeat out-of-fold predictions drive feature changes and example selection. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md) [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)

## Measured examples

- Dataset: 2,000 wine reviews, with 1,200 dev and 800 held out; scores ranged 80-98 with mean `88.73` and SD `3.17`. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Held-out RMSE improved from mean baseline `3.088`, word-count CatBoost `2.466`, and direct Jev score `2.145`, to `1.869` after one proposal round and `1.772` after five rounds; Spearman rose to `0.799`. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Round 1 added 18 features with dev CV RMSE `1.903`; round 5 ended with 38 features and dev CV RMSE `1.840`, after both accepted and rejected rewordings/drops. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- The held-out gain from round 1 to round 5 was `-0.097` RMSE points with paired bootstrap 95% CI `[-0.147, -0.050]`. Most gain came from the first proposal, but iterative error feedback had measurable value. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- The final set contained 29 Score and 9 Noul questions. The leading feature, overall tone positivity, held `17.4%` of normalized CatBoost feature importance; a prestige-signal Noul held `7.2%`. Importance is not accuracy share. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)

## Citation bookmarks

- Overall design and baseline table: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Proposal contract: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Error-focused example selection: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Loop implementation: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Pseudocode and anti-prefilter rationale: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Generalization and next steps: [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md) [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)

## Themes for continuous supervision

- **Learn supervision features, not opaque labels:** propose questions such as “is the worker repeating an approach?”, “did evidence invalidate the plan?”, or “is the next action underivable?”; retain named, inspectable Jev probabilities as features.
- **Error-driven discovery:** show a proposer both run examples where the current supervisor failed and ones it handled well, plus previous predictions and feature importance.
- **Cheap semantic featurization, conventional learner:** Jev can run continuously across traces; CatBoost/logistic regression can learn coaching routes from human or high-quality agent labels.
- **Distribution-aware features:** score spread can reveal semantic ambiguity separately from the expected rubric level.

## Gotchas and failure modes

- Scale is request-bound: one request per row per round, independent of question count. The cookbook warns that 100,000 rows means 100,000 requests each round and eight workers can already hit a shared-key rate limit. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- No candidate screening occurs before the costly all-row pass. The cookbook proposes screening answerability, single-meaning wording, applicability, and expected variation with Nouls as a future extension. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md) [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Additions go straight in unless flat; revisions/drops receive stricter CV tests. This asymmetry can grow correlated or weak features, and the authors explicitly suggest correlation pruning.
- The demonstration is one dataset and one discovery run. It recommends multiple seeds/slices, deployment-shaped splits, untouched final test data, plateau stops, alternative learners, and embedding/simple-feature baselines. [autoresearch_feature_discovery](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery.md)
- Bigger general models remain necessary for open-ended feature proposal and revision. Jev is the high-volume measurement layer, not the creative search mechanism.
- Numeric improvement does not itself establish a safe coaching action. Labels must encode actual desired intervention outcomes, and deployment evaluation must include false-positive costs and human-review load.

## Task recipes

1. **Discover coaching signals:** collect trace windows plus a label such as `no intervention`, `ask for evidence`, `redirect`, `clarify`, or `stop`; hold out entire runs/issue families; ask a stronger proposer for Jev Score/Noul features; fit an interpretable learner; feed high-error examples into the next round.
2. **Keep the loop auditable:** store question wording/version, full Jev distributions, feature encodings, CV decisions, accepted/rejected actions, proposer model, and untouched test results. Do not store only the final route.
3. **Budget the search:** screen proposed questions on answerability and ambiguity before all-row evaluation; stop after a fixed request budget or CV plateau; use grouping/chronology to prevent near-duplicate run leakage.
4. **Retain an agent tier:** let continuous Jev features trigger routine policies; route novel, low-support, distribution-shifted, or high-impact cases to a larger model/human supervisor.
