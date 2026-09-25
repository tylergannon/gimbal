# Historical teachings: programmatic workflows

## Scope and authority

This leaf extracts authoring rationale from the preserved programmatic-workflow
research and three current worklogs. The `ephemeral/legacy/` documents are
historical inspiration and rationale, not the current Gimbal API. Current
worklog decisions are authoritative where they correct the historical material.
Do not infer an exported identifier, runtime guarantee, or compatibility promise
from a quoted old sketch. The strongest recurring design test is whether a
workflow remains understandable as a short, ordinary Go program.

## Proposed subskill: Author workflows

Audience: a programmer writing or revising a Gimbal workflow.

Importance/decision: Make the workflow body read like one or two pages of
pseudocode. Evaluate indirection, composition, DRYness, names, and types by
whether a reader can see the stages, branches, loops, and evidence path at a
glance. This is an authoring ideal, not a demand for a graph-shaped DSL.
Authority: historical direction, reaffirmed by the current graph worklog.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CORRECTIONS-VERBATIM.md:L5-L7`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CLARIFICATION-VERBATIM.md:L25-L37`;
`ephemeral/worklog/202609152233-issue-227-graph.md:L16-L20`.

Keep program shape distinct from invocation data: inputs include available data
that may grow and the quality/shape of the index may evolve during execution.
Keep input validation, prompts, configuration, and adapters beside the
algorithm rather than hiding the algorithm in a framework. Ordinary Go owns
control flow; typed values and errors make phase boundaries visible.
Authority: historical teaching and recovered POC description, proposal for
current authoring.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/INPUT-SEPARATION-VERBATIM.md:L1-L2`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md:L21-L42`.

For independent work, freeze the shared proposal or baseline, run reviewers or
candidates independently, join before adjudication, and make ownership of
revision/selection explicit. Use ordinary goroutines and `errgroup` when
parallelism is needed; joining and cancellation belong to the enclosing Go
operation. A failed candidate is data for selection, while a reviewer execution
error may fail the round according to the task contract.
Authority: historical concurrency recommendation, not a current concurrency
guarantee.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:L10-L21`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:L23-L74`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:L76-L115`.

Do not guess control-flow meaning from unsupported syntax. The graph extractor
worklog says reassignment of session/group/loop identifiers is diagnostic, and
conditional children or nested calls in unsupported positions are diagnosed
rather than inferred. Rewrite the source so ownership is declared where the
node is created. The generated graph is visualization support, not a second
workflow runtime or a reason to contort readable Go.
Authority: current instruction/decision.
`ephemeral/worklog/202609152233-issue-227-graph.md:L28-L50`.

## Proposed subskill: Build and release Gimbal

Audience: maintainers designing the library, graph tooling, checks, and release
workflow.

Importance/decision: Optimize for the agent's path to success: clear objective,
fast discovery of local context, and evidence that explains why a run wandered,
failed, cost too much, or needed unwanted steering. A semantic index is a
first-class companion to difficult workflow work, not an afterthought.
Authority: historical direction and correction; methodology must be reconciled
with current repository gates before implementation.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/CONTEXT-REFRAMING-VERBATIM.md:L39-L63`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CORRECTIONS-VERBATIM.md:L9-L15`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/ACTIONS-AND-KNOWLEDGE-VERBATIM.md:L23-L25`.

Treat context as one evolving information space: runtime state and the searchable
on-disk index should be presented together, while knowledge remains divided
across time and roles so one agent is not asked to remember everything. A
checker follows a builder; supervisors observe for distinct failure classes.
This motivates context/research seams, but does not authorize speculative
telemetry, provenance, or a compass mechanism in the first implementation.
Authority: historical rationale; the explicit rejection of an ungrounded
compass is current historical correction.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/KEY-CLAIM-VERBATIM.md:L8-L16`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/KEY-CLAIM-VERBATIM.md:L14-L16`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CORRECTIONS-VERBATIM.md:L13-L15`.

Preserve proof boundaries. The recovered POC explicitly lacked graph-engine
parity, automatic context, concurrent call ownership, supervisors, native
session continuity, and a visualizer; its old commands and import paths are
not current Gimbal commands. Current graph work also removed `graph.json` and
made registration by name the sole graph surface because files become an API
that agents may build upon.
Authority: historical limitation plus current instruction.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md:L30-L42`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md:L93-L98`; `ephemeral/worklog/202609152233-issue-227-graph.md:L41-L50`.

## Proposed subskill: Use Gimbal

Audience: a programmer calling an existing workflow, inspecting runs, and
working within Promise, chapter, or sprint delivery conventions.

Importance/decision: Read the workflow's stated goal, inputs, acceptance, and
evidence contract before supplying runtime data. Existing workflow code is the
authority for its actual shape; historical names such as `SprintExecute`,
`ChapterLoop`, and `DeliveryLoop` are examples of visible sequencing, not an
API catalog. A delivery loop's meaningful sequence is plan, critique, update,
implement, review, and bounded repair.
Authority: historical source example, usable as a glossary pattern only.
`ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-WORKFLOWS.md:L19-L25`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-WORKFLOWS.md:L28-L46`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/POC-WORKFLOWS.md:L49-L97`.

Completion is behavioral evidence, not nil error, loop exhaustion, or a green
aggregate gate alone. Preserve explicit acceptance and distinguish a run that
ended from one that fulfilled its promise. Independent review is valuable
because it checks the builder's result from a different role and time position.
Authority: current worklog correction and historical context-engineering
rationale.
`ephemeral/worklog/interview-249-workflow.md:L3-L10`; `ephemeral/legacy/ephemeral/projects/gimbal/programmatic-workflows/CONTEXT-REFRAMING-VERBATIM.md:L59-L63`.

## Coverage, gotchas, conflicts, retrieval hints

Coverage: readable Go, shape/input separation, index-aware context, research
before coding, role/evidence independence, structured concurrency, graph
legibility, bounded repair, and historical POC limits.

Gotchas: old POC code imports `tractor`; old schema/run commands are historical;
the concurrency file explicitly says it is not today's guarantee; a graph is a
view of a workflow, not a replacement execution language; selection does not
silently merge or publish a candidate (`CONCURRENCY-SHAPES.md:L99-L109`).

Conflicts: historical graph persistence guidance is superseded by the current
worklog's no-`graph.json` decision (`202609152233-issue-227-graph.md:L7-L10`,
`:L46-L50`). Historical API sketches and invented role/catalog identifiers are
explicitly non-authoritative (`POC-RECOVERY.md:L76-L83`).

Retrieval hints: search `pseudocode`, `shape`, `input`, `semantic index`,
`context`, `errgroup`, `review`, `promise`, `acceptance`, `graph`, and
`graph.json`. Start with this leaf for rationale, then consult current API and
skill leaves for executable names and present contracts.
