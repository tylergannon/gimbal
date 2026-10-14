---
name: df-chapter-create
description: Diffusion chapter creation workflow for concise long-range docs/chapters vectors spanning roughly 12-100 sprints. Use this whenever the user asks to create, plan, draft, start, or define a chapter, long-term vector, multi-sprint arc, architecture direction, design theme, or durable roadmap without replacing sprint planning.
argument-hint: <chapter-seed>
---

# Chapter Create

Create a concise chapter: an optional long-term vector for a series of future
sprints. A chapter should shape design direction, architecture, product themes,
and acceptance posture across at least a dozen sprints and possibly up to 100.
It is not an executable sprint, not an OKR system, and not a fan-out planning
workflow.

Do not run multi-agent draft/critique for chapters. Write one clear chapter
from repo context and the user's seed. Future sprint planning can optionally
read the chapter as context.

Chapters should include a small pyramid index: reversible summaries at multiple
zoom levels. This makes enumeration cheap for future sprint planners while still
letting an agent expand into the full chapter when needed.

## Model selection

Before model preflight or substantive work, read [references/models.md](references/models.md)
and run the adjacent `scripts/resolve_models` for the target project. Apply
skill defaults, then `~/.config/diffusion/skills/models.yaml`, then the project's
`.diffusion/skills/models.yaml`. Use the resolved model and effort for every
role; the reference describes main-worker handoff and prevents recursive handoff.

```yaml
model_defaults:
  main:
    model: inherit
```

## Arguments

```text
$ARGUMENTS
```

Everything in `$ARGUMENTS` is the chapter seed: the user's description of the
long-term direction, capability, architecture, or theme.

## Chapter Rules

- Chapters are optional. Do not create one for ordinary sprint-sized work.
- Chapters guide sprints, but sprint docs and sprint ledgers remain the source
  of truth for executable work and status.
- A chapter should plausibly guide 12-100 sprints.
- A chapter should focus on design direction, architecture, themes, principles,
  doctrine, durable capability shape, and review posture.
- Keep the chapter document under 3000 tokens so sprint planners can read it
  without drowning out current sprint context.
- Include a `Pyramid Index` near the top. Keep it short enough that an agent can
  scan all chapters by reading only pyramid indexes first.
- Do not include implementation task lists fine-grained enough to belong in a
  sprint.
- Do not create competing chapter drafts, agent critiques, or a chapter
  interview unless the user explicitly asks for more discussion before writing.

## Phase 1: Orient

Read only the context needed to draft the chapter.

1. Read repo instructions if present: `AGENTS.md`, `CLAUDE.md`, or equivalent.
2. Inspect existing chapter state:
   ```bash
   ls docs/chapters 2>/dev/null || true
   cat docs/chapters/ledger.yaml 2>/dev/null || true
   ```
3. Read the most relevant existing chapter docs, if any. Avoid broad scans.
4. Review recent sprint direction:
   ```bash
   ls docs/sprints/SPRINT-*.md 2>/dev/null | tail -5
   cat docs/sprints/ledger.yaml 2>/dev/null | tail -80
   ```
5. Search for design docs, architecture notes, and modules directly related to
   the seed.

## Phase 2: Decide Scope

Before writing, decide whether a chapter is warranted.

Create a chapter when the seed describes a durable vector that should influence
many future sprints. If the work is one sprint, a short sequence of sprints, or
mostly implementation detail, tell the user it should be a sprint instead and
stop unless they explicitly want a chapter anyway.

If an existing active chapter already covers the seed, prefer updating or
referencing that chapter instead of creating a duplicate.

## Phase 3: Reserve ID

Create `docs/chapters/` if needed. Use the bundled helper when available:

```bash
GOWORK=off go run -C "<loaded-skill-dir>/scripts/chapter" . --project "$PWD" next-id
```

Chapter ids use `CHAPTER-XXXX`, starting at `CHAPTER-0001`.

## Phase 4: Write The Chapter

Write `docs/chapters/CHAPTER-XXXX-slug.md`.

Use this structure unless the repo has a stronger local chapter format:

```markdown
# CHAPTER-XXXX: [Title]

Chapter: `CHAPTER-XXXX`
Status: active

[One-paragraph summary of the durable vector.]

## Pyramid Index

- L0: [One sentence naming the chapter vector.]
- L1:
  - [3-5 bullets covering the major design or architecture directions.]
- L2:
  - [Pointers to the sections below that expand each L1 point.]

## Vector

[The long-term direction this chapter gives future sprints.]

## Design Direction

[Product, UX, operating, or domain design themes to preserve.]

## Architecture Principles

[Durable architecture choices, boundaries, source-of-truth rules, and
integration posture.]

## Sprint Horizon

[What kinds of sprint work should belong here across roughly 12-100 sprints.
Use bullets, not a detailed sprint backlog.]

## Review Posture

[How future planners and executors should tell whether work is advancing the
chapter.]

## Non-Goals

[What this chapter deliberately does not cover.]

## Sprint Planning Notes

[Concise instructions future sprint planners should carry forward.]
```

Keep it concise and scannable. Prefer strong principles and boundaries over a
large inventory of possible tasks. The pyramid index is an index, not a second
chapter body; do not duplicate every detail there.

## Phase 5: Update Ledger

Add or update `docs/chapters/ledger.yaml`:

```yaml
chapters:
  - id: CHAPTER-XXXX
    title: [Title]
    status: active
    doc: docs/chapters/CHAPTER-XXXX-slug.md
    created: "YYYY-MM-DDTHH:MM:SSZ"
    updated: "YYYY-MM-DDTHH:MM:SSZ"
    summary: >-
      [One concise summary.]
    sprint_ids: []
```

Use the helper when available:

```bash
GOWORK=off go run -C "<loaded-skill-dir>/scripts/chapter" . --project "$PWD" add "Title" \
  --doc docs/chapters/CHAPTER-XXXX-slug.md \
  --summary "One concise summary."
```

If there are already clearly related sprint ids, include them only when the repo
already treats those sprints as part of the chapter or the user asked for it.
Do not force every future sprint into the new chapter.

## Phase 6: Verify

Before finishing:

1. Confirm the document is under 3000 tokens by rough word count:
   ```bash
   wc -w docs/chapters/CHAPTER-XXXX-slug.md
   ```
   A 3000-token chapter should usually be under about 2200 words. Prefer much
   shorter when possible.
2. Confirm the ledger references the doc path and chapter id.
3. Report the created chapter id, document path, ledger update, and any existing
   sprint ids linked.

## Output Checklist

- [ ] Chapter is warranted and not just a sprint.
- [ ] Existing active chapters checked for overlap.
- [ ] Chapter doc created under `docs/chapters/`.
- [ ] Chapter doc is under 3000 tokens.
- [ ] Chapter ledger updated.
- [ ] No chapter fan-out, competing drafts, or critique workflow was run.
