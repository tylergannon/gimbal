# Jev for Gimble: an evidence-backed direction

**Date:** 2026-09-21  
**Decision:** Run one offline experiment. Do not add a global Jev integration or live auto-steering yet.

## The short answer

Yes: Jev could materially improve Gimble supervision, but in a narrower and more useful role than “another supervisor agent.”

The right mental model is a **semantic tripwire**. Jev can cheaply turn a short text snapshot of an agent run into typed signals such as “the completion claim conflicts with tool evidence,” “the work is drifting outside the stated scope,” or “this looks like a repeated strategy without progress.” Gimble code can use those signals to decide whether to do nothing, show an alert, select a pre-authored steer, or spend money on a generative supervisor that writes situation-specific coaching.

Jev is not itself the coach. It cannot generate novel steering text, explain a causal chain, edit code, or reason through a new recovery plan. It is also **text-only today**: no image, screenshot, audio, or video input. A vision model, OCR, or accessibility-tree extractor would have to turn visual state into text first. The official boundaries are indexed in [interface capabilities](index/sources/core-foundations/interface-capabilities.md) and [programmatic supervision](index/sources/core-foundations/programmatic-supervision.md).

That split is promising for continuous supervision at scale:

1. deterministic Go handles facts it can know exactly;
2. Jev handles repeated, bounded semantic judgments;
3. a generative model or person handles novel diagnosis and coaching;
4. Gimble remains the owner of policy, state, actions, and evidence.

## Why the idea is plausible—but not proven

The official API is unusually well shaped for this job. A request supplies one text or JSON state and many independent `Noul`, `Choice`, or `Score` questions. The questions share the state and are evaluated in parallel. The published model price is $0.042 per million input tokens with free output tokens; the current documented limits are 64k tokens across the request, 32k for state plus the longest question, 250k tokens/second, and 1,200 requests/minute. Those numbers make frequent bounded judgments economically plausible. They do not establish a latency SLA or supervision accuracy. See [core capabilities](index/sources/core-foundations/interface-capabilities.md) and [speculative fan-out](index/sources/patterns-demos/speculative-fan-out.md).

The six-day-old ecosystem supplies useful early evidence, not production proof. Public artifacts report:

- much lower false-alarm rates when semantic checks run at a completed turn boundary rather than on every edit hunk;
- cheap large-scale trace classification, but poor absolute macro-F1 on a difficult 17-class failure taxonomy and severe confidence miscalibration;
- successful low-latency routing and guardrail demos, alongside prompt sensitivity and protocol-specific failures;
- context-pruning experiments that sometimes save tokens but can slow the agent, invalidate provider prompt caches, or remove failed attempts the agent needs to avoid repeating.

Those stories are catalogued with their evidence strength in [the ecosystem field report](02-jev-ecosystem-field-report.md). Most results are author-reported, on small or project-specific datasets, and only days old. They justify an experiment, not a product commitment.

## The boundary that should not move

### Keep in deterministic Go

- exit status, test results, schema validation, file existence, hashes, elapsed time, repeated identical commands, changed-path boundaries, and exact counts;
- whether a turn is active and a steer can land;
- rate limiting, timeouts, retry budgets, spend limits, model-version recording, and fail-open behavior;
- policy: which signals may trigger which action.

Jev is documented to be jagged on counting, arithmetic, dates, multi-hop indirection, long distracting state, and logically equivalent question formulations. Its `Choice` output always chooses among the supplied options even when none is good. Raw probabilities from separately phrased questions must not be treated as a coherent Bayesian model. See [primitives and confidence](index/sources/core-foundations/primitives-confidence.md) and the [technical report](01-jev-capabilities-and-gimble-fit.md).

### Give Jev bounded semantic work

- does the agent’s completion claim conflict with the command output?
- is the current work semantically outside the task despite touching an allowed path?
- is the agent repeating the same approach in different words without new evidence?
- is a tool result likely relevant to the next decision?
- which member of a small, closed intervention taxonomy best fits this snapshot?

Ask atomic questions. Keep the action set closed. Validate the question form on Gimble data because `Noul`, `Choice`, and `Score` can behave differently even when they appear equivalent.

### Keep novel coaching generative

If the needed message depends on a new causal diagnosis, code-specific recovery plan, or synthesis of several failures, send the selected evidence to a generative supervisor. Jev can decide that review is warranted or choose a known coaching mode; it cannot write the message. A few proven, context-independent interventions could eventually use literal templates visible at the workflow call site, but that should be an outcome of the experiment rather than a premise.

## Turning coaching examples into a real evaluation dataset

The useful training unit is a **decision point**, not an arbitrary transcript chunk. Start with a snapshot immediately after a turn or another stable workflow boundary. It should contain only information available at that moment:

