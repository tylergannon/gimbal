# Measuring economical research

The objective is useful, evidence-backed research and a queryable semantic
index with few repairs, at low total cost. A cheaper run is not a success if
it omits the difficult facts, hides disagreements, or cannot answer questions.

## Fixed measure before optimization

Start with a small controlled local-source suite. Each case has a research
goal, original documents, required facts with exact supporting passages, and
questions with expected evidence and answers. Include different time/version
scopes, different measurement units, cross-source answers, genuine source
disagreement, and an unanswerable question. These authored fictional sources
give us inspectable ground truth. They do not establish live-web research
quality or broad domain generalization.

Research and indexing roles see only the goal and source directory. A fixed
reader sees the question and generated index, with a bounded passage budget;
it does not receive expected answers. A fixed independent assessor compares
the produced document/index with the originals and gold facts. The optimizer
receives measured development results, not held-out answers. Each trial uses an independently named temporary working directory containing only that case’s sources. The bundled answer key is loaded into memory and removed from disk before trials; per-case assessor keys are written outside trial directories only after research ends and removed immediately after assessment. Completed trials are moved into the report directory before the next trial starts, outside the next worker’s temporary neighborhood; original paths remain recorded for interpreting native logs. Earlier outputs and the report are not in the trial directory tree. This is experimental blinding,
not a security sandbox against a malicious agent.

The research binding collects sources and builds topic indexes. The index binding curates the combined index, extracts auditable claims, and repairs it. Compare one role at a time from a common baseline to attribute changes. Per-role reports retain model usage, cost and summed turn seconds, including parallel turns; this is workload, not elapsed wall time or a direct measure of cognitive difficulty. Infer the value of stronger judgment from fewer errors/repairs and better independent query results. Fixed-source cases do not measure web discovery or download performance.

Keep these axes separate:

| Measure | Definition |
| --- | --- |
| Research coverage | Required facts correctly represented with their qualifications / required facts. |
| Research correctness | Supported factual assertions / factual assertions assessed, with unsupported and contradicted counts visible. |
| Index coverage | Required facts correctly represented or reachable from the index / required facts. |
| Index correctness | Supported index assertions / index assertions assessed; missing qualifications and unmarked source disagreement count as defects. |
| Query success | Correct answer or justified abstention, with valid supporting original-source passages, within the fixed returned-passage budget. |
| Evidence recall | Required evidence passages found / required passages; unknown-answer questions scored separately. |
| Whole-index size | Serialized claim/coherence packet bytes and o200k token proxy; a 24k proxy margin identifies possible single-request experiments, not proof of Jev token fit or contradiction recall. Exhaustive pair coverage remains active. |
| Repairs | Initial, after one repair, and final audit outcomes; count repair passes rather than hiding them in the final score. |
| Economy | Catalog-priced usage, elapsed time, and cost per successful trial. Candidate ranking uses priced researcher and curator usage plus Jev claim-audit input. Full-run recorded cost stays separate and unknown when a fixed role has no price. Role turns and summed time expose downstream work; automatic supervisor Jev and Jev output are not metered in this proxy. |

Gold comparisons are independent of the operational Jev audit. Otherwise the
optimizer could win by exploiting Jev's own mistakes. Exact quotes and source
paths are checked by code; semantic coverage and support remain judgments,
whose evidence and limitations are retained. Empty documents/indexes do not
receive perfect precision. Failed and incomplete trials stay in denominators.

The first quality threshold is conservative: all required facts covered, no
assessor-reported unsupported/contradicted factual assertions or hidden source
conflicts, and every query successful. It is a starting operating criterion,
not an empirically calibrated universal score. Report raw numerators and
denominators so thresholds can be revised deliberately rather than silently.

## The Gimbal workflow

`research-eval` reads a local candidate configuration and optionally an
alternative suite. Default candidates allow independent research/index assignments of Gemini Flash, OpenAI Luna and Terra, and Claude Haiku and Sonnet through their native providers for collection/topic indexing and
combined-index curation. Other research roles retain the production defaults so attribution is meaningful; `--fixed-model` explicitly selects a different pipeline and the report records that choice. The planner defaults to Opus 5.5, the reader to Gemini Flash, and the assessor to Claude Sonnet. Gemini Flash completed the bounded reader protocol in the live comparison; Haiku had stalled in an earlier trial. Luna and Terra remain candidate options when their provider is available. Resolved role bindings are recorded with each child run. Before optimization, the assessor evaluates one known-good and one version-confused index sample in separate sessions using the same assessment prompt. The label is withheld from its prompt and directory name. Raw false-positive and false-negative counts are reported; any error stops optimization. This small sanity check does not establish exhaustive judge accuracy or eliminate model-family bias.

