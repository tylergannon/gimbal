# Intent routing

## Purpose

Intent routing uses Jev as a fast classifier in front of heterogeneous handlers. Application code can route each request to deterministic logic, a context-specific LLM, or a human, while confidence and a second complexity judgment constrain automation.

## Key concepts

- **Jev sits before—not inside—the handlers.** The pattern classifies once, then ordinary code invokes a database/code path, a specialist LLM, or a human according to the result. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:5-7` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:238-243`
- **Classify routing intent and difficulty together.** The customer-service example asks a Choice for intent and a Score for complexity in the same evaluation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:244-267` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:269-297`
- **Use confidence as a fail-closed precondition.** Intent confidence below 0.5 routes directly to a human rather than selecting any automated handler. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:299-309`
- **Each label can own a different execution environment.** `order_status` invokes deterministic code; product and return questions invoke distinct specialist LLM contexts; complaints choose between a resolution LLM and a human. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:310-327`
- **A secondary answer can refine escalation.** Complaint handling escalates when complexity is high *or when the complexity judgment itself has low confidence*. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:319-331`
- **Selective invocation controls expensive resources.** The source’s stated payoff is that the fast classifier handles routing in one call and expensive LLM/human resources are invoked only when needed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:329-331`

## Citation bookmarks

- Pattern boundary and handler types: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:238-267`
- Intent/complexity question contract: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:269-297`
- Full code route with confidence and complexity gates: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:299-327`
- Resource-selection rationale: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:329-331`

## Themes

- Choose the cheapest capable handler after a bounded classification.
- Context specialization remains in the downstream handler; Jev selects it.
- Complexity and uncertainty both push toward escalation.
- Code retains routing authority and can keep a deterministic lane completely free of LLMs.

## Gotchas

- The fixed intent labels must be exhaustive enough for the application or include a safe `other`/fallback route; the example relies on a confidence floor but does not show unknown-label evolution.
- Complexity is a separate uncertain model judgment, so thresholding its value without checking its confidence can create false automation. The example explicitly checks both.
- The 0.5 confidence threshold is illustrative and must be validated for each Gimble coaching route and cost of error.

## Task recipes

- **Triage a possible supervision event:** classify `no_action`, `deterministic_check`, `agent_coaching`, and `human_authority`; ask a parallel complexity/risk score; route low-confidence or high-risk cases to a human. Start from `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:269-327`.
- **Load context only when needed:** use the selected coaching intent to invoke a specialist generative agent with the relevant Gimble semantic-index slice, leaving cheap/no-op and deterministic validation paths agent-free. The source analogue is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:310-331`.
- **Fail closed:** when intent or risk is uncertain, observe and escalate rather than steering automatically. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/intent-routing.md:299-331`.