- the run goal and visible scope values;
- a bounded recent event/transcript tail;
- deterministic facts such as commands, exit codes, tests, changed files, elapsed time, and whether progress occurred;
- the candidate semantic signals and the action that a supervisor could have taken then.

Do not include later steers, final run status, validator outcomes, or the agent’s subsequent recovery in the predictor input. Those are labels or outcomes; including them would leak the answer.

Label two things separately:

1. **Intervention need:** no review, review now, or insufficient information.
2. **Reason/action family:** evidence contradiction, scope drift, semantic repetition, unsupported completion, missing user input, or another taxonomy grounded in observed runs.

The negative set matters at least as much as the positive set. It needs productive exploration, expected red-green TDD cycles, temporary compile failures during a refactor, deliberate retries with new evidence, long but healthy research, and correct completion. Otherwise the detector will learn that normal agent work looks broken.

Split data by run, workflow, and time—not by adjacent decision points—so near-duplicate windows from one session cannot appear in both training and evaluation. Keep a final chronological test slice untouched until questions, features, and thresholds are locked.

Jev itself is not fine-tuned. Two approaches are worth comparing:

- a direct policy over a small atomic question battery;
- a downstream logistic model or small gradient-boosted tree using Jev probabilities plus deterministic telemetry.

The second pattern is demonstrated in TypeSafe’s feature-discovery cookbook, but it must earn its complexity on Gimble data. If the corpus is small or has few positive interventions, do not fit a calibrator or claim generalization; use Jev only to rank cases for human review while collecting more examples. See [autoresearch feature discovery](index/sources/cookbooks-a/autoresearch_feature_discovery.md).

## The one experiment to run first

Run an **offline semantic-thrashing and unsupported-completion replay** over finished Gimble runs. Do not mutate a run, call `Steer`, or add product code.

Compare four lanes on the same held-out decision points:

1. deterministic Go signals only;
2. the current cheap generative supervisor baseline;
3. a small atomic Jev battery;
4. Jev signals plus deterministic telemetry in a simple downstream model, only if the dataset supports it.

Measure:

- precision-recall curves and precision at a fixed human-review budget;
- false interventions per healthy run and recall on independently labeled intervention points;
- lead time before an eventual failure or expensive supervisor intervention;
- Brier score/reliability plots for any probability used as a threshold;
- latency, token use, and actual cost per decision point;
- disagreement cases, especially confidently wrong Jev answers.

Use a development split to choose the question forms and operating point, then freeze them before touching the chronological test split. The go/no-go rule should be comparative, not an invented universal threshold: proceed only if Jev finds materially more semantic failures than deterministic signals while matching the generative baseline’s intervention precision at a meaningfully lower measured cost or latency. Report confidence intervals and all failure cases. If no Pareto-improving operating point exists, stop.

## Staged decision

1. **Offline replay:** establish whether the signal exists.
2. **Live shadow:** if replay succeeds, observe new turns without steering. The storage shape for shadow judgments must use a real Gimble contract designed at that point; this research does not assume arbitrary `ValueSet` events can be emitted from outside a scope.
3. **Review gate:** let Jev summon a generative supervisor or human, still without automated steering.
4. **Opt-in template steering:** only for an intervention family that has independently demonstrated high precision and a harmless false-positive cost.

Every network failure, timeout, 429, or 529 should mean “no Jev opinion,” not a blocked workflow. Pin the Jev model version during evaluation, record the concrete returned model, and revalidate before moving an alias. The public policy says customer data is not used for training, but standard documentation does not promise zero retention; sensitive trace deployment needs legal review and likely an enterprise ZDR agreement.

## Recommendation

Proceed with the offline replay. The evidence is strong enough that Jev may remove most routine semantic scanning from a generative supervisor, which is exactly the leverage in the original idea. The evidence is not strong enough to put Jev on every event, prune agent context, or let it steer autonomously.

The first win to look for is modest and concrete: **Jev notices a bounded class of semantic failures early and cheaply, so the expensive supervisor reads only the cases that deserve coaching.** If that survives held-out Gimble traces and then live shadow observation, the path to continuous supervision becomes credible. If it does not, the experiment still leaves behind a useful labeled failure corpus and honest baselines.

## Open questions

- Are there enough genuinely independent positive coaching examples in saved Gimble runs to support calibration rather than anecdote?
- Which stable boundary—completed turn, failed check, scope transition, or planner decision—provides the best signal-to-noise ratio?
- Can untrusted transcript and tool text be safely represented without turning Jev’s state into an injection channel?
- Does a direct atomic battery generalize better than a downstream learned model once workflows change?
- What retention, residency, security, and ZDR terms are acceptable for proprietary code and transcripts?
