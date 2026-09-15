---
name: df-sprint-plan
description: Diffusion sprint planning workflow for creating sprint docs through Claude/Codex/Gemini draft, critique, interview, and merge, using Google's agy CLI for the Gemini lane. Use this whenever the user asks to plan a sprint, create the next sprint, draft a multi-agent implementation plan, or align planned work to optional docs/chapters context.
argument-hint: <seed-prompt>
---

# Sprint Plan: Collaborative Multi-Agent Planning

You are the orchestrator of a planning workflow that produces high-quality sprint documents through competitive ideation across three planning agents (Claude, Codex, Gemini), cross-critique, human interview, and synthesis. If the repo has `docs/chapters/`, treat chapters as optional long-range planning context: they can guide sprint alignment, but sprints remain the executable unit.

Chapter handling is deliberately lightweight. Do not run chapter fan-out, chapter critique, or competing chapter drafts. Read chapter context only when it is clearly relevant, then proceed with the normal sprint fan-out.

When sprint or chapter docs have a `Pyramid Index`, use it as the first-pass
summary surface before reading deeper sections. The index should make summaries
easy to enumerate and reversible: L0 gives one sentence, L1 gives grouped
summary bullets, and L2 points to the detailed sections that expand those
bullets.

> **Bundled tool:** The sprint ledger CLI ships with this skill at
> `scripts/ledger.py`. Ledger commands below invoke it via its installed path,
> `.agents/skills/df-sprint-plan/scripts/ledger.py`, run from the repo root.

## Model selection

Before model preflight or substantive work, read [references/models.md](references/models.md)
and run the adjacent `scripts/resolve_models.py` for the target project. Apply
skill defaults, then `~/.config/diffusion/skills/models.yaml`, then the project's
`.diffusion/skills/models.yaml`. Use the resolved model and effort for every
role; the reference describes main-worker handoff and prevents recursive handoff.

```yaml
model_defaults:
  main:
    model: inherit
  claude:
    model: claude-fable-5-1
  codex:
    model: gpt-6-astra
  gemini:
    model: cli-default
```

## Arguments

```
$ARGUMENTS
```

Everything in `$ARGUMENTS` is the **seed prompt** — the user's description of what this sprint should accomplish.

In the commands below, replace `<claude-model-args>`, `<codex-model-args>`,
and `<gemini-model-args>` with the corresponding resolved `model_args` argv,
shell-quoting each argument. Defaults expand to `--model claude-fable-5-1`,
`--model gpt-6-astra`, and no Gemini model flag. Never run literal placeholders.
Use the same assignments for preflight and every later call. If a configured
model is unavailable, report that error instead of silently using a CLI default.

## Phase 0: Preflight Check (MANDATORY)

**Goal**: Verify all three planning-agent CLIs are installed and have valid API keys. The Gemini lane uses Google's `agy` CLI. **Do not skip this phase.**

Run these three commands in parallel:

```bash
which claude && claude --version
```

```bash
which codex && codex --version
```

```bash
which agy && agy --version
```

If **any** CLI is missing (`which` fails), **HALT immediately** and tell the user:

> **Sprint planning requires all three agent CLIs.** Missing: `<name>`.
> Please visit the official site for `<name>` and follow their install instructions.

For the Gemini lane, report the CLI name as `agy`.

Next, verify API keys by running these three commands in parallel:

```bash
claude <claude-model-args> -p "say ok" --dangerously-skip-permissions
```

```bash
codex exec <codex-model-args> --sandbox workspace-write "echo ok"
```

```bash
agy <gemini-model-args> -p "say ok" --dangerously-skip-permissions
```

If **any** command fails with an authentication or API key error, **HALT immediately** and tell the user:

> **Sprint planning requires valid API keys for all three agents.** `<name>` failed authentication.
> Please refer to the official `<name>` documentation for API key setup, then re-run `/df-sprint-plan`.

For the Gemini lane, report the CLI name as `agy`.

**Do not attempt to work around missing CLIs or API keys. Do not proceed with fewer than three agents. The competitive multi-agent process is the point of this skill — running with a subset defeats its purpose.**

---

## Workflow Overview

This is a **7-phase workflow** (including preflight):
0. **Preflight** — Verify all three CLIs are installed and authenticated
1. **Orient** — Review project state and recent sprints
2. **Intent** — Write concentrated intent document
3. **Draft** — Claude, Codex, and Gemini each independently draft a plan (parallel)
4. **Critique** — Each agent critiques the other two drafts (parallel)
5. **Interview** — Clarify with the human planner, informed by all drafts and critiques
6. **Merge** — Orchestrator synthesizes best ideas into final sprint document

