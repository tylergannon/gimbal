# Jev as a programmatic supervision sensor for Gimble

## Purpose

This leaf translates Jev's documented architecture and known failure modes into a bounded Gimble supervision role. It distinguishes what Jev can continuously detect from what still requires Gimble code, a human, or a reasoning/generative agent.

## Key concepts

- **The strongest fit is a semantic sensor inside Gimble, not a supervisor agent replacement.** TypeSafe explicitly describes System One as “for building AI-powered software, not agents”: it neither generates code nor chooses its own next action. Code owns control flow and side effects; Jev supplies narrow common-sense judgments over unstructured data. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- **The product positioning directly includes harness and AI verification.** The use-case map proposes checking prompts, reasoning traces, tool calls, and other AI outputs for jailbreaks, citation errors, hallucinations, mistakes, and response-quality failures; it also names model routing, semantic context retrieval, trace classification, and guardrails as harness uses. [use case map](https://docs.typesafe.ai/concepts/use-case-map.md)
- **Tool-trace verification is a documented decomposition pattern.** The guide replaces one broad “are the tool calls correct?” judgment with separate Nouls for tool relevance, argument/schema match, identity links between calls and results, coordinate propagation, date match, and unit match. This is a close analogue for Gimble's existing event stream and makes errors attributable to a particular contract. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- **A scalable detector should ask many independent checks over one curated trace window.** All questions see the same state, run in parallel, and do not condition each other. Speculative checks can be computed up front and ignored when irrelevant. This makes one Jev call a plausible way to emit a vector of supervision signals at each meaningful Gimble event. [introduction](https://docs.typesafe.ai/introduction.md); [primitives](https://docs.typesafe.ai/primitives.md)
- **Jev can identify the intervention category but cannot write the intervention.** A Choice can pick among predeclared coaching modes, Nouls can detect particular faults, and Scores can prioritize. The actual steering message must come from a fixed template, retrieved example, human, or generative/reasoning model because Jev 1.13 is not trained for text generation. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md); [system one](https://docs.typesafe.ai/concepts/system-one.md)
- **Code should own the intervention policy.** Deterministic rules and side effects belong in code; Jev's probabilities can feed weighted rules or a downstream classical model. High-confidence, low-risk cases may trigger a template automatically; uncertain or high-risk cases should escalate to a person or reasoning model. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- **The trace state must be intentionally constructed.** Relevant goal, plan, recent model messages, tool arguments/results, policy constraints, and known outcomes can be labeled fields in a JSON object, and instructions should point to the relevant paths. Sending an entire long run is likely to reduce accuracy through distractors and context rot. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md); [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **Jev is intentionally literal and weak at multi-hop inference.** Supervision questions must express the exact observable condition, reduce indirection, and place boundary cases in criteria. Complex diagnoses should be decomposed into smaller checks and combined by code. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **Exact invariants should not be delegated to Jev.** Counting repeated attempts, comparing timestamps, checking budgets, validating JSON/schema mechanically, matching IDs, enforcing authority boundaries, and evaluating deterministic run status belong in code. Jev is documented as unreliable for counting, arithmetic, date ordering, and inferred structural identities. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **The agent trace is adversarial state whether or not the agent intended an attack.** Jev treats state as data but does not treat it as hostile by default; injected instructions and text arguing for its own classification can shift answers. Agent prompts, web pages, command output, and model prose can therefore steer the checker. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **A continuous supervision loop needs labeled outcome data.** The docs recommend plotting confidence against accuracy on the application's own data and note that probabilities can become features for a downstream classical model. Gimble should retain the trace slice, Jev version, full distributions, intervention taken, and observed outcome so thresholds and intervention policy can be evaluated rather than assumed. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md); [models](https://docs.typesafe.ai/models.md)
- **A second reasoning model remains the escalation mechanism.** The System One concept explicitly frames confidence as a way to decide when to act and when to escalate to a person or reasoning model. A practical Gimble design is Jev on the hot path for cheap triage, with selective expensive review rather than a second agent crawling every trace. [system one](https://docs.typesafe.ai/concepts/system-one.md)

## Important citation bookmarks

- “AI-powered software, not agents” architecture: [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- Composability claims (structured, parallel, fast, calibrated, consistent): [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- Tool-call trace decomposition: [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
- Harness, routing, and guardrail use cases: [use case map](https://docs.typesafe.ai/concepts/use-case-map.md)
- Context rot and adversarial-state warning: [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- Literalness, numeric, date, and indirection limits: [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- Non-generation boundary: [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)

## Themes

- **Observe continuously, intervene selectively:** Jev can cheaply narrow a large stream to the moments and categories that need richer review.
- **Semantic checks beside deterministic checks:** the model handles judgments such as relevance, contradiction, evidence support, or apparent confusion; code handles counts, chronology, schemas, permissions, and budgets.
- **A vector, not a verdict:** retain multiple atomic probabilities so the supervisor can explain which condition fired and learn better policies later.
- **Predeclared action space:** Jev works best when Gimble already knows possible intervention classes; it is not a discoverer of arbitrary new coaching text.
- **Evidence-driven calibration:** confidence gates and weights must be learned from representative runs, by model version and risk class.

## Gotchas and version limitations

- **Prompt injection is a first-order blocker for autonomous enforcement.** A monitored agent may quote hostile web content or instructions. Precise criteria and testing are mitigations in the docs, not a guarantee of isolation. Do not let Jev alone authorize destructive actions, secrets, or irreversible cancellation. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **Whole-transcript supervision conflicts with the documented relevance requirement.** Sliding windows, event-focused state construction, retrieval, or staged evaluation are likely necessary even before the 64k/32k hard limits. [models](https://docs.typesafe.ai/models.md); [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **No causal coaching claim is documented.** Detecting a trace pattern accurately does not show that the chosen intervention improves the run. Gimble needs outcome experiments that distinguish detection quality from intervention efficacy.
- **No explanation or supporting-span field is returned.** The primitive response contains typed values and probabilities, not a rationale or source citation. If a human reviewer needs evidence, Gimble must preserve the evaluated trace window and identify candidate evidence separately. [api](https://docs.typesafe.ai/api.md)
- **Jev cannot invent a novel remedy.** A Choice winner is constrained to supplied options, and the generation workaround is explicitly discouraged. [primitives](https://docs.typesafe.ai/primitives.md); [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- **Self-consistency is a design claim, not a guarantee in this source set.** The guide points to a separate cookbook for evidence; evaluate repeated-run variance on the intended Gimble rubric. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)

## Task recipes

### Prototype a Gimble “semantic tripwire”

1. Choose one observable, frequent, recoverable failure mode, such as unsupported completion claims after a failing tool result.
2. Build state from `goal`, `latest_agent_message`, `latest_tool_call`, `latest_tool_result`, and the relevant acceptance rule; do not send the full run.
3. Ask separate Nouls for “claim contradicts result,” “agent noticed the failure,” and “further work is possible.” Use Jev only for semantics; compute exit status and result presence in code. The decomposition precedent starts at [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md).
4. Run in shadow mode: record probabilities without steering. Hand-label false positives, false negatives, and ambiguous cases.
5. Only after calibration, attach a reversible prewritten nudge to a high-confidence/low-risk condition; route the middle band to a reasoning reviewer.

### Build a coaching taxonomy from examples

1. Collect real trace slices and label the minimum intervention that would have helped, including `none` and `unknown`.
2. Define a Choice whose options are actions Gimble can actually take. Give each option contrastive coverage, exclusions, and examples. Use [advanced](https://docs.typesafe.ai/primitives/advanced.md) as the rubric pattern.
3. Add independent Nouls for safety-critical traits rather than making the Choice carry all meaning.
4. Evaluate confusion matrices, probability calibration, and outcome improvement on held-out traces. Pin the model during the evaluation.
5. Use a template or reasoning model to produce the actual coaching text after the class is selected.

### Supervise a long-running agent without flooding Jev

1. Trigger evaluation on meaningful events (failed command, repeated tool, assertion of completion, request for authority), not every token.
2. Maintain deterministic counters and chronology in Gimble, then include their named textual summaries only when semantically relevant.
3. Construct a compact event window plus the current goal and applicable policy; retrieve older evidence only when a question needs it.
4. Ask the whole applicable atomic rubric in one call and ignore speculative answers that do not apply. Start at [primitives](https://docs.typesafe.ai/primitives.md).
5. Escalate only uncertain/high-risk cases to a generative reviewer, preserving the Jev signals as structured context.

### Evaluate whether Jev actually improves supervision

1. Establish a baseline: expert review hours, detected issue rate, false interventions, recovery rate, run success, and time-to-recovery.
2. Replay a representative labeled corpus in shadow mode with a pinned Jev version; retain full distributions and actual outcomes.
3. Compare detector precision/recall and calibration by failure type and stakes; choose separate thresholds per action.
4. A/B test interventions only on reversible cases, because classifier accuracy and coaching effectiveness are different claims.
5. Re-run before moving an alias or changing rubrics, because the docs warn aliases can change answers. [models](https://docs.typesafe.ai/models.md)

## Gaps left by this source segment

- No independent or vendor-published benchmark on agent traces, tool-use errors, prompt injection resistance, or coaching outcomes.
- No returned rationale, evidence spans, or citation attribution for a judgment.
- No documented streaming/session API; the described contract is stateless request/response, so Gimble must own event selection and state assembly.
- No native mechanism to learn from Gimble's examples; Jev's weights are shared and not customer-fine-tuned. Learning requires request examples/rubrics or a downstream model.
- No guidance on correlated question errors, temporal drift, or calibration under rare supervision failures.

