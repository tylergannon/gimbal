# Confidence-gated routing

## Purpose

Confidence-gated routing separates the predicted answer from permission to act: the answer selects a candidate path, while confidence and the consequences of error determine whether code executes it, requests confirmation, or escalates.

## Key concepts

- **Confidence is a second decision axis.** The page’s compact contract is “the answer tells you what; confidence tells you whether to act,” and it presents the pattern as a way to increase reliability and safety. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:5-7` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:238-242`
- **Risk should set the threshold.** In the voice-banking example, the same intent classifier permits a lower threshold for reading a balance than for approving a transfer because a wrong transfer has higher consequences. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:240-259`
- **Use a general uncertainty floor plus action-specific policy.** Below 0.6 any action goes to support; balance lookup proceeds at/above the floor; transfer approval proceeds automatically only above 0.85 and otherwise asks for confirmation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:281-304`
- **Fallback is part of the normal control flow.** The example explicitly routes unknown/other intents to a human and treats moderate-confidence high-stakes decisions as a verification step rather than a model failure. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:283-304`
- **Policy remains visible in code.** The numeric thresholds and branch-specific consequences are expressed in ordinary `if`/`elif` statements, not hidden in model prompts. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:281-306`

## Citation bookmarks

- Pattern contract: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:5-7`
- Risk-differentiated route diagram: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:240-259`
- Intent question: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:261-279`
- Code-owned thresholds and fallbacks: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:281-306`

## Themes

- Uncertainty is actionable state, not just telemetry.
- Thresholds belong to consequences and action classes, not to the model globally.
- Verification and human review are first-class routes.
- The safest default for an unrecognized branch is escalation, not action.

## Gotchas

- The values 0.6 and 0.85 are illustrative voice-banking policy, not recommended universal thresholds.
- A high confidence score does not establish that the question captures the right concept or that the model is calibrated on Gimble traces; both wording and empirical calibration need evaluation.
- An automated coaching nudge and an irreversible workflow mutation have different stakes and therefore should not share a threshold merely because they use the same predicted label.

## Task recipes

- **Gate Gimble interventions by consequence:** allow a low-stakes request for a status summary at a lower confidence; require stronger evidence for steering that changes scope or execution; route authority-sensitive or destructive actions to a human. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:240-259`.
- **Add a three-way policy:** below a global floor do nothing/escalate; in the middle ask a cheap generative supervisor or request confirmation; above an action-specific threshold perform the bounded intervention. The source analogue is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/confidence-routing.md:281-306`.
- **Tune safely:** label historical decisions by action class, sweep thresholds against false-action and missed-intervention costs, then keep the selected constants next to the questions for human review.