Track progress through each phase using the adapter's task tracker when one is available; otherwise keep a concise local checklist in the working notes.

---

## Phase 1: Orient

**Goal**: Understand current project state and recent direction.

### Steps:
1. Read repo instructions if present: `AGENTS.md`, `CLAUDE.md`, or equivalent.
2. Check sprint ledger status:
   ```bash
   python3 .agents/skills/df-sprint-plan/scripts/ledger.py stats
   ```
3. Find existing sprint documents:
   ```bash
   ls docs/sprints/SPRINT-*.md 2>/dev/null | tail -3
   ```
   If no sprint directory exists yet, note this is the first sprint and jump to **Semantic Index Setup** (below) before continuing.
4. Check for semantic index configuration. This is mandatory for every sprint planning run:
   ```bash
   cat docs/SEMANTIC-INDEX.md 2>/dev/null
   ```
   Parse for two sections:

   **`## Token Cache`** — if present, verify the local path is on disk:
   ```bash
   ls "<token-cache-local-path>" 2>/dev/null | head -3
   ```
   If missing but a `Sync command` is listed, offer to run it now or note that the token cache needs syncing before full retrieval is possible.

   **`## Semantic Index`** — if present with `Status: Available`:
   - Hit the index before drafting: read the entrypoint, follow at least one relevant route, and open the most relevant leaf citations for the seed prompt. Use `rg` against the token cache local path for targeted searches the index points toward.
   - Summarize retrieved findings in the Orientation Summary as a **Semantic Prior Art** bullet.
   - Pass the `Entrypoint` path and token cache local path into the INTENT doc so planning agents can also query them.

   **`Status: Not configured`** or file absent (and this is NOT the first sprint): write a minimal `docs/SEMANTIC-INDEX.md` using the "not configured" template from **Semantic Index Setup** below, so future agents have a clear record. Continue without retrieval.
5. If `docs/chapters/ledger.yaml` exists, read it before drafting only to decide whether optional chapter context applies. Then read any chapter docs for active chapters that are directly named in the seed prompt, already linked from recent relevant sprints, or clearly aligned with the seed. Do not invent a chapter link just because chapters exist. Do not ask agents to draft, critique, or rank chapters.
6. Read the **3 most recent sprint documents** to understand recent trajectory. If any are chapter-linked, note whether that chapter is still active.
7. Identify relevant code areas for the seed prompt:
   - Search for related modules, types, or patterns
   - Note existing implementations this plan might extend

### Deliverable:
Write a brief **Orientation Summary** (3-5 bullet points) covering:
- Current project state relevant to the seed
- Recent sprint themes/direction
- Semantic prior art retrieved from the index (if available), or a note that no index is configured
- Chapter context, if applicable: selected chapter id/title, why it fits, and any active acceptance criteria the sprint should advance
- Key modules/files likely involved
- Constraints or patterns to respect

---

## Semantic Index Setup (first-sprint initialization only)

Run this section only when no `docs/sprints/SPRINT-*.md` files exist (i.e., this is the first sprint being planned in this repo).

**Two things to configure — they are separate:**

- **Token cache**: The full local materialization of all tokens relevant to the business — source code, docs, research, schemas, communications, whatever makes up the total business context. This must exist as a real POSIX filesystem so tools like `rg`, `find`, `grep`, and `wc` work on it directly. It can be enormous (hundreds of millions to billions of tokens). It can live remotely (GitHub, Google Drive, Dropbox, Box, S3, any POSIX-syncable store) but must be synced locally before agents can use it.
- **Semantic index**: A compact routing tree built over the token cache. Agents use it to peek and sniff relevant citations without ingesting the full token cache. It is much smaller and must also be on the local filesystem.

### Round 1: Token Cache

Ask up to 3 short questions using the adapter's structured question mechanism when available, otherwise ask directly:

**Question 1** — "Does your project have a token cache — a local token cache of all business-relevant tokens?"
- Yes, it's already materialized locally (provide the local path)
- Yes, but it lives remotely and needs to be synced to a local path
- Not yet — I'll set it up separately and link it later
- This project won't use a token cache

