# Decision primitives, probability, and confidence

## Purpose

This leaf maps Jev's three output shapes to programmatic decisions and records the confidence semantics needed to gate automated supervision safely.

## Key concepts

- **Choice selects among a closed set.** A Choice question returns the highest-probability option, the full distribution over caller-supplied options, and a scalar confidence derived from that distribution. It is appropriate for routing or coaching-mode selection, not an open-ended response. [choice](https://docs.typesafe.ai/primitives/choice.md)
- **Choice criteria are part of the model input.** Option names and descriptions are visible to the model, while question IDs are not. Use descriptions to distinguish adjacent interventions, include `other`/`none of the above` when the set is not exhaustive, and use structured `what`, `not_for`, and examples when boundaries remain confused. A Choice accepts up to 255 options. [choice](https://docs.typesafe.ai/primitives/choice.md)
- **Noul estimates one yes/no proposition.** The single `noul` value is the probability of “yes”: values near 1 are strong yes, near 0 strong no, and near 0.5 ambiguous. Noul has no separate confidence field because one probability completely specifies the binary distribution. [noul](https://docs.typesafe.ai/primitives/noul.md)
- **Noul is probability of a proposition, not degree on a scale.** A middle value can mean weak evidence or ambiguity; it does not mean “medium severity.” Degree belongs in Score. Threshold direction should reflect asymmetric error cost, and an ambiguous band can route to review. [noul](https://docs.typesafe.ai/primitives/noul.md)
- **Noul questions should be atomic and positively oriented.** A conjunction such as “angry and requesting a refund” should become two Nouls combined in code. Phrase high values as yes to avoid inverted downstream logic, and add explicit true/false criteria only when the boundary needs clarification. [noul](https://docs.typesafe.ai/primitives/noul.md)
- **Score locates state on a caller-defined ordered rubric.** It accepts 2–10 ordered level descriptions. The output includes the probability-weighted numeric position, the level legend, probabilities, and confidence; a fractional score represents a distribution between levels, not a hidden continuous ground-truth quantity. [score](https://docs.typesafe.ai/primitives/score.md)
- **Read Score's distribution, not just its mean.** Different probability distributions can produce the same score, and confidence describes concentration rather than correctness. Low Score confidence can mean overlapping levels, a multi-dimensional question, or insufficient state. [score](https://docs.typesafe.ai/primitives/score.md)
- **Good Score levels describe situations.** The model sees descriptions but not level numbers or neighboring relationships, so “moderately severe,” number-only levels, and “worse than previous” are weak. Each Score should measure one dimension; add only levels that are distinct, then validate wording on known examples. [score](https://docs.typesafe.ai/primitives/score.md)
- **Confidence is derived from the returned probability distribution.** For Choice and Score, peaked distributions are high-confidence and flat distributions low-confidence. The docs' three-option interactive demo uses an approximation, while the production formula is not specified. Full probabilities are available if a different uncertainty statistic better fits the application. [confidence](https://docs.typesafe.ai/confidence.md)
- **Calibration is population-level, not a guarantee about one answer.** System One probabilities are trained to reflect uncertainty across groups of predictions. High confidence says the distribution is concentrated; it does not prove the selected answer is correct. [system one](https://docs.typesafe.ai/concepts/system-one.md); [score](https://docs.typesafe.ai/primitives/score.md)
- **Confidence should change system behavior, not decorate logs.** The documented pattern is high confidence → automatic action, medium → confirmation/review/more evidence, low → no action or escalation. The boundaries depend on the consequence of a wrong action and should be tuned using the application's own data. [confidence](https://docs.typesafe.ai/confidence.md)
- **Questions over one state should be batched.** Different primitive types can be mixed; questions are evaluated independently and in parallel, adding question tokens but little latency. The docs report a worked 13-question comparison as 11.5× cheaper and 9.6× faster than separate calls, though that is a cookbook-specific observation rather than a universal benchmark. [primitives](https://docs.typesafe.ai/primitives.md)
- **Dependent questions require another request.** Answers in one request do not become context for sibling questions. Only perform a second call when code genuinely needs the first answer to fetch evidence, build state, or choose the next answer space; otherwise fan out speculatively and ignore unused answers. [primitives](https://docs.typesafe.ai/primitives.md)
- **Composite judgments belong in code.** Split broad judgments into independent Nouls or Scores, normalize differently sized Score scales, and combine with explicit weights or a downstream classical model. This keeps the weighting inspectable and editable without prompt rewrites. [how to build with system one](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md); [score](https://docs.typesafe.ai/primitives/score.md)

## Important citation bookmarks

- Primitive selection guide: [primitives](https://docs.typesafe.ai/primitives.md)
- Choice request/response contract: [api](https://docs.typesafe.ai/api.md)
- Noul request/response contract: [api](https://docs.typesafe.ai/api.md)
- Score request/response contract: [api](https://docs.typesafe.ai/api.md)
- Confidence semantics and three-way routing: [confidence](https://docs.typesafe.ai/confidence.md)
- Risk-adjusted threshold example: [confidence](https://docs.typesafe.ai/confidence.md)
- Batch/parallel and dependency rules: [primitives](https://docs.typesafe.ai/primitives.md)

## Themes

- **Typed evidence rather than prose:** every answer can drive a branch, threshold, rank, or feature without parsing generated language.
- **Honest abstention:** uncertainty should create review or evidence-gathering paths.
- **One semantic dimension per question:** diagnosable signals are more useful than a single opaque “agent quality” score.
- **Code-side policy:** risk tolerance, weights, thresholds, and side effects remain explicit ordinary code.
- **Probabilities as data:** keep full distributions for calibration, monitoring, and downstream modeling rather than persisting only the winning label.

## Gotchas and version limitations

- Confidence is not correctness, and the local docs contain no per-domain accuracy or calibration error. A high-confidence wrong answer is still possible. [system one](https://docs.typesafe.ai/concepts/system-one.md); [score](https://docs.typesafe.ai/primitives/score.md)
- The same semantic question expressed as Noul versus Choice need not yield arithmetically comparable probability, and a question plus its negation need not sum to one. Do not transfer thresholds across primitive types or impose identities across separately evaluated questions. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- Score is weak for numeric precision. Do not interpolate its levels to reconstruct a real magnitude; use the expectation for ranking/thresholding and keep exact arithmetic in code. [jev 1.13](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md)
- Adding rubric examples can raise confidence without making an answer more correct. Validate examples on held-out inputs with known expected levels. [score](https://docs.typesafe.ai/primitives/score.md)
- The docs do not publish the exact confidence function, calibration dataset, expected calibration error, or stability of values across model versions.

## Task recipes

### Encode “does this run need intervention?”

1. Use independent Nouls for observable failure conditions such as “Is the agent repeating an unsuccessful action?”, “Does the latest tool result contradict the agent's claim?”, and “Is required user authority missing?”; do not join them into one compound question. Start at [noul](https://docs.typesafe.ai/primitives/noul.md).
2. Store each probability, then compose intervention policy in code. Use separate positive/negative thresholds and an uncertain band sent to a reviewer. Start at [noul](https://docs.typesafe.ai/primitives/noul.md).
3. Tune thresholds by false-intervention versus missed-intervention cost on labeled Gimble runs, not by copying documentation examples. Start at [confidence](https://docs.typesafe.ai/confidence.md).

### Select the coaching mode

1. Use Choice only after enumerating a closed, actionable set such as `clarify_goal`, `point_to_evidence`, `stop_repetition`, `request_authority`, `escalate_reasoner`, and `none`. Include `none` because a trace may need no coaching. Start at [choice](https://docs.typesafe.ai/primitives/choice.md).
2. Give each option contrastive `what`, `not_for`, and real trace examples. Start at [choice](https://docs.typesafe.ai/primitives/choice.md).
3. Use the full distribution: low confidence can trigger review, and a meaningful runner-up can cause a second check or multiple flags rather than blindly accepting the winner. Start at [choice](https://docs.typesafe.ai/primitives/choice.md) and [choice](https://docs.typesafe.ai/primitives/choice.md).

### Score intervention urgency without hiding dimensions

1. Ask separate Scores for consequence, time sensitivity, and evidence sufficiency; define concrete situations at every level. Start at [score](https://docs.typesafe.ai/primitives/score.md).
2. Normalize scales before weighting and keep the weights in code. Start at [score](https://docs.typesafe.ai/primitives/score.md).
3. Gate action on both score and confidence; a high severity estimate with low confidence should usually gather evidence or escalate instead of issuing a forceful automatic steer.

## Gaps left by this source segment

- No exact confidence formula or published calibration metrics.
- No guidance on online recalibration under label delay or class imbalance.
- No benchmark for agent-trace classification, coaching selection, or supervision outcomes.

