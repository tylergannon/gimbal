# Decision-model premise

## Purpose

This leaf records TypeSafe’s stated model philosophy: optimize a narrow decision model for calibrated, machine-consumable outputs rather than optimize a conversational model for preferred prose. It is useful when judging whether Jev belongs in a Gimble supervision path at all.

## Key concepts

- **Machine-native intelligence is the design target.** TypeSafe names structure, reliability, observability, testability, speed, consistency, and low cost as software-like properties for AI-to-AI and AI-to-software interaction. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:9-19`
- **Jev is deliberately narrow.** The product is framed for production code that needs an inspectable decision, not for a model that does everything. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:15-19`
- **RLCD changes the output contract.** TypeSafe says its training objective returns decisions and probabilities rather than generated text, with higher probability intended to correspond to a greater empirical chance of correctness. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:23-38` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:49-56`
- **Calibration is a population property.** Predictions assigned 0.2, 0.8, or 1.0 should be correct at the corresponding frequencies across groups of predictions; no probability is a guarantee for one answer. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:57-63`
- **Persuasiveness is not sufficient proof for automation.** The source warns that human-preferred text can still be unreliable for unattended action and distinguishes conversational optimization from constrained-decision optimization. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:65-79` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:91-91`

## Citation bookmarks

- Machine-native design properties: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:9-19`
- RLHF/RLVR/RLCD comparison: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:23-41`
- Calibration semantics and non-guarantee: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:49-63`
- Automation warning: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:65-91`

## Themes

- Evaluate Jev on the reliability of repeated decisions, not on how persuasive an individual answer looks.
- Observability comes from stable typed outputs plus code-visible policy.
- A generative supervisor and a decision model have complementary optimization targets.

## Gotchas

- Calibration must be measured on representative labeled data; the primer’s calibration explanation is a goal/contract, not evidence that a particular Gimble coaching classifier is calibrated on Gimble runs.
- A reported confidence or probability should not be treated as certainty for a single intervention. The workflow still needs consequences-aware routing and fallback.
- The document makes product-positioning claims but does not provide benchmark data, training-set details, or supervision-specific evaluation results in this assigned source.

## Task recipes

- **Assess a coaching detector:** assemble labeled historical windows for one atomic coaching need, bucket predictions by probability, and compare observed positive rate with predicted rate. Anchor the interpretation at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:49-63`.
- **Choose Jev versus an LLM:** use Jev when the output can be constrained to an inspectable fixed decision; retain an LLM for generation, conversation, or novel reasoning. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/machine-learning-primer.md:15-19`.

