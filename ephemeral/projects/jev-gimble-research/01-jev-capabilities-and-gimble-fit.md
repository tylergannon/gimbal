# TypeSafe AI Jev 1.13: Technical Assessment and Gimble Integration Fit

**Date**: 2026-09-21
**Target Model**: `jev-1.13.0` (`jev-latest`)
**Context**: Gimble Go Workflow Library Assessment

This document assesses TypeSafe AI's Jev model for integration into Gimble's agent workflow ecosystem. It synthesizes official TypeSafe documentation, architectural boundaries, and known jagged edges to establish concrete integration strategies, operational costs, and experimental gateways.

---

## 1. Technical Specifications, Contracts, and Operational Envelope

### 1.1 API Contract and Primitive Mechanics
Jev operates exclusively through `POST https://api.typesafe.ai/v1/systemone` using three scalar classification primitives, fundamentally distinguishing it from generative LLMs:
- **`Noul`**: Binary probability estimation ($0.0$ to $1.0$).
- **`Choice`**: Multi-class categorical selection across a discrete set (up to 255 options), returning the selected key and discrete probabilities.
- **`Score`**: Ordinal rating (2 to 10 levels), returning an expected value ($\sum i \cdot P(i)$).

As documented in `pages/models.md`, Jev 1.13 is **strictly text-only**. It natively rejects images, screenshots, and audio. While structured JSON states are accepted, all leaf values must resolve to text or numbers. Multimodal visual testing (e.g., UI checks in Gimble) requires external transcription via models like Gemini Flash prior to Jev ingestion.

### 1.2 Context Ceilings and Go Integration
Jev enforces a strict 64,000 total request token limit, with the ingested `state` plus the single longest question constrained to a 32,000 token maximum. Jev's architecture supports "speculative fan-out" batching, evaluating up to 255 questions against a single shared state in parallel.

Integrating Jev into Gimble requires a zero-dependency `net/http` client. In accordance with Gimble's "no reflection" rule (`ephemeral/research/api/API.md`), the client must map Jev JSON structures strictly via static Go structs, avoiding `runtime.Caller` heuristics.

### 1.3 Operational Envelope: Pricing, Quotas, and Latency
- **Pricing**: $0.042 per 1,000,000 input tokens; **output tokens are free** ($0.00). Speculative batching amortizes state input costs across all evaluated questions.
- **Latency Profiles**: TypeSafe does not publish formal P50/P95/P99 latency bounds. However, empirical benchmarks show a mean round-trip of ~111 ms for small JSON states and ~270 ms for a 13-question batched document (vs. 1.5–14s for generative models).
- **Throughput Limits**: Base quotas are 250,000 tokens/sec and 1,200 req/min. Clients must handle dynamic downward throttling via `429 Too Many Requests` (token limits) and `529 Overloaded` (GPU exhaustion), parsing `Retry-After` headers for exponential backoff.
- **Privacy Governance**: TypeSafe contractually commits to zero training on customer data. However, Zero Data Retention (ZDR) is not enabled by default; Gimble deployments passing proprietary source code into Jev `state` must secure an Enterprise ZDR contract.

---

## 2. Probabilistic Calibration, Confidence, and Jagged Edges

### 2.1 Calibration and Epistemic Confidence
Jev calculates explicit `confidence` for `Choice` and `Score` primitives, mathematically derived from probability concentration (e.g., for Choice: $\max(0, \min(1, \frac{N \cdot p_{\max} - 1}{N - 1}))$). `Noul` questions do not report confidence; callers must infer certainty from distance to $0.5$.

TypeSafe claims robust calibration via RLCD (Reinforcement Learning for Calibrated Decisions) training. Case studies (e.g., SEC 10-K classification) demonstrate 90.0% precision within the high-confidence cohort ($\ge 0.90$) compared to 40.0% in unsure cohorts, alongside sub-$0.01$ standard deviation variance in self-consistency checks.

### 2.2 Structural Invariance Failures
Jev exhibits severe, documented failures in mathematical composition across questions (`pages/model-jaggedness/jev-1.13.md`):
- **Noul vs. Choice Divergence**: Re-framing a `Noul` proposition as a binary `Choice` yields radically conflicting probabilities.
- **Complement Non-Additivity**: $P(A) + P(\neg A) \neq 1.0$.
Consequently, Bayesian arithmetic on independent Jev probabilities within Gimble is strictly prohibited.

### 2.3 Cognitive Jagged Edges and Context Rot
Jev is highly susceptible to specific cognitive failures:
- **Literal Reading**: Interprets exact phrasing rather than conversational intent.
- **Arithmetic and Counting**: Fails at duration math, discrete tallying (item counting), chronological date ordering, and hexadecimal coordinate proximity.
- **Multi-Hop Indirection**: Accuracy degrades rapidly on relational graph chains.
- **Context Rot**: Large traces containing irrelevant logs act as semantic distractors. State payloads must be aggressively pre-filtered in Go.
- **Adversarial Rationales**: Without explicit security framing (e.g., a `contains_prompt_injection` Noul), Jev treats agent self-justifications ("tests passed") in raw logs as factual, exposing vulnerability to semantic jailbreaks.

---

## 3. Live Agent Supervision and Steering Integration

### 3.1 Supervisor Cadence and Steering Cascades
Gimble's `Session.Steer` API requires free-form text injections. Because Jev is non-generative, its integration requires a deterministic mapping in Go from discrete `Choice` keys or high-probability `Noul` flags to pre-defined textual templates.

