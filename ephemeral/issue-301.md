# Advisory semantic-index quality feedback

Whenever a workflow builds or updates a semantic index, launch a bounded evaluation sidecar and let the parent continue without waiting. Start with research-document. This replaces the earlier acceptance-gate proposal following Tyler's direction: practical feedback now, not a rigorous benchmark or completion gate.

The sidecar receives the task goal, local corpus path, and index entrypoint. It samples three useful questions from original sources with supporting passages, asks fresh cheap-model sessions to retrieve answers through the index, then assesses those answers against the sources. Expected passages are withheld from retrieval prompts. Use Luna initially; no outside research or automatic repairs.

Post concise advisory feedback linked from the originating run: what was found correctly, missed, or unsupported; concrete routing improvements with affected paths; observed retrieval calls, recorded returned-text volume, elapsed time, and provider tokens where available. Do not substitute self-reported counts or the skill's structural evaluator for observed retrieval. Explain unavailable accounting and the limitations of this small sample.

Keep one evaluation pass and a finite deadline. Sidecar launch/evaluation failures must be visible but must not fail the parent. Preserve the existing document editorial gate and ordinary-Go workflow style. No new general evaluation framework, held-out benchmark suite, budget acceptance gate, or repair loop is requested. Retiring or repairing the skill's old structural evaluator is outside this first integration.

Demonstrate useful cheap-model feedback on a real local index, parent progress independent of the sidecar, visible incomplete outcomes, and withheld expected evidence in fresh retrieval prompts. Describe what was observed in the PR/chat; do not commit run artifacts.