**Question 2** (if remote) — "Where does the token cache live remotely?"
- GitHub repository — provide the clone URL
- Google Drive folder — provide the Drive path (requires `rclone` with a Drive remote)
- Dropbox folder — provide the Dropbox path (requires `rclone` with a Dropbox remote)
- Box folder — provide the Box path (requires `rclone` with a Box remote)
- S3 bucket prefix — provide `s3://bucket/prefix`
- Other POSIX-syncable path — describe the sync mechanism

**Question 3** (if remote) — "What absolute local path should the token cache be materialized at?"
(e.g. `/data/token-cache/`, `~/token-cache/acme/`)

Also collect free text: "What does the token cache contain?" (scope description)

**Generating the sync command** based on remote type:

| Remote type | Initial sync command | Refresh command |
|---|---|---|
| `git` | `git clone <url> <local-path>` | `git -C <local-path> pull` |
| `rclone-gdrive` | `rclone sync "gdrive:<drive-path>" "<local-path>"` | same |
| `rclone-dropbox` | `rclone sync "dropbox:<dropbox-path>" "<local-path>"` | same |
| `rclone-box` | `rclone sync "box:<box-path>" "<local-path>"` | same |
| `s3` | `aws s3 sync s3://<bucket>/<prefix> <local-path>` | same |
| `local` | *(already materialized — no sync needed)* | — |

For rclone-based sources, note that the user must have `rclone` installed and the remote configured (`rclone config`).

### Round 2: Semantic Index

Ask up to 2 short questions using the adapter's structured question mechanism when available, otherwise ask directly:

**Question 1** — "Do you have a semantic index built over the token cache?"
- Yes, it's already built and available locally (provide the local path to its entrypoint)
- Not yet — I'll run `/df-semantic-index <token-cache-path> <index-path>` after the token cache is synced
- No — this project won't use a semantic index

**Question 2** (if available) — "What is the local path to the semantic index entrypoint?"
(e.g. `/data/token-cache-index/README.md`)

### Writing `docs/SEMANTIC-INDEX.md`

Write this file based on the interview answers. This file is permanent — future coding agents read it to discover and access both the token cache and the semantic index. Be explicit and leave nothing ambiguous.

**When token cache and/or index are available:**

```markdown
# Semantic Index Configuration

## Token Cache

The token cache is the full local materialization of the project context.
Tools like `rg`, `find`, `grep`, and `wc` operate on it directly.

**Remote source**: <GitHub URL | gdrive:<path> | dropbox:<path> | box:<path> | s3://bucket/prefix | "local only">
**Remote type**: `git` | `rclone-gdrive` | `rclone-dropbox` | `rclone-box` | `s3` | `local`
**Local path**: `/absolute/path/to/token-cache/`
**Sync command**:
```
<exact shell command to materialize or refresh the local token cache>
```
**Token cache scope**: <what this token cache contains and why it's relevant>

## Semantic Index

The semantic index is a routing tree built over the token cache.
Use it to retrieve relevant citations without scanning the full token cache.

**Status**: Available | Not yet built
**Local path**: `/absolute/path/to/index/`
**Entrypoint**: `/absolute/path/to/index/README.md`
**Access**: Read the entrypoint, follow routing nodes, open leaf citations.
Citations in leaf files resolve to paths under the token cache local path.

[If not yet built:]
To build: run `/df-semantic-index <token-cache-local-path> <index-output-path>`
```

**When neither is configured:**

```markdown
# Semantic Index Configuration

## Status: Not configured

## Notes

No token cache or semantic index has been configured for this project.
Sprint planning and execution proceed without index-assisted prior art retrieval.

To set up later:
1. Assemble the token cache (full project token cache) at a local POSIX path.
2. Link it here under ## Token Cache with the sync command.
3. Run `/df-semantic-index <token-cache-path> <index-output-path>` to build the index.
4. Update ## Semantic Index with the entrypoint path.
```

After writing `docs/SEMANTIC-INDEX.md`, continue with Phase 2.

---

## Phase 2: Intent

**Goal**: Create a concentrated intent document that all three agents will use.

### Steps:

1. Determine the next sprint number:
   ```bash
   ls docs/sprints/SPRINT-*.md 2>/dev/null | tail -1
   ```
   Extract NNN and increment. If none exist, start at `001`.

2. Create the drafts directory if needed:
   ```bash
   mkdir -p docs/sprints/drafts
   ```

3. Write the intent document to `docs/sprints/drafts/SPRINT-NNN-INTENT.md`:

