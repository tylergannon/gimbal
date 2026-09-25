# Chapters, sprints, ledgers, and retrieval

Audience: Gimbal contributors, sprint planners, reviewers, and agents deciding
whether a planning artifact is context, executable work, or proof.

Authority/currentness: This leaf is a research index over the exclusive
`09-chapters-sprints` manifest segment. The chapter and sprint skills describe
workflow policy; the checked-in Sprint 001 document and ledgers show the current
repository instance. The segment was read on 2026-09-18; citations point to the
source lines and should be reopened if those sources change.

## Vocabulary and boundaries

- **Chapter** means an optional durable vector for a series of future sprints,
  shaping design direction, architecture, product themes, and acceptance
  posture across roughly 12–100 sprints. It is not an executable sprint, OKR
  system, or fan-out planning workflow. `.agents/skills/df-chapter-create/SKILL.md:L7-L17`
- A chapter is warranted for a durable direction affecting many future sprints;
  one-sprint work, a short sequence, or implementation detail belongs in a
  sprint instead. `.agents/skills/df-chapter-create/SKILL.md:L82-L92`
- **Sprint** is the executable planning unit. Chapters may guide alignment, but
  sprints remain executable and their sprint docs/ledgers are the source of
  truth for executable work and status. `.agents/skills/df-chapter-create/SKILL.md:L46-L51`
- Sprint planning produces a final `docs/sprints/SPRINT-NNN.md` after intent,
  independent drafts, critiques, interview, and merge. `.agents/skills/df-sprint-plan/SKILL.md:L112-L119`
- **Task** appears in this segment as a sprint's implementation/proof unit or a
  runtime task scope shown in the Sprint 001 tree. The planning skill does not
  define a separate durable task ledger or task lifecycle. `.agents/skills/df-sprint-plan/SKILL.md:L321-L358`
- **Program** is not defined as a planning artifact here. Sprint 001 says a
  program can read the checkpoint and issue a Go scope query; that is a consumer
  of run observation, not a program planning model. `docs/sprints/SPRINT-001.md:L68-L84`
- **Run** is the observed execution in Sprint 001: a live/finished page, a run
  log, a checkpoint, scopes, turns, and messages. The segment gives no general
  run registry or planning authority. `docs/sprints/SPRINT-001.md:L7-L38`
- **Promise** is not defined by the chapter/sprint sources. Do not infer a
  Promise Loop, warranty, task, or run relationship from these files; retrieve
  the separate promise source when that distinction is required.

## Chapter artifact and authority

- Chapters should remain under 3000 tokens, have a short Pyramid Index, and
  express principles/boundaries rather than a fine-grained sprint backlog.
  `.agents/skills/df-chapter-create/SKILL.md:L46-L61`
- The chapter template names Vector, Design Direction, Architecture Principles,
  Sprint Horizon, Review Posture, Non-Goals, and Sprint Planning Notes. These
  are durable guidance surfaces, not execution status. `.agents/skills/df-chapter-create/SKILL.md:L104-L160`
- The chapter ledger stores `id`, title, status, document path, timestamps,
  summary, and optional linked `sprint_ids`; related sprint ids are included
  only when the repo already treats them as part of the chapter or the user asks.
  `.agents/skills/df-chapter-create/SKILL.md:L162-L189`
- The helper reserves `CHAPTER-XXXX` ids from the chapter ledger and resolves
  the project root from the current working directory or nearest `.git`/`docs`
  marker. `.agents/skills/df-chapter-create/scripts/chapter.py:L12-L37`
- `next-id` chooses one greater than the largest existing `CHAPTER-####`; `add`
  rejects malformed or duplicate ids and writes timestamps/status/doc/summary.
  `.agents/skills/df-chapter-create/scripts/chapter.py:L158-L200`

## Sprint planning and authority

- Chapter handling inside sprint planning is deliberately lightweight: read
  chapter context only when relevant, then run normal sprint planning; do not
  run chapter fan-out, critique, or competing chapter drafts.
  `.agents/skills/df-sprint-plan/SKILL.md:L9-L17`
- Orientation checks the sprint ledger, recent sprint docs, semantic-index
  configuration, relevant chapter ledger/docs, and recent trajectory. A chapter
  link must be selected because it fits the seed, not merely because chapters
  exist. `.agents/skills/df-sprint-plan/SKILL.md:L123-L169`
- The intent doc records seed, orientation, Pyramid Index, semantic-index state,
  optional chapter context, recent sprint context, code areas, constraints, and
  success criteria. `.agents/skills/df-sprint-plan/SKILL.md:L308-L358`
- If chapter context is selected, it must preserve chapter direction without
  making the chapter authoritative over sprint status. `.agents/skills/df-sprint-plan/SKILL.md:L349-L354`
- The final sprint structure includes Overview, Use Cases, Architecture,
  Implementation Plan, Files Summary, Definition of Done, Risks & Mitigations,
  Dependencies, and Open Questions; omit `Chapter:` when no chapter applies.
  `.agents/skills/df-sprint-plan/SKILL.md:L496-L528`
