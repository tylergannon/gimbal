# Source map for the Jev claim-audit proposal

Consulted 2026-09-29. The proposal is in [proposal.md](proposal.md).
Official documents were fetched into this local source cache:

`/Users/tyler/.cache/gimbal-research/jev-claim-audit/2026-09-29/`

`catalog.json` there records the original URL, local absolute filename, and
SHA-256 of each downloaded document. These vendor documents are local research
inputs, not committed repository documentation. Fetch the `.md` form of each
official URL below if recreating the cache on another machine.

| Primary source | Local file | What it establishes |
| --- | --- | --- |
| [Models](https://docs.typesafe.ai/models) | `models.md` | Current version, the two distinct token limits, price, dynamic rate limits, text-only input. |
| [HTTP API](https://docs.typesafe.ai/api) | `api.md` | Request/response shape, question/answer IDs, usage, error handling. |
| [State](https://docs.typesafe.ai/concepts/state) | `concepts--state.md` | Shared state with independently evaluated questions. |
| [Choice](https://docs.typesafe.ai/primitives/choice) | `primitives--choice.md` | Closed output labels, distributions, confidence, question-ID visibility. |
| [Confidence](https://docs.typesafe.ai/confidence) | `confidence.md` | Confidence is derived from the distribution; policies need domain-specific thresholds. |
| [Jev 1.13 limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13) | `model-jaggedness--jev-1.13.md` | Numerical/date weaknesses, scope/literal-reading risks, context and generation limits. |
| [Citation-checking cookbook](https://docs.typesafe.ai/cookbooks/citation_check) | `cookbooks--citation_check.md` | Deterministic quote checks followed by source-relative semantic judgments; limited example evidence. |

## Current repository evidence

Line numbers below refer to baseline
`f276674347930bffc77022896f00a3bc4a762281`. They are orientation pointers,
not a claim that historical design documents describe the current runtime.

| File | Lines / symbol | Relevance |
| --- | --- | --- |
| [researchdocument.go](../../../internal/workflows/researchdocument/researchdocument.go) | 61, 126–132, 298–313 | Default three editorial rounds and `maxRounds` validation; initial curation then immediate authoring, where the bounded audit/repair stage belongs. |
| [researchdocument.go](../../../internal/workflows/researchdocument/researchdocument.go) | 342–381 | Gap collection, curation, and revision: repeat audit belongs here. |
| [researchdocument.go](../../../internal/workflows/researchdocument/researchdocument.go) | 405–438 | Existing source-count and nonempty-file checks do not verify claims. |
| [researchdocument.go](../../../internal/workflows/researchdocument/researchdocument.go) | 441–465 | Research, curation, author, and editor instructions currently rely on prose guidance. |
| [supervise_jev.go](../../../supervise_jev.go) | 18–24, 120–146 | Current Jev client use and unrelated supervision-specific threshold/context limits. |
| [go.mod](../../../go.mod) | 20 | Existing Go Jev SDK dependency at v0.2.0. |
| [expr.go](../../../internal/generate/expr.go) | 51–69, 202–209 | External calls versus recognized graph-visible RunCommand operation. |
| [rules.md](../../../internal/gimballint/rules.md) | GIMBAL101–109 | Static authoring constraints; ordinary explicit Go is permitted. |
| [command.go](../../../command.go) | 18, 98–110, 240–250 | Blocking command semantics, output bounding, immediate-process cancellation. |
| [graph.go](../../../workflow/graph.go) | 96–103 | Existing command graph representation. |
| [secrets.go](../../../cmd/gimbal/secrets.go) | 18–89 | CLI secret resolution and environment precedence. |
| [main.go](../../../cmd/gimbal/main.go) | 25–44 | Secrets loaded before ordinary CLI execution. |
| [gimbal-workflows skill](../../../skills/gimbal-workflows/SKILL.md) | Authoring and hosted/standalone sections | Workflows remain readable Go; hosted graph requires generation and matching binary. |
| [semantic-index skill](../../../.agents/skills/df-semantic-index/SKILL.md) | Required Index Contract | Existing index formats are flexible; no current universal atomic-claim schema. |

Also inspected the current `go doc -all .`,
[API design record](../api/API.md), [sprint record](../api/SPRINTS.md),
[definition of done](../../../docs/definition-of-done.md), and
[web application notes](../../../docs/web-app.md).
The definition-of-done document's timed-supervision description is stale
relative to current Godoc and code; it is not used as evidence for current
Jev behavior.

The installed dependency source is available at
`/Users/tyler/go/pkg/mod/github.com/kazz187/jev-sdk-go@v0.2.0/`.
Its `wire.go`, `client.go`, and `retry.go` expose structured responses, model
selection, and retry support. The proposal does not require another SDK.

## Algorithm and exploratory evidence

- [Mathematical analysis](mathematical-analysis.md): exact-oracle pair coverage,
  the hidden-single-edge lower bound, and globally retained witness marking in
  O(m² + N log K) requests. These are mathematical assumptions and proofs, not
  measurements of Jev fidelity.
- [Scope-owned-services corpus](../scope-owned-services/corpus/INDEX.md): the
  existing research index with source-linked claims about process lifetime and
  platform qualifications, suitable for selecting realistic evaluation cases. Topic-001's local
  originals include `sources/go-os-exec-exec.go.txt` and
  `sources/go-os-exec-exec_test.go.txt`. Some other routes lack local source
  directories; chosen references must be resolved to originals before labeling.
  Existing claims and clips are evaluation candidates, not ground truth.
- Exploratory Jev 1.13 calls tried source checks, individual pairs, group screens
  at 5/10/20/40 claims, and shared-state packed pair questions. Repetitive
  synthetic group examples produced expected top labels, but differently
  measured limits remained ambiguous or falsely conflicting, including with
  focused follow-ups and packed questions. They establish request feasibility
  and a known limitation, not corpus-level accuracy or a pruning threshold.
  No raw run output is part of this research note.
- A whole-corpus feasibility request sent all 21 non-source Markdown documents
  from that corpus, preserving their full text and relative paths. Jev accepted
  the packet at 15,736 input tokens. This demonstrates that this raw index and
  clip packet fits; it does not establish complete atomic extraction, the size
  of the eventual claim records, source truth, or contradiction recall. The
  proposal compares its cost with a clearly hypothetical 300-claim pair pass.

The proposal owns the starting five-claim packing choice, estimated token
margins, incremental extraction, bounded resume/repair, and evidence-backed
reviewed-resolution semantics. These are proposed operating decisions, not
vendor guarantees. The whole-index screen is an optional research direction;
there is no group-calibration implementation program in the baseline.

## Boundaries of this research

The official example establishes a useful construction pattern, not a
production accuracy guarantee. Live exploratory judgments are reported in the
chat, not committed as run output. No corpus-level evaluation or completed
research-document integration is claimed. The proposed scheduling, marking,
and completion policies are design decisions, distinct from vendor facts.