```markdown
# Sprint NNN Intent: [Title]

## Seed

[The original seed prompt]

## Context

[Your orientation summary from Phase 1]

## Pyramid Index

- L0: [One sentence summarizing this sprint intent.]
- L1:
  - [3-5 bullets covering the main work surfaces, risks, and proof shape.]
- L2:
  - [Pointers to intent sections or source docs that expand each L1 point.]

## Semantic Index

[One of the following:]
- **Available** — Token cache: `<Local path from docs/SEMANTIC-INDEX.md>`. Index entrypoint: `<Entrypoint from docs/SEMANTIC-INDEX.md>`. Access: read entrypoint, follow routing nodes, open leaf citations; use `rg` against the token cache for targeted searches. Prior art retrieved: <brief bullet summary of relevant findings, or "none found for this seed">.
- **Not configured** — No token cache or semantic index is set up for this project. See `docs/SEMANTIC-INDEX.md`.

Planning agents drafting this sprint must read the index entrypoint when available, follow routing nodes to relevant leaves, and run `rg` searches against the token cache before proposing architecture or implementation approaches. Do not attempt to scan the full token cache; use the index to sniff relevant areas first.

## Chapter Context

[Optional. If applicable: selected CHAPTER-XXXX, title, chapter doc path, relevant chapter acceptance criteria, and why this sprint belongs there. If no active chapter fits, write "No chapter link selected" and explain briefly. Do not request chapter-level fan-out or critique.]

## Recent Sprint Context

[Brief summaries of recent sprints reviewed, or "First sprint" if none]

## Relevant Codebase Areas

[Key modules, files, patterns identified during orientation]

## Constraints

- Must follow project conventions in AGENTS.md
- Must integrate with existing architecture
- Chapter links are optional; if chapter-linked, preserve the chapter direction without making the chapter authoritative over sprint status
- [Any other constraints identified]

## Success Criteria

What would make this sprint successful?

## Open Questions

Questions that the drafts should attempt to answer.
```

---

## Phase 3: Draft (Claude, Codex, Gemini via agy — parallel)

**Goal**: Get three independent draft plans from competing agents.

### Launch all three planning agents in parallel:

Each agent reads the INTENT and writes its own draft. Substitute the actual sprint number for NNN.

```bash
# Claude planning agent
claude <claude-model-args> -p "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md - this is a concentrated intent for our next sprint. Fully familiarize yourself with our project structure (see AGENTS.md), planning style (see any existing docs/sprints/SPRINT-*.md files), and any Chapter Context in the intent. If a chapter is selected, read its docs/chapters/CHAPTER-*.md file and keep the plan aligned to that chapter without making the chapter authoritative over sprint status. If the intent's Semantic Index section shows an available index, read the entrypoint file and follow routing nodes to retrieve prior art relevant to the sprint seed before proposing architecture or implementation approaches. Then draft a comprehensive sprint plan to docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md following the sprint template structure: Overview, Use Cases, Architecture, Implementation Plan (phased with files and tasks), Files Summary, Definition of Done, Risks & Mitigations, Dependencies, and Open Questions." --dangerously-skip-permissions &

# Codex planning agent
codex exec <codex-model-args> --sandbox workspace-write "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md - this is a concentrated intent for our next sprint. Fully familiarize yourself with our project structure (see AGENTS.md), planning style (see any existing docs/sprints/SPRINT-*.md files), and any Chapter Context in the intent. If a chapter is selected, read its docs/chapters/CHAPTER-*.md file and keep the plan aligned to that chapter without making the chapter authoritative over sprint status. If the intent's Semantic Index section shows an available index, read the entrypoint file and follow routing nodes to retrieve prior art relevant to the sprint seed before proposing architecture or implementation approaches. Then draft a comprehensive sprint plan to docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md following the sprint template structure: Overview, Use Cases, Architecture, Implementation Plan (phased with files and tasks), Files Summary, Definition of Done, Risks & Mitigations, Dependencies, and Open Questions." &

# Gemini planning agent through Google's agy CLI
agy <gemini-model-args> -p "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md - this is a concentrated intent for our next sprint. Fully familiarize yourself with our project structure (see AGENTS.md), planning style (see any existing docs/sprints/SPRINT-*.md files), and any Chapter Context in the intent. If a chapter is selected, read its docs/chapters/CHAPTER-*.md file and keep the plan aligned to that chapter without making the chapter authoritative over sprint status. If the intent's Semantic Index section shows an available index, read the entrypoint file and follow routing nodes to retrieve prior art relevant to the sprint seed before proposing architecture or implementation approaches. Then draft a comprehensive sprint plan to docs/sprints/drafts/SPRINT-NNN-GEMINI-DRAFT.md following the sprint template structure: Overview, Use Cases, Architecture, Implementation Plan (phased with files and tasks), Files Summary, Definition of Done, Risks & Mitigations, Dependencies, and Open Questions." --dangerously-skip-permissions &

wait
```

