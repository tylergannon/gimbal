# Tool routing and message guardrails

## Purpose

This leaf covers two bounded control patterns: compiling natural-language requests into typed function calls over closed argument sets, and screening LLM inputs/outputs with atomic hazard checks before application code chooses pass, review, block, or support.

## Key concepts and measured results

- **Function routing maps directly onto ordinary typed functions.** The cookbook chooses one of ten trading functions and fills `Literal`, `list[Literal]`, and boolean arguments with Choice/Noul questions. Since Choice option keys are the actual accepted argument values, downstream code does not translate generated labels. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:28-47`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:77-109`
- **Open numeric, date, and free-text arguments are deliberately excluded.** The example only asks Jev about closed sets; unmodeled arguments keep their function defaults. This avoids pretending the model can safely synthesize arbitrary typed values, but means the dispatcher is incomplete for functions that require open values. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:112-115`
- **Optionality gets its own absolute check.** Each optional argument has a “was this stated?” Noul alongside the Choice that would select a value. If not stated, the dispatcher omits the argument and preserves the function default; without the Noul, Choice must pick something and can do so confidently. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:116-169`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:317-351`
- **Speculative fan-out makes one command a single request.** The dispatcher builds 54 questions covering function selection and every function's arguments, then reads only the selected function's answers. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:171-196`
- **The function-calling walkthrough shows 14 successful-looking curated commands, not an accuracy benchmark.** Examples fill multiple arguments, sets, flags, defaults, and function names; displayed minimum call confidence ranges from 0.53 to 1.00. No negative set, confusion matrix, or malformed-command evaluation is reported. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:198-266`
- **Composite call confidence is the weakest contributing judgment.** The example takes the minimum selected argument probability rather than multiplying probabilities, because one bad argument can spoil the call while a product mechanically shrinks as argument count grows. It also exposes the weakest argument for diagnosis. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:317-351`
- **Guardrails are decomposed into hazards plus severity.** The cookbook runs separate input and output batteries: four Nouls detect particular hazards and one Score estimates consequence. Application policy—not Jev—maps threshold crossings to pass, review, block, or support. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:5-26`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:125-208`
- **Input and output require different questions.** Input checks ask whether a user requests unsafe conduct; output checks ask whether the assistant actually complied or supplied it. This prevents a safe refusal that mentions harmful content from being treated like a harmful answer. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:181-208`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:325-339`
- **Routing thresholds are explicitly product policy.** Each Noul has a lower review threshold and higher action threshold; severity can promote review to block, and action precedence resolves multiple signals. The same Jev result (`jailbreak=0.74`, severity `0.51`) blocks under the sample strict policy but reviews under the permissive one. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:211-274`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:346-370`
- **The guardrail demonstration covers 15 cached examples, not a held-out evaluation.** Ten prompts and five replies produced all four routes under hand-set thresholds: safe contextual violence passed, dosage ambiguity reviewed, a dosage answer blocked, self-harm routed to support, and two jailbreak examples blocked. Results came from `jev-1.12` on 2026-08-15. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:59-64`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:84-123`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:276-344`

## Important citation bookmarks

- Closed-set signature shapes and exclusions: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:77-115`
- Function-routing spec semantics: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:116-196`
- Function examples and confidence aggregation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:198-266`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:317-351`
- Guardrail question batteries: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:125-208`
- Guardrail policy implementation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:211-274`
- All displayed guardrail outcomes: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:276-344`

## Themes

- **Constrain before inference:** derive the legal action and argument space from program types, then let Jev choose within it.
- **Separate relative choice from absolute applicability:** a Choice always has a winner; a Noul is needed to preserve “not stated,” “none applies,” or “do not act.”
- **Policy remains ordinary code:** model probabilities are observations; thresholds, precedence, side effects, and fallbacks are visible application logic.
- **Check both sides of a model call:** preconditions and generated outcomes have different semantics and need distinct batteries.

## Gotchas and failures

- The function dispatcher is a routing demonstration, not tool-call validation. It does not prove that arguments satisfy non-enum schemas, that the function is safe to execute, or that its result answers the request.
- One request asks 54 speculative questions, but the cookbook reports only 14 selected outputs and no adversarial, out-of-domain, or “no tool applies” cases. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:171-196`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:198-259`
- Defaults can hide missing understanding: leaving an unmodeled numeric/date/free-text parameter at its default is safe only if that default is semantically acceptable. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:112-115`
- The guardrail set is tiny and selected to illustrate routing. It establishes mechanics, not production recall, false-positive rate, or jailbreak robustness.
- Guarding agent outputs with the same text model family does not create a hard security boundary. Treat high-stakes blocks or destructive interventions as policy decisions that require independent testing and possibly another authority.

## Task recipes

### Route a Gimble supervision intervention through a closed action set

1. Define ordinary actions such as `send_template`, `request_evidence`, `pause_for_user`, `escalate_reviewer`, and `do_nothing`.
2. Derive Choice options from those names; use separate Nouls for whether intervention is warranted and whether optional parameters were actually present.
3. Compute composite confidence as the weakest required judgment, and do not execute below a risk-adjusted threshold. Start from `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/function_calling.md:317-351`.
4. Validate exact parameter schemas, permissions, budgets, and side-effect safety in Gimble code before execution.

### Add two-sided supervision checks

1. For agent input/state, ask whether the next intended action violates authority, lacks evidence, or repeats an exhausted tactic.
2. For the resulting message/tool call, ask distinct questions about what it actually did: unsupported claim, wrong tool purpose, risky side effect, or failure to address the result.
3. Map each high-probability condition to a domain-specific action and set a review band below it. Use the policy pattern at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/llm_guardrails.md:211-274`.
4. Validate on labeled run traces; the cookbook's sample thresholds are illustrative only.

## Gaps

- No production-scale tool-routing accuracy, argument-level accuracy, or “no tool” evaluation.
- No tool-result validation, effect verification, permission model, or rollback design.
- No guardrail precision/recall, class prevalence, calibration curve, or adaptive-attack benchmark.

