# Semantic-index leaf: Gimble graph rules

Route: static graph extraction -> scope/context provenance

- API contract and lint inventory: `rules-source-excerpts.md:207-221` (API extract,
  source: `ephemeral/research/api/API.md:275-293,856-870`).
- Scope identity, ancestry, Set/SetJSON, shadowing, and end behavior:
  `rules-source-excerpts.md:25-86` (source: `scope.go:29-74,127-201`).
- Session owner, post-end rejection, and Fork parent/current-scope behavior:
  `rules-source-excerpts.md:103-148` (source: `session.go:14-47,410-420,487-512`).
- Group child scopes and Wait join: `rules-source-excerpts.md:150-176` (source:
  `group.go:19-98`).
- Loop task scope and reserved `task` value: `rules-source-excerpts.md:178-205`
  (source: `loop.go:66-165`).
- Five-check issue audit and required corrections:
  `rules-issue-162-audit.md:7-146` (all sections; local issue source:
  `ephemeral/research/graphs/issue-162.json`).
- Compact business rule report and counterexample:
  `rules-report.md:45-85` (sections “Runtime-enforced versus static-only” and
  “Issue #162 corrections”; counterexample source:
  `ephemeral/attest/loop-practice/group-in-task/main.go:163`).
- Official analysis framework feasibility: `rules-go-analysis.md:3-44` (source URL
  and fetched excerpts), `rules-go-inspect.md`, and
  `rules-go-analysistest.md`.

Route: static graph extraction -> output model

- Runtime graph identity is scope-key prefix containment with session and turn
  descendants: `rules-source-excerpts.md:207-221` (“Static graph contract”; source:
  `ephemeral/research/api/API.md:720-799`).
- Data edges are explicit only where workflow calls ScopeText/Get/GetJSON;
  Generate performs no implicit context injection: `rules-report.md:8-15`;
  sources `scope.go:184-201` and `API.md:216-219`.

Route: static linting -> analyzer proof

- Use AST/type resolution plus `inspect` and `buildssa`; use package facts for
  inter-package summaries: `rules-go-analysis.md:38-44` and
  `rules-report.md:87-108` (“Feasibility and boundaries”).
- Test each diagnostic with positive/negative `analysistest` fixtures including
  branches, context.With* aliases, raw goroutine captures, Group.Go, and
  range-over-function task bodies: `rules-go-analysistest.md:20-24`.
