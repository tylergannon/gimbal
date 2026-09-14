# Loop node: infer judge model is set independently, default Gemini 2.7 Flash

URL: https://github.com/tylergannon/gimble/issues/37
State: closed
Milestone: None
Updated: 2026-09-03T23:32:04Z

Branch `worktree-goal-gates`, `engine/loop.go` `judge`, `graph/parse.go` `applyDefaults`.

## Bug

The infer judge runs on the loop node's `llm_model` / `llm_provider` / `reasoning_effort`, and those inherit the pipeline `defaults` block. So the judge runs on whatever model the body runs on unless every loop node overrides it. The judge is meant to be a cheap model that looks at evidence files, often screenshots.

## Required behavior

The judge model is set independently of the body and has its own default.

- Default judge model: Gemini 2.7 Flash, provider `gemini` (routes to the `agy` harness).
- A loop node may override with its own `llm_model` / `llm_provider` / `reasoning_effort`.
- Pipeline `defaults` do not apply to the judge.

## Changes

- `graph/parse.go`: stop inheriting `LLMModel`, `LLMProvider`, `ReasoningEffort` from `defaults` onto `LoopNode`. Keep `Timeout` inheritance.
- `engine/loop.go` `judge`: when the loop node sets no model, use the judge default constant instead of the runner's `DefaultModel` / `DefaultProvider`.
- `internal/modelalias`: add a `flash` alias for the Gemini 2.7 Flash id so the default can be named.
- Test: a pipeline with `defaults.llm_model` set and a loop node with none runs the judge on the default judge model; an explicit loop-node model wins.
- Docs: `docs/spec.md` loop node field table; `src/content/docs/loops.md`.