### Wait for all three to complete, then read all three drafts:
- `docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-NNN-GEMINI-DRAFT.md`

---

## Phase 4: Critique (Claude, Codex, Gemini via agy — parallel)

**Goal**: Each agent reads and critiques the other two competing drafts.

### Launch all three planning agents in parallel:

Each agent reads the two drafts it did NOT write and produces a single critique file covering both.

```bash
# Claude critiques Codex and Gemini drafts
claude <claude-model-args> -p "You are reviewing two competing sprint plan drafts. Read docs/sprints/drafts/SPRINT-NNN-INTENT.md for context, then read both docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md and docs/sprints/drafts/SPRINT-NNN-GEMINI-DRAFT.md. Write your critique to docs/sprints/drafts/SPRINT-NNN-CLAUDE-CRITIQUE.md. For each draft, evaluate: architectural soundness, completeness, phasing/ordering, risk coverage, feasibility, and definition of done. Note the strongest ideas worth keeping from each, and weaknesses or gaps." --dangerously-skip-permissions &

# Codex critiques Claude and Gemini drafts
codex exec <codex-model-args> --sandbox workspace-write "You are reviewing two competing sprint plan drafts. Read docs/sprints/drafts/SPRINT-NNN-INTENT.md for context, then read both docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md and docs/sprints/drafts/SPRINT-NNN-GEMINI-DRAFT.md. Write your critique to docs/sprints/drafts/SPRINT-NNN-CODEX-CRITIQUE.md. For each draft, evaluate: architectural soundness, completeness, phasing/ordering, risk coverage, feasibility, and definition of done. Note the strongest ideas worth keeping from each, and weaknesses or gaps." &

# Gemini critiques Claude and Codex drafts through Google's agy CLI
agy <gemini-model-args> -p "You are reviewing two competing sprint plan drafts. Read docs/sprints/drafts/SPRINT-NNN-INTENT.md for context, then read both docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md and docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md. Write your critique to docs/sprints/drafts/SPRINT-NNN-GEMINI-CRITIQUE.md. For each draft, evaluate: architectural soundness, completeness, phasing/ordering, risk coverage, feasibility, and definition of done. Note the strongest ideas worth keeping from each, and weaknesses or gaps." --dangerously-skip-permissions &

wait
```

### Wait for all three to complete, then read all three critiques:
- `docs/sprints/drafts/SPRINT-NNN-CLAUDE-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-NNN-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-NNN-GEMINI-CRITIQUE.md`

---

## Phase 5: Interview

**Goal**: Refine understanding through human dialogue, now informed by all drafts and critiques.

### Before the interview:

Briefly summarize for the user:
- The key differences across the three drafts
- Where the critiques agree (consensus weaknesses or strengths)
- Any unresolved tensions or open questions

### Conduct the interview:

Ask **2-4 targeted questions** using the adapter's structured question mechanism when available, otherwise ask directly. Cover:

1. **Draft preference**: "Having seen the three approaches, which direction resonates most?"
2. **Scope validation**: "Should we expand or narrow the scope based on what you've seen?"
3. **Priority/trade-offs**: "Which aspects are most critical vs. nice-to-have?"
4. **Technical preferences**: Any strong opinions on approach, especially where drafts diverge?

Note the answers — incorporate refinements in the merge phase.

---

## Phase 6: Merge

**Goal**: Orchestrator synthesizes the best ideas from all drafts into a final sprint document.

### Merge process:

1. **Identify consensus** across critiques:
   - Where do multiple agents agree on a strength or weakness?
   - Which ideas appear in multiple drafts independently?

2. **Compare all three drafts**:
   - Architecture approach differences
   - Phasing/ordering differences
   - Risk identification gaps
   - Definition of Done completeness
   - Novel ideas unique to one draft
   - If chapter context was selected, whether each draft stays aligned with that chapter without expanding into chapter planning

