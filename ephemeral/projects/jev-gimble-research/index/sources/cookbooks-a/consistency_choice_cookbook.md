# Self-consistency: Choice questions

## Purpose

This cookbook repeats an eight-question moderation rubric 15 times across Jev and several general LLM conditions, then distinguishes raw label agreement, probability stability, application agreement after abstention, and automatic-action coverage. Its central supervision lesson is that stable-looking top labels can still flip, and abstention can stabilize policy without making the underlying model deterministic or correct.

## Key concepts and evidence

- The same borderline post and eight fixed-label `Choice` questions are repeated 15 times per condition. A fresh irrelevant `uid` changes every request, so the experiment measures operational repeatability under harmless state variation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:9-30` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:237-250`
- The application returns `uncertain` when the top probability is below `0.60`; parse failures remain distinct, and single-pick models cannot participate in uncertainty evaluation because their one-hot vectors contain no calibrated uncertainty information. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:254-280` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:935-948`
- Model aliases are resolved and recorded: 15 `jev-latest` calls all returned `jev-1.13.0`, guarding analysis against a silent alias change inside a run. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:557-581`
- Three metrics remain separate: raw plurality agreement, policy agreement including `uncertain`, and automatic-action/abstention share. None measures accuracy. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:950-1008`

## Measured examples

- Jev’s raw top-label agreement was `90.8%`. It flipped on two of eight questions: primary risk (`Harassment` 11 versus `Violence` 4) and link handling (`RmLink` 8 versus `Brigade` 7). Both became consistently `uncertain` under the `0.60` policy. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:781-787`
- Jev’s mean per-label probability SD was `0.0098`, max `0.0515`. Haiku at temperature zero was lower at `0.0012`; the other five probability-output LLM conditions were `2.5x-5.6x` Jev’s mean. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:841-855`
- With abstention, Jev policy agreement rose to `99.2%`, with `74.2%` automatic answers and `25.8%` uncertain. No Jev question produced two different concrete actions after uncertainty was removed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:993-1015`
- Jev averaged `114ms` and historical estimated `$0.000046` per full eight-question rubric. Compared LLM conditions averaged `826ms-13.0s`; pricing is explicitly historical and not verified current billing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:584-630`

## Citation bookmarks

- Experiment summary: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:5-41`
- Borderline state and rubric: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:106-234`
- Experiment grid: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:474-496`
- Probability variation table: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:789-855`
- Abstention policy and result table: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:935-1015`

## Themes for continuous supervision

- **Repeatability is measurable:** replay identical supervision questions with irrelevant UID variation before trusting a route that will run continuously.
- **Abstain before acting:** low top probability can map competing labels to one review outcome, reducing unstable automated interventions.
- **Keep raw and policy layers:** retain full probability vectors even when application policy emits a single label or `uncertain`.
- **Version every result:** record requested alias and returned concrete Jev version in long-running supervision systems.

## Gotchas and failure modes

- The experiment cannot separate sensitivity to the changing irrelevant UID from variation on byte-identical requests. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:247-250`
- High repeatability is not accuracy. Haiku t=0 reached `100%` here, but the cookbook explicitly warns that this says nothing about correctness. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:915-920`
- The `0.60` threshold is illustrative, not calibrated or selected for optimal agreement. Production policy must use labeled cases plus relative costs of incorrect action and review. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:937-944`
- Abstention creates new edges: probabilities near `0.60` can still flip between a concrete label and `uncertain`; the underlying model is no more deterministic. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/consistency_choice_cookbook.md:1050-1053`
- Label design can force a false single “primary” category for multi-faceted situations. Continuous coaching may need independent Nouls rather than mutually exclusive Choices.

## Task recipes

1. **Stability-test coaching routes:** choose borderline real run windows; repeat the same Choice rubric 15+ times with irrelevant metadata changes; report top-label agreement, per-label probability SD, parse failures, abstention rate, and concrete-action conflicts.
2. **Set an intervention threshold:** use labeled coaching outcomes and asymmetric costs; sweep top-probability thresholds; choose a point based on false interventions versus human/agent review load, not repeatability alone.
3. **Monitor drift:** record concrete model versions and periodically replay a frozen ambiguity suite. Alert on changes in raw distributions even when the final abstention policy still masks label flips.
