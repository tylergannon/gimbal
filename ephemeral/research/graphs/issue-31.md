# Living instructions: guided intake, proof contract, and size-matched loops built into Tractor

URL: https://github.com/tylergannon/gimble/issues/31
State: closed
Milestone: None
Updated: 2026-09-01T23:35:46Z

## Summary

Tractor is a sharp tool that still requires the caller to show up already knowing how to use it. The `tractor` skill maps four "moments" to four examples; beyond that, authoring is schema-plus-lint archaeology, and the strategy of *whether* and *how* to use Tractor lives in a pile of skills spread across three repos (`tylergannon/agents`, `diffusioninc/skills`, and this repo's `skills/`).

This issue asks for Tractor to carry its own operating instructions: a built-in guidance flow that sizes the work, interviews the caller until the requirements and validation contract are up to snuff, picks or builds the matching workflow, and tells the caller what to watch for while it runs. It should do all of that without loading the strategy corpus into the parent agent's context. The tool should know how to use itself.

## The vector

Two dimensions.

**Size.** Three tiers, each with its own shape.

1. **SIMPLE.** A small item. No pipeline. One subagent, optionally with a supervisor node to keep it on goal.
2. **MEDIUM.** A clear articulation. One loop (implement → check), monitored.
3. **LARGE.** The project is divided into chapters and sprints. A separate loop runs per chapter; the chapter loop contains a sprints loop; each sprint is finished by a coding → validation loop.

**Definition.** Given the size, judge how much information has been supplied and ask questions until a sufficiently detailed set of requirements and a validation contract exist. A good spec goes straight to work. An absent or thin spec is interviewed into shape. The stopping rule is spec-authoring's: keep asking only while an answer would change the contract; stop after two rounds that surface only derivable detail.

The dimensions combine. The interview is proportional to the tier, and the contract it produces is what the workflow runs against.

## The contract, and the loop that consumes it

The validation contract is a proof-of-work claims list: observable behaviors the finished work must demonstrate, written before work starts. It is not a checklist of commands.

The ask: the contract lives in the workspace as a checklist, and the workflow loops over it. Each lap picks an unchecked claim, does the work, demonstrates it, and checks the box. The run cannot reach `success` until every claim has been demonstrated by a command. This is the loop-frames memo's type-1 loop (items live in a workspace checklist the loop node re-reads and selects from on every arrival; agents mark items done as ordinary work; the engine owns selection and injection, never storage) and the memo's resurrected goal gate at the exit. The LARGE tier is that loop nested: chapters → sprints → claims.

## Self-loading guidance

Instead of a million skills, the parent agent's context should hold only the trigger (the `tractor` skill description) and the tool surface. Sizing, interviewing, contract writing, shape selection, prompt doctrine, and run-watching should be loaded by Tractor at the moment each is needed and dropped afterward. Candidate mechanisms, none decided:

- **The authoring flow is itself a Tractor run.** An embedded workflow: an intake node judges size, a requirements node interviews (the parent agent is the HITL relay), a contract node writes the checklist, an emit node writes the pipeline and starts it. The parent only relays questions and reports the `run_id`; its context never sees the doctrine.
- **On-demand guidance tools or resources** over MCP, deferred-loaded the way `get_pipeline_schema` already is, one per stage (size, interview, contract, watch).
- **Embedded named workflows with parameters** (adoption brief, funnel stage 2), so MEDIUM and LARGE become `start_run(workflow, args)` with no authoring at all.

## Watching the run

After `start_run`, the caller needs to know what `get_run_status` fields mean as narration, what a healthy loop looks like, when to steer and when to leave it alone, what the run directory holds and where the proof is, what `COMPLETED` and `FAILED` mean for the claims, and how to resume. Today this is four sentences in the skill.

## What exists today

Inputs to consolidate, not the answer. The skills need heavy rework; treat them as source material for prompts the binary carries. Every item below is copied verbatim, with provenance, into `ephemeral/projects/tractor/living-instructions/sources/` on the `worktree-goal-gates` branch (see its `SOURCES.md`), so the author of this work reads from one directory instead of three repos.

- **This repo:** `skills/tractor` (moments → examples, prompt doctrine, symptom table, "make done honest"), `skills/orchestrate-attractor-loops` (manual manager loop: design → critique → implement → review → live validation), `examples/loops/`, 27 teaching lint rules, five MCP tools.
- **tylergannon/agents:** `proof-of-work` (claims before work, evidence in the PR), `spec-writing` / `spec-authoring` / `spec-review` (minimal spanning spec, grill HITL, stopping rule), `slice-design` (vertical slices, acceptance thresholds, handoff contract), `cell-lifecycle`, `adversarial-review` / `request-adversarial-review` / `consensus` (reviewer owns scope, re-review the whole target, never confirm-my-fix), `write-prompts` (loop anatomy: nine components, six halt conditions), `grilling` (one question at a time, recommend an answer, look facts up instead of asking).
- **diffusioninc/skills:** `df-easy-loop-e2e` (requirements interview with one multiple-choice question per visit and a "begin work" option; validation-holdout criteria the reviewer judges against but never names; plan as a `- [ ]` checklist; the reviewer is the only agent that checks boxes; the coder proposes checks with evidence), `df-easy-loop-simple` (same from an existing spec), `df-chapter-create` / `df-sprint-plan` / `df-sprint-execute` (chapters span 12–100 sprints; sprints are the executable unit; ledgers; Pyramid Index), `df-promise`.
- **Design records in this repo:** `ephemeral/projects/tractor/adoption-design/brief.md` and `wisdom.md` (the agent funnel; Pathfinder, Ladder, Consensus Loop; the principle → mechanism map), `ephemeral/projects/tractor/loop-frames/memo.md` (loop node, checklist items, ephemeral frames, goal gates; §9 as amended by later sections).

## Asks, as claims to demonstrate

1. A caller who says "keep working on this until it's done" with a one-line goal is asked one question at a time until a claims contract exists, and a run starts, without the caller reading anything beyond the trigger.
2. A caller who supplies a complete spec gets no interview beyond confirming the claims, and the run starts.
3. SIMPLE work is recognized and is not turned into a pipeline.
4. A LARGE goal runs as nested loops (chapter → sprint → claim), and its run directory shows which claims each sprint demonstrated.
5. Each lap of the claim loop checks exactly one box, and `success` is reached only through a command that demonstrates the goal.
6. After the run starts, the parent agent's context holds the trigger, the `run_id`, and the watching guidance, and none of the authoring doctrine.
7. The watching guidance lets the parent narrate progress and decide steer, stop, or resume without reading run files.

## Open questions

- HITL channel for a detached run's interview: `steer_run` into a blocking node, or the parent relaying a question file (the df-easy-loop pattern)?
- Does the LARGE tier need loop nodes from the loop-frames memo first, or can it ship on today's five node types plus a checklist convention in the milestone-loop style?
- Where does the contract live: a workspace checklist (the memo's position) or a pipeline field?
- Validation holdout: does the reviewer see the whole contract, or is a subset withheld from the coder and reviewer as df-easy-loop does?

## Out of scope

PR #29's `proof_contract` schema and its mode and terminal vocabulary (declined). A DSL. Engine changes not required by the three tiers. The terminal-enforcement piece is small and separate: a `goal_gate` lint that only a tool node's `on_success` may route to `success`, prototyped on the `worktree-goal-gates` branch.

