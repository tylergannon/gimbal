# Workflow model cost guidance

## Correction

The built-in workflows spend frontier-model tokens in repeated synthesis,
editorial, planning, and critique roles even when a caller's project is clearly
routine. Treating every configured default as untouchable prevents an informed
caller from tuning those costs down.

## Decision

Keep the exact configured defaults visible and unchanged until the provider
evaluation has evidence for replacing them. Add workflow-specific help that
names the expensive roles, explains when their strength is justified, and gives
one deliberate Sol/Opus override for bounded routine work. Update the Gimble
runner skill to permit that documented judgment without inviting blanket role
overrides.
