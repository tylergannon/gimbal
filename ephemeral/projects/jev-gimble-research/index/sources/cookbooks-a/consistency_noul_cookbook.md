# Self-consistency: Noul questions

## Purpose

This cookbook repeats a 14-question insurance-claim rubric 15 times and compares Jev’s P(true) outputs with probability- and hard-answer modes from general models. It demonstrates why continuous supervision should use an uncertainty band rather than a single `0.5` cut and why raw probabilities must remain visible after policy routing.

## Key concepts and evidence

- A `Noul` answers one True/False semantic question with P(true). The operational risk is threshold sensitivity: small movements around a cutoff can change pay/deny/review or, by analogy, continue/steer/stop decisions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:9-17`
- The state contains both clear facts and deliberate borderline judgments: track-day location, uncovered rental item, missing police report, and premature automated approval. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:101-113`
- Questions are phrased so “yes” consistently means the checked property is true. This orientation discipline makes probability rows comparable and prevents downstream inversions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:160-182`
- A fresh irrelevant `uid` changes each repeat for both general LLMs and Jev. The authors explicitly note that the setup cannot distinguish UID sensitivity from identical-request variation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:185-202`
- The application maps `<0.30` to `no`, inclusive `0.30-0.70` to `uncertain`, and `>0.70` to `yes`, entirely in code and without another Jev request. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:623-646`

## Measured examples

- Jev’s mean per-question probability SD was `0.0102`, below all tested general-LLM probability conditions in this run. Its `covered` answer ranged `0.43-0.53`, crossing the naïve `0.5` threshold. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:29-36`
- All 15 `jev-latest` requests resolved to `jev-1.13.0`, making concrete model version observable. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:443-468`
- Jev averaged `111ms` and historical estimated `$0.000043` per full 14-question rubric. General-model conditions averaged `1.1-13.9s`; the document warns prices are historical, not current verified billing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:470-515`
- Jev varied most on `covered` (`0.43-0.53`) and `exclusion` (`0.53-0.62`). Of 14 questions, only `covered` crossed `0.5`; factual checks were steadier, while judgment-heavy questions varied more across general models. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:517-528` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:618-621`

## Citation bookmarks

- Experiment summary and key result: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:5-36`
- State and rubric design: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:101-182`
- Request/parse caveats: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:185-209`
- Experiment grid and versions: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:365-468`
- Uncertainty band: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:623-683`

## Themes for continuous supervision

- **Independent signals beat forced labels:** ask separate Nouls for “repeating?”, “blocked?”, “missing evidence?”, “contradicted?”, and “needs user decision?” so multiple conditions can coexist.
- **Use bands, not razor cutoffs:** a wide review interval prevents tiny probability movements from creating opposite automated interventions.
- **Make orientation obvious:** every question should make P(true)’s operational meaning unambiguous.
- **Preserve continuous values:** policy decisions are a view over probabilities, not a replacement for them.

## Gotchas and failure modes

- Temperature zero did not guarantee repeatability for general models; hard yes/no mode hides uncertainty rather than removing it. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:19-32`
- General-model JSON formatting failed despite strict prompts (Haiku commonly added fenced JSON), so parse failure must be its own state rather than an implied low probability. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:204-207`
- The `0.30-0.70` band is illustrative, neither calibrated nor optimized. Production boundaries need labeled examples and explicit error/review costs. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:632-635`
- A review band adds two new edges; values near `0.30` or `0.70` may still alternate between automatic and uncertain, and clearing the band does not prove correctness. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_noul_cookbook.md:680-683`
- This is one constructed claim, not an accuracy or calibration benchmark.

## Task recipes

1. **Build a supervision rubric:** define independent, positively oriented Nouls over one bounded trace window; batch all questions in one `system_one` call; retain model version and every raw probability.
2. **Route with three-way policy:** below a calibrated low boundary, take no intervention; above a high boundary, take the bounded intervention; between them, ask a larger supervisor/human or collect more context.
3. **Regression-test stability:** repeat a frozen suite of clear and borderline traces across model upgrades, harmless metadata changes, and identical requests; track per-question ranges, threshold crossings, and review-load changes.
