# Delivery loops: research synthesis

## Scope and reading

- Sources: the two current `df-easy-loop` skills, current `df-sprint-execute`, the legacy workflow-program direction, and the two Sprint 001 planning worklogs.
- This leaf extracts reusable delivery principles for Gimble workflow authors and maintainers.
- The diffusion skills describe an external operating method. They do not, by themselves, define Gimble builtins or a required public API.
- The legacy document is a historical design record; it explicitly says the described indexing/context behavior was not implemented at the time. [ephemeral/legacy/docs/workflows-as-programs.md:L3-L9]

## Proposed topics

### 1. Choose the entry contract: interview or existing spec

- Audience: Author.
- Authority: current skill for the distinction; historical design for the underlying context principle.
- Importance: decision. The workflow must know whether uncertainty is part of the job or whether a user-supplied specification is authoritative.
- Decision: use a requirements interview when material product decisions are unresolved; use the simple loop when an existing spec already describes what is being built.
- The end-to-end skill starts with a requirements agent, while the simple skill explicitly has no requirements interview and takes a spec document as input. [.agents/skills/df-easy-loop-e2e/SKILL.md:L169-L170] [.agents/skills/df-easy-loop-simple/SKILL.md:L31-L32]
- The end-to-end requirements stage records questions and answers verbatim, and derives validation criteria from them. [.agents/skills/df-easy-loop-e2e/SKILL.md:L268-L284]
- Coverage: good for method selection and preserving user decisions.
- Gap: neither skill gives a Gimble-level predicate for “materially unresolved”; authors still need judgment.
- Conflict: treating the e2e sequence as mandatory for every workflow would contradict the simple path and add ceremony where a settled spec exists.

### 2. Keep objective, responsibility, and success criteria together

- Audience: Author.
- Authority: historical design, with current skills as operational examples.
- Importance: decision. Each participant should receive enough context to act and to recognize completion without reconstructing the project from memory.
- The design says the first message should state the outcome, the agent’s responsibility, success recognition, constraints, current state, and routes to needed information. [ephemeral/legacy/docs/workflows-as-programs.md:L124-L142]
- The design also says the workflow carries responsibilities that no single agent should have to keep in its head. [ephemeral/legacy/docs/workflows-as-programs.md:L16-L24]
- Coverage: strong conceptual guidance for prompt/context assembly.
- Gap: the design leaves context assembly and indexed retrieval proposed rather than implemented. [ephemeral/legacy/docs/workflows-as-programs.md:L112-L122] [ephemeral/legacy/docs/workflows-as-programs.md:L199-L204]
- Conflict: do not infer a universal task schema or a Gimble wrapper from this design hypothesis; the document explicitly rejects an enormous universal document. [ephemeral/legacy/docs/workflows-as-programs.md:L119-L122]

### 3. Make plan, critique, and adaptation a closed handoff

- Audience: Author / Build-release.
- Authority: current skill.
- Importance: decision. Planning gains value when critique changes the authoritative plan before coding begins.
- Both easy-loop variants route plan -> plan-critique -> plan-update; the update reads the requirements or spec plus both earlier artifacts. [.agents/skills/df-easy-loop-e2e/SKILL.md:L286-L340] [.agents/skills/df-easy-loop-simple/SKILL.md:L248-L304]
- Updates are constrained to genuine derisking and maintainability/test improvements, which guards against critique becoming speculative scope expansion. [.agents/skills/df-easy-loop-e2e/SKILL.md:L333-L340]
- Coverage: clear artifact dependencies and a useful adaptation rule.
- Gap: the skills do not define how to adjudicate contradictory critique findings; that remains a workflow-specific decision.
- Historical Sprint 001 notes show why critique should inspect the intent and current code rather than blindly accept drafts: they identify accounting eligibility and missing persistence destinations as concrete defects. [ephemeral/worklog/20260912-sprint001-codex-critique.md:L1-L5]

### 4. Separate roles and preserve independent judgment

- Audience: Author / Build-release.
- Authority: historical design plus current skill.
- Importance: decision. Builders, validators, supervisors, and reviewers should have distinct responsibilities so local implementation success cannot silently substitute for acceptance.
- The historical design assigns building, supervising, and checking to different participants, while warning that handoffs must retain enough of the larger objective. [ephemeral/legacy/docs/workflows-as-programs.md:L157-L187]
- The easy loops use separate coding, validation, and review stages; the reviewer sees all prior coding updates and may route back to coding. [.agents/skills/df-easy-loop-simple/SKILL.md:L306-L365]
- The e2e variant makes validation holdouts authoritative: any unmet holdout fails validation even if other checks pass. [.agents/skills/df-easy-loop-e2e/SKILL.md:L371-L394]
- Coverage: strong separation of concerns and explicit return path.
- Gap: “independent” is a responsibility boundary, not necessarily a different model or process; the skills do not define a statistical independence guarantee.
- Conflict: exact model assignments and a fixed number of roles are staging choices, not Gimble semantics.

### 5. Treat artifacts as durable handoffs; use session reuse selectively

