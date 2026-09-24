# Intent routing

## Purpose

Intent routing uses Jev as a fast classifier in front of heterogeneous handlers. Application code can route each request to deterministic logic, a context-specific LLM, or a human, while confidence and a second complexity judgment constrain automation.

## Key concepts

- **Jev sits before—not inside—the handlers.** The pattern classifies once, then ordinary code invokes a database/code path, a specialist LLM, or a human according to the result. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md) [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- **Classify routing intent and difficulty together.** The customer-service example asks a Choice for intent and a Score for complexity in the same evaluation. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md) [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- **Use confidence as a fail-closed precondition.** Intent confidence below 0.5 routes directly to a human rather than selecting any automated handler. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- **Each label can own a different execution environment.** `order_status` invokes deterministic code; product and return questions invoke distinct specialist LLM contexts; complaints choose between a resolution LLM and a human. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- **A secondary answer can refine escalation.** Complaint handling escalates when complexity is high *or when the complexity judgment itself has low confidence*. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- **Selective invocation controls expensive resources.** The source’s stated payoff is that the fast classifier handles routing in one call and expensive LLM/human resources are invoked only when needed. [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)

## Citation bookmarks

- Pattern boundary and handler types: [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- Intent/complexity question contract: [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- Full code route with confidence and complexity gates: [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)
- Resource-selection rationale: [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md)

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

- **Triage a possible supervision event:** classify `no_action`, `deterministic_check`, `agent_coaching`, and `human_authority`; ask a parallel complexity/risk score; route low-confidence or high-risk cases to a human. Start from [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md).
- **Load context only when needed:** use the selected coaching intent to invoke a specialist generative agent with the relevant Gimble semantic-index slice, leaving cheap/no-op and deterministic validation paths agent-free. The source analogue is [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md).
- **Fail closed:** when intent or risk is uncertain, observe and escalate rather than steering automatically. Start at [intent routing](https://docs.typesafe.ai/patterns/intent-routing.md).