- After merge, `ledger.py sync` updates the sprint ledger; a YAML sprint may
  carry `chapter: CHAPTER-XXXX`, and a chapter ledger may list the sprint id,
  while `docs/sprints/ledger.yaml` remains authoritative for individual sprint
  status. `.agents/skills/df-sprint-plan/SKILL.md:L530-L540`

## Ledger mechanics and current repository state

- The sprint helper supports YAML and legacy TSV; YAML is preferred because it
  preserves optional chapter links. Its YAML statuses are `planned`,
  `in-progress`, `done`, and `abandoned`. `.agents/skills/df-sprint-plan/scripts/ledger.py:L2-L25`
- YAML ids are `SPRINT-####`; chapter ids are `CHAPTER-####`. The helper
  canonicalizes status aliases and rejects statuses outside the selected format.
  `.agents/skills/df-sprint-plan/scripts/ledger.py:L206-L225`
- `add` allocates the next numeric id, timestamps the entry, and accepts a
  chapter only in YAML after validating its chapter id. `set_status` updates
  status and timestamp; `set_chapter` can set or clear a YAML chapter link.
  `.agents/skills/df-sprint-plan/scripts/ledger.py:L238-L281`
- `current` means the first in-progress entry; `next_planned` means the lowest
  numbered planned entry. `sync_from_docs` scans `docs/sprints/SPRINT-*.md`,
  extracts title/chapter from the first 40 lines, and adds missing entries.
  `.agents/skills/df-sprint-plan/scripts/ledger.py:L283-L307`
- The checked-in sprint ledger currently has one planned record:
  `SPRINT-0001`, “Token usage by scope,” with created/updated timestamps.
  `docs/sprints/ledger.yaml:L1-L6`
- The current sprint is explicitly rewritten after earlier PRs; its Pyramid
  Index is a compact, reversible summary of placement, snapshot, page, shared
  definition, and proof. `docs/sprints/SPRINT-001.md:L1-L38`

## Historical proof and release examples

- Sprint 001 defines proof as running the real thing and reporting what was seen
  in the PR description. It forbids proof programs, reconcile scripts, run
  output under `ephemeral/`, and committed run artifacts. `docs/sprints/SPRINT-001.md:L299-L315`
- Its live proof must exercise parent-created/child-used sessions, concurrent
  attempts, and a two-level scope; it must be viewed live, after completion,
  and from an `observation.json`-only restart. `docs/sprints/SPRINT-001.md:L301-L315`
- Definition of Done binds claims to observed placement, totals, drill-down,
  timeline, checkpoint-only rendering, shared Go/TS numbers, and passing build
  checks; several criteria explicitly require seeing the live page.
  `docs/sprints/SPRINT-001.md:L343-L364`

## Semantic index and Pyramid routes

- Sprint planning treats `docs/SEMANTIC-INDEX.md` as mandatory configuration
  inspection. If an index is available, read its entrypoint, follow a relevant
  route, open leaf citations, run targeted `rg`, and summarize the result in
  orientation and intent. `.agents/skills/df-sprint-plan/SKILL.md:L138-L155`
- The repo's pointer says the token cache is local `docs/` and `ephemeral/`;
  the semantic index is a routing tree over it. It is currently not built, so
  direct `rg` retrieval is the prescribed route until growth justifies one.
  `docs/SEMANTIC-INDEX.md:L3-L20`
  `docs/SEMANTIC-INDEX.md:L22-L35`
- Sprint 001 demonstrates the route: L1 carries behavior and proof shape; L2
  points to Architecture, Implementation Plan, and Definition of Done.
  `docs/sprints/SPRINT-001.md:L7-L38`

## Gaps, conflicts, retrieval hints

- No source here defines a Gimbal builtin command registry. Treat all `python3
  .../ledger.py`, `/df-semantic-index`, and skill phase commands as workflow
  tooling instructions, not as evidence that Gimbal exposes those commands.
  `.agents/skills/df-sprint-plan/SKILL.md:L19-L21`
  `docs/SEMANTIC-INDEX.md:L35-L35`
- No source here defines Promise, a durable task model, or a general program/run
  contract. Retrieve the promise, workflow, or runtime segments before making
  those distinctions.
- For “what guides many future sprints?”, open chapter rules and template first.
  For executable status, open `docs/sprints/ledger.yaml` and the sprint doc.
  For proof, open Sprint 001 Definition of Done and Phase 4.
- For retrieval policy, open `docs/SEMANTIC-INDEX.md`, then the sprint skill's
  Semantic Index section; the current state explicitly routes through `rg`.

Editorial currentness note: Sprint 001 is historical implementation/proof guidance,
not a current storage API. Its `observation.json` checkpoint is superseded by
the current run-store implementation indexed in the operations leaf. Likewise,
the pointer’s “not yet built” status predates this scoped research index; it
is not evidence that this new index is absent. The ledger’s recorded status
is not independent evidence of implementation progress.