A PromiseLoop chooses the next candidate and whether a new combination is
worth trying, from the supplied finite allowlist. Code validates the selection,
runs the real `research-document` workflow in a fresh directory with those
bindings and frozen local sources, evaluates the output, and sends actual
measurements back. It never lets a planner rewrite the suite, gold answers,
scoring code, or quality threshold. Bound the number of trials and per-trial
time, and retain errors as failed attempts rather than evidence of poor model
reasoning. A model or authentication failure is visible, not a silent fallback.

Fixed-source trials copy the originals once into the corpus’s shared `sources/` directory. Topic indexes link to those originals rather than multiplying identical source copies across all five researchers.

The bundled practical suite has one five-source Atlas development case and a separate five-source Cedar holdout. Every candidate dispatch runs all development cases. Selection requires one complete passing trial of every development case. The default three-round budget samples a cheap baseline and controlled role swaps from the 25 allowed combinations. The planner stops once a baseline and a controlled comparison produce an eligible choice; there are no mandatory repeats or exhaustive provider coverage. Reports expose trial counts, final quality-pass rate, first-audit cleanliness rate, mean repair passes, and quality-pass-with-at-most-one-repair rate, with failures in their denominators. One repair is a reported goal, not a hard eligibility cutoff. Require quality
success, then minimize cost among eligible candidates, using fewer repairs to
break a cost tie; show first-pass cleanliness and the one-repair tradeoff rather
than averaging quality away. The first bounded
experiment is exploratory and cannot establish a population success rate.

At the default limits, three rounds × one case × sixty minutes bounds development research at three hours, plus calibration, planning, assessments, retrieval checks, cleanup, and one holdout trial. Successful comparisons can stop earlier. Assessor, reader and answer-judge requests have separate deadlines so an unresponsive provider cannot stall the comparison indefinitely. A real Sonnet curation trial exceeded the former fifteen-minute research timeout. The result is a provisional choice for these fixed-source cases, not a reliability estimate or a broad model ranking.

After development selection, freeze the chosen candidate and evaluate the
held-out case without returning its scores to the optimizer. A holdout failure
means the promise is unfulfilled; it must not trigger tuning on that same
holdout. Ended dispatch or an exhausted budget is not a successful evaluation.

Each trial submits the compiled `research-document` workflow to the same hosted
project through a short CLI RunCommand, with separate output/work directories
and explicit model bindings. The parent observes the accepted run ID through
its existing registry, cancels it through the live controller on timeout, and
waits for terminal cleanup. The short submission waits for its accepted ID
even if the parent cancels meanwhile. An ambiguous submission failure remains
an operational error, not proof that no child started. Nested `gimbal.Run` is
explicitly unsupported by the current generator; killing a standalone research
process would bypass provider cleanup.

Queryability uses a code-owned navigation protocol: a fixed reader starts with
INDEX.md, requests at most two paths per turn, and gets at most six files and
18,000 bytes across four turns. It must cite exact original passages it read.
Observed out-of-protocol tool use or absent telemetry invalidates the query.
Each bundled corpus contains more source files than the reader can fetch. Required evidence is spread across sources, so the reader must use the index to choose within its budget. This constrains recursive grep and unrestricted corpus dumping; costs and failures stay visible. The semantic assessor checks meaning
in addition to the deterministic passage and answer-term checks.

## Research basis and limits

[ALCE](https://arxiv.org/abs/2305.14627) separates factual correctness from
citation quality. [ARES](https://arxiv.org/abs/2311.09476) separates retrieval
relevance, answer relevance, and faithfulness, and explicitly addresses judge
error with labeled evidence. [BEIR](https://arxiv.org/abs/2104.08663) motivates
testing retrieval across different tasks rather than trusting a single domain.
This workflow adopts those distinctions, not their benchmark scores or claimed
statistical guarantees. A small local suite must be supplemented by real,
independently labeled research tasks before choosing organization-wide defaults.

## Provider scope and price proxies

The active experiment is limited to Gemini, OpenAI and Claude. Tyler deferred GLM and DeepSeek after a router-backed GLM extraction turn produced no claim records before the trial deadline. That observation does not establish whether deferred routing, provider throughput or another cause was responsible. Custom candidate files can later revisit other providers; they are not part of the current default comparison.

The report uses catalog prices or harness-stated cost. These are usage-price comparisons, not subscription invoices. Unknown prices and absent usage remain unpriced; there is no special router-price fallback. Comparable cost excludes fixed roles because Gemini Pro author/review usage is unpriced in the current catalog; their turns and time remain visible, so this is a provisional economical choice rather than a full invoice comparison.

The operational extraction judge has a separate labeled live regression in `internal/claimaudit/extraction_live_test.go` (`GIMBAL_LIVE=1`, existing `TYPESAFE_API_KEY`). It distinguishes pure index metadata from subject facts, including facts embedded in routes, and tests missing qualifications, negation and partial coverage. This protects the extraction contract without treating a small calibration set as a recall guarantee.

Trial relocation assumes the parent remains alive through cancellation and cleanup. A killed host can leave old temporary work and stale run records; those artifacts are not successful or blinded completed trials.