3. **Document the synthesis**:

   Write to `docs/sprints/drafts/SPRINT-NNN-MERGE-NOTES.md`:
   ```markdown
   # Sprint NNN Merge Notes

   ## Claude Draft Strengths
   - ...

   ## Codex Draft Strengths
   - ...

   ## Gemini Draft Strengths
   - ...

   ## Consensus Critiques (multiple agents agreed)
   - ...

   ## Valid Critiques Accepted
   - ...

   ## Critiques Rejected (with reasoning)
   - ...

   ## Interview Refinements Applied
   - ...

   ## Final Decisions
   - ...
   ```

4. **Write the final sprint document**:

   Create `docs/sprints/SPRINT-NNN.md` incorporating:
   - Best ideas from all three drafts
   - Responses to valid critiques (especially consensus ones)
   - Interview refinements

   Follow the standard sprint template:
   ```markdown
   # Sprint NNN: [Title]

   Chapter: `CHAPTER-XXXX` - [Chapter Title]

   ## Pyramid Index

   - L0: [One sentence summarizing the sprint.]
   - L1:
     - [3-5 bullets covering implementation surfaces, acceptance, and risk.]
   - L2:
     - [Pointers to sections below that expand each L1 point.]

   ## Overview
   ## Use Cases
   ## Architecture
   ## Implementation Plan
   ## Files Summary
   ## Definition of Done
   ## Risks & Mitigations
   ## Dependencies
   ## Open Questions
   ```

   Omit the `Chapter:` line when no chapter was selected.

5. **Update the ledger and chapter index**:
   ```bash
   python3 .agents/skills/df-sprint-plan/scripts/ledger.py sync
   ```
   If the repo uses `docs/sprints/ledger.yaml`, ensure the sprint entry includes `chapter: CHAPTER-XXXX` when a chapter was selected. If the bundled ledger helper supports it, prefer:
   ```bash
   python3 .agents/skills/df-sprint-plan/scripts/ledger.py set-chapter SPRINT-NNN CHAPTER-XXXX
   ```
   If `docs/chapters/ledger.yaml` exists, also ensure that chapter's `sprint_ids` list includes the new sprint id. Preserve the rule that `docs/sprints/ledger.yaml` remains authoritative for individual sprint status.

6. **Show the user** the final document and ask for approval.

---

## File Structure

After df-sprint-plan completes, you'll have:

```
docs/sprints/
├── drafts/
│   ├── SPRINT-NNN-INTENT.md              # Concentrated intent (Phase 2)
│   ├── SPRINT-NNN-CLAUDE-DRAFT.md        # Claude draft (Phase 3)
│   ├── SPRINT-NNN-CODEX-DRAFT.md         # Codex draft (Phase 3)
│   ├── SPRINT-NNN-GEMINI-DRAFT.md        # Gemini draft (Phase 3)
│   ├── SPRINT-NNN-CLAUDE-CRITIQUE.md     # Claude critiques Codex + Gemini (Phase 4)
│   ├── SPRINT-NNN-CODEX-CRITIQUE.md      # Codex critiques Claude + Gemini (Phase 4)
│   ├── SPRINT-NNN-GEMINI-CRITIQUE.md     # Gemini critiques Claude + Codex (Phase 4)
│   └── SPRINT-NNN-MERGE-NOTES.md         # Synthesis notes (Phase 6)
└── SPRINT-NNN.md                          # Final merged sprint
```

---

## Output Checklist

At the end of this workflow, you should have:
- [ ] Orientation summary complete
- [ ] `docs/SEMANTIC-INDEX.md` exists: either written during setup (first sprint), already present (subsequent sprints), or created as "Not configured" (no index)
- [ ] Semantic prior art retrieved from the index (if available) and summarized in the Orientation Summary
- [ ] Intent document written (`drafts/SPRINT-NNN-INTENT.md`) with a populated `Semantic Index` section
- [ ] Chapter context evaluated; if selected, active chapter doc read and referenced
- [ ] All three drafts received (Claude, Codex, Gemini)
- [ ] All three critiques received (each critiquing the other two)
- [ ] Interview conducted (2-4 questions answered)
- [ ] Merge notes written (`drafts/SPRINT-NNN-MERGE-NOTES.md`)
- [ ] Final sprint document written (`SPRINT-NNN.md`) with a concise `Pyramid Index`
- [ ] Ledger updated via `python3 .agents/skills/df-sprint-plan/scripts/ledger.py sync`; chapter field and `docs/chapters/ledger.yaml` `sprint_ids` updated if applicable
- [ ] User approved the final document