In Gimble's `WithSupervisor` loop (default 3-minute cadence), Jev serves as a highly efficient sub-30-second pre-filter. By evaluating rapid semantic checks, Jev gates expensive generative LLM invocations, saving 90-95% in supervisor spend while preserving rapid anomaly detection.

### 3.2 Race Conditions and Loop Detection Boundaries
While Jev's 111ms latency easily beats agent turn duration, it operates without concurrency locks. Gimble handles race conditions by dropping steering injections (`landed = false`) if an agent advances before the Jev HTTP call completes.

Crucially, **Jev must not be used for mechanical loop detection**. Counting repetitive tool calls, tracking identical file hashes, or verifying deterministic exit codes remains the domain of Go sliding-window algorithms. Jev's sole responsibility is **semantic stuck-state detection**: identifying semantic drift, circular paraphrased reasoning, and premature declarations of task completion.

---

## 4. Offline Trace Classification and Workflow Routing

### 4.1 Trace Partitioning and Cost Economics
For continuous offline monitoring of Gimble `run.jsonl` traces, Jev offers dramatic cost advantages over generative models. Monitoring 10,000 agent turns costs ~$1.68/day with Jev, compared to ~$5.20 for Gemini Flash and ~$44.00 for Claude Haiku. 
To avoid context rot, Go code must slice hierarchical scope keys (`lap.3/bakeoff.1/attempt.2`) into focused 2k–4k token evaluation chunks before sending state to Jev.

### 4.2 Failure Mode Classification Taxonomies
Gimble relies on a strict division of labor for classifying agent failures. Deterministic Go code must handle permission/OS errors (e.g., exit code 126), syntax errors, and out-of-scope file modifications (`filepath.Rel`). Jev is strictly responsible for semantic drift and hallucinated dependencies. Jev classifies these failures using a multi-question battery of decomposed atomic `Noul` and `Choice` heads (e.g., asking separate `failure_class` and `out_of_scope_intent` questions) rather than one broad question, which reduces its inherent counting and literalism weaknesses.

### 4.3 Pre-Flight Sanitization and Post-Turn Validation
Jev is uniquely suited for guardrail architectural patterns:
1. **Pre-Flight Prompt Sanitization**: Incoming task prompts must be screened using a Jev `Noul` battery to detect prompt injections (adversarial jailbreaks) and credential leaks (raw secret API keys). If either triggers above a confidence threshold (e.g., $0.70$ for injection, $0.60$ for leaks), Gimble deterministic code aborts the run before `s.Generate` is dispatched.
2. **Post-Turn Output Validation**: Following the `citation_check.md` pattern, Gimble must first verify the syntax and physical existence of cited files using deterministic Go (`os.Stat`). Jev then runs a semantic fidelity battery (`Choice` & `Noul`) to evaluate whether the verified file content actually supports the agent's claimed fix.

### 4.4 Workflow Intent Routing and Bake-Off Ranking
Jev efficiently routes tasks between heavyweight (GPT-5.5, Claude Sonnet) and lightweight models (Haiku, Codex Luna) using parallel Intent and Complexity classification, demonstrably reducing misrouted tool execution from 16.8% to 7.3%.

Under Gimble's Definition of Done (`docs/definition-of-done.md`), Bake-Off evaluation proceeds in two phases:
1. **Deterministic Filter**: Tests pass, files exist.
2. **Jev Evaluation**: Jev validates semantic requirement fulfillment and scores simplicity across passing candidates. Code style never blocks a merge.

---

## 5. Negative Boundaries and Experimental Framework

### 5.1 Negative Boundaries and Generative Requirements
Jev is fundamentally inadequate and must **not** be used when:
1. Synthesizing causal explanations, code patches, or multi-step deductive proofs. Generative models (e.g., Gemini 2.0 Flash, Claude 3.5 Sonnet) are strictly required for these tasks.
2. Parsing code ASTs, regular expressions, or evaluating numerical metrics (dates, counts, sizes). Deterministic Go code must handle these natively.

### 5.2 Tri-Band Escalation Policy
Integration requires explicit confidence gating:
- **High ($\ge 0.85$ conf / $\ge 0.70$ prob)**: Execute automated steering/routing.
- **Medium ($0.50$-$0.85$ conf / $0.35$-$0.70$ prob)**: Caution; do not steer. Escalate to secondary verification (e.g., fast LLM) or wait for next cadence.
- **Low ($< 0.50$ conf / $< 0.35$ prob)**: Abstain explicitly. Yield to human operator or powerful reasoning model.

### 5.3 Concrete Experimental Design
Before Gimble integration, three high-value experiments must be executed comparing Jev 1.13 against Go heuristics, Gemini Flash, and Claude Haiku baselines:
1. **Semantic Thrashing Detection**: Evaluate 100 historical Gimble sessions (50 looping, 50 successful) to measure anomaly detection.
2. **Bake-Off Selection**: Score 30 historical bake-offs (90 candidate diffs) against Definition of Done requirements.
3. **Pre-Flight Sanitization**: Screen 100 task prompts for injection vulnerability and clarity.

### 5.4 Quantitative Acceptance Metrics
Production adoption of Jev within Gimble requires satisfying strict, measured thresholds:
1. **Precision $\ge 90.0\%$**: Minimizes disruptive false-positive steering interrupts.
2. **Recall $\ge 80.0\%$**: Catches 4 in 5 runaway semantic loops.
3. **Latency Overhead ($P_{95}$) $\le 250\text{ ms}$**.
4. **Calibration Error**: Brier Score $\le 0.12$ and ECE $\le 0.08$.
5. **Cost**: $\le \$0.25$ overhead per 1,000 agent turns.
