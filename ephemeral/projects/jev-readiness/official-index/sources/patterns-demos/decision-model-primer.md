# Decision-model premise

## Purpose

This leaf records TypeSafe’s stated model philosophy: optimize a narrow decision model for calibrated, machine-consumable outputs rather than optimize a conversational model for preferred prose. It is useful when judging whether Jev belongs in a Gimble supervision path at all.

## Key concepts

- **Machine-native intelligence is the design target.** TypeSafe names structure, reliability, observability, testability, speed, consistency, and low cost as software-like properties for AI-to-AI and AI-to-software interaction. [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- **Jev is deliberately narrow.** The product is framed for production code that needs an inspectable decision, not for a model that does everything. [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- **RLCD changes the output contract.** TypeSafe says its training objective returns decisions and probabilities rather than generated text, with higher probability intended to correspond to a greater empirical chance of correctness. [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md) [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- **Calibration is a population property.** Predictions assigned 0.2, 0.8, or 1.0 should be correct at the corresponding frequencies across groups of predictions; no probability is a guarantee for one answer. [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- **Persuasiveness is not sufficient proof for automation.** The source warns that human-preferred text can still be unreliable for unattended action and distinguishes conversational optimization from constrained-decision optimization. [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md) [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)

## Citation bookmarks

- Machine-native design properties: [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- RLHF/RLVR/RLCD comparison: [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- Calibration semantics and non-guarantee: [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)
- Automation warning: [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md)

## Themes

- Evaluate Jev on the reliability of repeated decisions, not on how persuasive an individual answer looks.
- Observability comes from stable typed outputs plus code-visible policy.
- A generative supervisor and a decision model have complementary optimization targets.

## Gotchas

- Calibration must be measured on representative labeled data; the primer’s calibration explanation is a goal/contract, not evidence that a particular Gimble coaching classifier is calibrated on Gimble runs.
- A reported confidence or probability should not be treated as certainty for a single intervention. The workflow still needs consequences-aware routing and fallback.
- The document makes product-positioning claims but does not provide benchmark data, training-set details, or supervision-specific evaluation results in this assigned source.

## Task recipes

- **Assess a coaching detector:** assemble labeled historical windows for one atomic coaching need, bucket predictions by probability, and compare observed positive rate with predicted rate. Anchor the interpretation at [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md).
- **Choose Jev versus an LLM:** use Jev when the output can be constrained to an inspectable fixed decision; retain an LLM for generation, conversation, or novel reasoning. Start at [machine learning primer](https://docs.typesafe.ai/introduction/machine-learning-primer.md).