- Audience: Author / Build-release.
- Authority: current skill, supported by historical design.
- Importance: decision. Handoffs should name concrete files and preserve evidence; session continuation can retain working context when the same role returns.
- The orchestration loop requires expected artifacts and an outcome file before routing, and reads only the routing file to select the next step. [.agents/skills/df-easy-loop-e2e/SKILL.md:L191-L228]
- A continuing role resumes its prior session/thread; an empty-context role starts fresh, and missing identity is terminal for a continuing role. [.agents/skills/df-easy-loop-e2e/SKILL.md:L199-L209]
- The design treats ordinary files and indexes as first-class context that lets useful findings survive scope changes. [ephemeral/legacy/docs/workflows-as-programs.md:L106-L117] [ephemeral/legacy/docs/workflows-as-programs.md:L144-L155]
- Coverage: concrete rules for artifact existence, route authority, and continuation.
- Gap: no rule says which artifacts are authoritative when multiple visits produce similarly named outputs; an author must define that lineage.
- Conflict: session reuse is delivery orchestration behavior, not evidence that a Gimble run should expose or depend on a vendor session identifier.

### 6. Bound loops and make stop conditions explicit

- Audience: Author / Build-release.
- Authority: current skill.
- Importance: decision. A delivery loop needs a finite or externally governed stop condition and must not quietly relaunch failed work.
- Easy-loop orchestration caps visits per step, treats missing artifacts/nonzero exits as terminal errors, and routes to `end` after a cap. [.agents/skills/df-easy-loop-e2e/SKILL.md:L191-L228]
- The coding/review loop returns to coding only when requirements are incomplete or validation criteria fail; the review stage alone checks plan boxes. [.agents/skills/df-easy-loop-e2e/SKILL.md:L396-L430]
- Sprint execution records unresolved blockers in a sprint-specific file, then continues with unblocked tasks. [.agents/skills/df-sprint-execute/SKILL.md:L157-L162]
- Coverage: good bounds and deferred-blocker handling.
- Gap: “continue with unblocked tasks” needs a project-specific definition of dependency and may be unsafe when a later task assumes the blocked one.
- Conflict: a generic retry helper or unbounded supervisor would hide the bounded delivery contract and violate the explicit cap semantics.

### 7. Execute planned work in order, but preserve scope and links

- Audience: Build-release.
- Authority: current skill.
- Importance: decision. Sprint execution should consume the accepted sprint plan as the executable unit and retain chapter/non-goal context.
- The sprint executor loads the sprint, checks the ledger, preserves chapter links, and carries vector, acceptance criteria, and non-goals into the execution prompt. [.agents/skills/df-sprint-execute/SKILL.md:L92-L145]
- It asks the implementer to execute phases in order, verify each task, and record blockers rather than silently skipping them. [.agents/skills/df-sprint-execute/SKILL.md:L149-L162]
- If an index is available, the executor resolves concrete prior-art routes before launch and again before each implementation phase. [.agents/skills/df-sprint-execute/SKILL.md:L113-L132] [.agents/skills/df-sprint-execute/SKILL.md:L164-L178]
- Coverage: strong alignment between plan, ledger, chapter scope, prior art, and execution.
- Gap: the skill does not decide whether a changed proof shape requires plan amendment before implementation; that judgment belongs to the project contract.
- Conflict: the sprint skill’s ledger updates and CLI preflight are process mechanics, not automatically runtime nodes in a Gimble workflow.

### 8. Proof must exercise behavior and report evidence

- Audience: Build-release / Use.
- Authority: current skill, reinforced by historical worklogs.
- Importance: decision. Passing inspection or unit tests alone is insufficient when the acceptance claim concerns running software.
- The e2e validation role must run the software, record commands and relevant output, judge behavior against requirements, and check every holdout criterion. [.agents/skills/df-easy-loop-e2e/SKILL.md:L385-L394]
- The simple-loop reviewer likewise must run the software and the repository’s standard test set, not only inspect code. [.agents/skills/df-easy-loop-simple/SKILL.md:L351-L365]
- Sprint review checks blockers, runs the test suite, verifies chapter links, and reports completed tasks, blockers, tests, and link status. [.agents/skills/df-sprint-execute/SKILL.md:L192-L216]
- Sprint 001 draft notes demonstrate the standard: a saved fixture could not prove cache inclusion without emitter/provider evidence and a nonzero-cache live observation. [ephemeral/worklog/20260912-sprint001-codex-draft.md:L1-L5]
- Coverage: clear distinction between implementation, runtime validation, and reporting.
- Gap: the skills do not prescribe the shape of a Gimble proof artifact or live browser/runtime probe.
- Conflict: do not turn these reporting files into new Gimble proof programs; they describe delivery practice, while Gimble’s own proof contract must remain local to the workflow/package.

## What should not become a Gimble builtin

- Fixed Claude/Codex/Gemini assignments, exact model counts, foreground CLI commands, visit directory names, and `outcome.yaml` formats are staging rituals of the diffusion skills. [.agents/skills/df-easy-loop-e2e/SKILL.md:L41-L65] [.agents/skills/df-sprint-execute/SKILL.md:L180-L188]
- Requirements interviews, plan critiques, coding loops, and sprint execution are useful workflow shapes, but they should be written as ordinary Go orchestration where needed rather than exposed as opaque `EasyLoop` or `SprintExecute` wrappers.
- The historical design’s governing test is legibility: workflow source should read like roughly a page of pseudocode, with meaningful stages, loops, checks, and participant relationships visible. [ephemeral/legacy/docs/workflows-as-programs.md:L64-L83]
- The durable Gimble topics are therefore responsibility-aware context, explicit artifact handoffs, bounded routing, independent behavioral validation, and honest deferred findings; the exact diffusion staging remains an external method.

## Top three topics

1. Choose interview versus existing spec, because it prevents unnecessary ceremony while protecting unresolved product decisions.
2. Separate roles with independent behavioral validation, because a green implementation path is not acceptance evidence.
3. Use durable artifact handoffs with explicit bounds and deferred blockers, because delivery must remain resumable, inspectable, and finite.
