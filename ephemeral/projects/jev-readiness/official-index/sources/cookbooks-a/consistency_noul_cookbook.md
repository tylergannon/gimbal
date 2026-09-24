# Self-consistency: Noul questions

## Purpose

This cookbook repeats a 14-question insurance-claim rubric 15 times and compares Jev’s P(true) outputs with probability- and hard-answer modes from general models. It demonstrates why continuous supervision should use an uncertainty band rather than a single `0.5` cut and why raw probabilities must remain visible after policy routing.

## Key concepts and evidence

- A `Noul` answers one True/False semantic question with P(true). The operational risk is threshold sensitivity: small movements around a cutoff can change pay/deny/review or, by analogy, continue/steer/stop decisions. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- The state contains both clear facts and deliberate borderline judgments: track-day location, uncovered rental item, missing police report, and premature automated approval. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Questions are phrased so “yes” consistently means the checked property is true. This orientation discipline makes probability rows comparable and prevents downstream inversions. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- A fresh irrelevant `uid` changes each repeat for both general LLMs and Jev. The authors explicitly note that the setup cannot distinguish UID sensitivity from identical-request variation. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- The application maps `<0.30` to `no`, inclusive `0.30-0.70` to `uncertain`, and `>0.70` to `yes`, entirely in code and without another Jev request. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)

## Measured examples

- Jev’s mean per-question probability SD was `0.0102`, below all tested general-LLM probability conditions in this run. Its `covered` answer ranged `0.43-0.53`, crossing the naïve `0.5` threshold. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- All 15 `jev-latest` requests resolved to `jev-1.13.0`, making concrete model version observable. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Jev averaged `111ms` and historical estimated `$0.000043` per full 14-question rubric. General-model conditions averaged `1.1-13.9s`; the document warns prices are historical, not current verified billing. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Jev varied most on `covered` (`0.43-0.53`) and `exclusion` (`0.53-0.62`). Of 14 questions, only `covered` crossed `0.5`; factual checks were steadier, while judgment-heavy questions varied more across general models. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md) [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)

## Citation bookmarks

- Experiment summary and key result: [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- State and rubric design: [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Request/parse caveats: [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Experiment grid and versions: [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- Uncertainty band: [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)

## Themes for continuous supervision

- **Independent signals beat forced labels:** ask separate Nouls for “repeating?”, “blocked?”, “missing evidence?”, “contradicted?”, and “needs user decision?” so multiple conditions can coexist.
- **Use bands, not razor cutoffs:** a wide review interval prevents tiny probability movements from creating opposite automated interventions.
- **Make orientation obvious:** every question should make P(true)’s operational meaning unambiguous.
- **Preserve continuous values:** policy decisions are a view over probabilities, not a replacement for them.

## Gotchas and failure modes

- Temperature zero did not guarantee repeatability for general models; hard yes/no mode hides uncertainty rather than removing it. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- General-model JSON formatting failed despite strict prompts (Haiku commonly added fenced JSON), so parse failure must be its own state rather than an implied low probability. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- The `0.30-0.70` band is illustrative, neither calibrated nor optimized. Production boundaries need labeled examples and explicit error/review costs. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- A review band adds two new edges; values near `0.30` or `0.70` may still alternate between automatic and uncertain, and clearing the band does not prove correctness. [consistency_noul_cookbook](https://docs.typesafe.ai/cookbooks/consistency_noul_cookbook.md)
- This is one constructed claim, not an accuracy or calibration benchmark.

## Task recipes

1. **Build a supervision rubric:** define independent, positively oriented Nouls over one bounded trace window; batch all questions in one `system_one` call; retain model version and every raw probability.
2. **Route with three-way policy:** below a calibrated low boundary, take no intervention; above a high boundary, take the bounded intervention; between them, ask a larger supervisor/human or collect more context.
3. **Regression-test stability:** repeat a frozen suite of clear and borderline traces across model upgrades, harmless metadata changes, and identical requests; track per-question ranges, threshold crossings, and review-load changes.
