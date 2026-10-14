---
name: df-sprint-execute
description: Diffusion sprint execution workflow for handing a planned sprint to claude, codex, or gemini, with Gemini routed through Google's agy CLI, and reviewing results. Use this whenever the user asks to execute, run, implement, or finish a planned sprint, especially when sprint ledgers, chapter links, or Pyramid Index summaries must be preserved.
argument-hint: <agent> [sprint-number]
---

# Sprint Execute

Execute a sprint plan using a specified agent CLI. Chapters are optional context. If the sprint is linked to a chapter, load that chapter first and keep the implementation aligned with the chapter vector while preserving the sprint as the executable unit. Do not run chapter-level fan-out or critique during execution. When sprint or chapter docs include a `Pyramid Index`, read it first as the summary index, then expand into detailed sections as needed.

> **Dependency:** This skill drives the sprint ledger CLI bundled with the
> `df-sprint-plan` skill in its `scripts/ledger` Go module.
> Install both skills together. Substitute the loaded planning skill directory
> below, and run commands from the target repo root so `--project "$PWD"`
> names the project rather than the installed skill.

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

Parse the arguments:
- **First word** — the agent to use: `claude`, `codex`, or `gemini` (required). The `gemini` agent runs through Google's `agy` CLI.
- **Second word** — optional sprint number (e.g., `007`). If omitted, determine the current sprint from the ledger.

If no agent is specified, **HALT** and tell the user:

> Usage: `/df-sprint-execute <agent> [sprint-number]`
> Agent must be one of: `claude`, `codex`, or `gemini`.

---

In the commands below, replace `<claude-model-args>`, `<codex-model-args>`,
and `<gemini-model-args>` with the corresponding resolved `model_args` argv,
shell-quoting each argument. Defaults expand to `--model claude-fable-5-1`,
`--model gpt-6-astra`, and no Gemini model flag. Never run literal placeholders.
Use the same assignments for preflight and every later call. If a configured
model is unavailable, report that error instead of silently using a CLI default.

## Phase 0: Preflight Check (MANDATORY)

**Goal**: Verify the chosen agent CLI is installed and authenticated. **Do not skip this phase.**

### Check CLI is installed:

```bash
which <agent-cli> && <agent-cli> --version
```

Use `agy` as `<agent-cli>` when `<agent>` is `gemini`.

If the CLI is missing, **HALT immediately**:

> **`<agent-cli>` CLI is not installed for `<agent>`.** Please visit the official site and follow their install instructions, then re-run `/df-sprint-execute`.

### Check API key works:

| Agent | Command |
|-------|---------|
| claude | `claude <claude-model-args> -p "say ok" --dangerously-skip-permissions` |
| codex | `codex exec <codex-model-args> --sandbox workspace-write "echo ok"` |
| gemini | `agy <gemini-model-args> -p "say ok" --dangerously-skip-permissions` |

If authentication fails, **HALT immediately**:

> **`<agent>` failed authentication.** Please refer to the official documentation for API key setup, then re-run `/df-sprint-execute`.

**Do not attempt to work around a missing CLI or API key.**

---

## Phase 1: Load Sprint

**Goal**: Identify and read the sprint plan to execute.

### Steps:

1. If a sprint number was provided, use it. Otherwise, find the current sprint:
   ```bash
   go run -C "<loaded-df-sprint-plan-dir>/scripts/ledger" . --project "$PWD" current
   ```
   If no sprint is in progress, check for the next planned one:
   ```bash
   go run -C "<loaded-df-sprint-plan-dir>/scripts/ledger" . --project "$PWD" next
   ```

2. Read the sprint document:
   ```bash
   cat docs/sprints/SPRINT-NNN.md
   ```
   If it doesn't exist, **HALT** and tell the user there is no sprint document for that number.

3. Check for semantic index configuration. This is mandatory for every sprint execution run:
   ```bash
   cat docs/SEMANTIC-INDEX.md 2>/dev/null
   ```
   Parse the file for two sections:

   **Token cache** — if a `## Token Cache` section exists:
   - Read the `Local path` field and verify it is present on disk:
     ```bash
     ls "<token-cache-local-path>" 2>/dev/null | head -3
     ```
   - If the path is missing or empty and a `Sync command` is listed, run the sync command now to materialize the token cache before execution. Inform the user: "Syncing token cache from remote — this may take a moment."
   - If the path is present, note its existence and approximate size (`du -sh "<local-path>" 2>/dev/null`).

   **Semantic index** — if a `## Semantic Index` section exists with `Status: Available`:
   - Extract the `Entrypoint` path and `Access` instructions.
   - Verify the entrypoint file exists: `ls "<entrypoint-path>" 2>/dev/null`
   - Resolve index hits for the sprint before launching execution: read the entrypoint, follow the routes relevant to the sprint's implementation plan, and open the most relevant leaf citations. Inject those concrete routes and citations into the execution prompt so the executor starts from resolved prior art, not just a pointer.

   If `docs/SEMANTIC-INDEX.md` is absent or the status is `Not configured`, note this and proceed without index retrieval.

4. Determine whether the sprint is chapter-linked:
   - Check for a `Chapter: ` line near the top of `docs/sprints/SPRINT-NNN.md`
   - If `docs/sprints/ledger.yaml` exists, check whether the sprint entry has `chapter: CHAPTER-XXXX`
   - If `docs/chapters/ledger.yaml` exists, verify that the chapter's `sprint_ids` includes this sprint

   If a chapter is linked, read `docs/chapters/ledger.yaml` and the chapter document before execution. Carry the chapter's vector, remaining shape, acceptance criteria, and non-goals into the execution prompt.

5. Mark the sprint as in progress (if not already):
   ```bash
   go run -C "<loaded-df-sprint-plan-dir>/scripts/ledger" . --project "$PWD" start NNN
   ```
   If the repo uses YAML status values instead of the bundled TSV helper, use the repo's ledger helper or update `docs/sprints/ledger.yaml` according to the repo convention. Preserve any existing `chapter:` field.

---

## Phase 2: Execute

**Goal**: Hand the sprint plan to the chosen agent for execution.

### Build the prompt:

The prompt sent to the agent should be:

> You are executing a sprint plan. Read the sprint document at docs/sprints/SPRINT-NNN.md and work through every task in the Implementation Plan, phase by phase, in order. For each task:
> 1. Implement the change described
> 2. Verify it works (run tests, type checks, etc. as appropriate)
> 3. Move on to the next task
>
> Follow all project conventions in AGENTS.md, CLAUDE.md, or equivalent. Do not skip tasks. Do not reorder phases unless a dependency requires it. If you hit a blocker you cannot resolve, write a note to docs/sprints/drafts/SPRINT-NNN-BLOCKERS.md describing what is blocked and why, then continue with the next unblocked task.
>
> [TOKEN CACHE + SEMANTIC INDEX — include this block only when docs/SEMANTIC-INDEX.md has an Available semantic index]
> A token cache (full project context) and semantic index are available.
> Token cache local path: <Local path from docs/SEMANTIC-INDEX.md ## Token Cache>
> Semantic index entrypoint: <Entrypoint from docs/SEMANTIC-INDEX.md ## Semantic Index>
> Access: Read the entrypoint, follow routing nodes to leaf citations. Citations resolve to files under the token cache path — use rg, find, or direct reads to inspect them.
> Resolved index hits for this sprint:
> <Concrete route files and leaf citations opened by the orchestrator before launch. Include "no relevant hits found" only after reading the entrypoint and following likely routes.>
> Before beginning each implementation phase, use these resolved hits and query the index again for prior art relevant to that phase's work area. Use rg against the token cache for targeted searches the index points toward. Incorporate relevant patterns, avoid known pitfalls, and note citations you relied on in docs/sprints/drafts/SPRINT-NNN-BLOCKERS.md or inline comments.
> [END TOKEN CACHE + SEMANTIC INDEX BLOCK]
>
> If this sprint is linked to a chapter, also read the linked chapter document and keep changes aligned with that chapter's vector and non-goals. Preserve the chapter link in the sprint document and sprint ledger. Do not mark chapter status done, paused, or abandoned unless the sprint plan explicitly calls for chapter review or closure.
>
> If the sprint document has a Pyramid Index, preserve it. If execution materially changes the sprint's actual scope or proof shape, update the Pyramid Index so L0/L1/L2 still summarize the finished work accurately.

When the semantic index block applies, substitute the actual entrypoint and access instructions from `docs/SEMANTIC-INDEX.md` before launching the agent.

### Launch the agent:

| Agent | Command |
|-------|---------|
| claude | `claude <claude-model-args> -p "<prompt>" --dangerously-skip-permissions` |
| codex | `codex exec <codex-model-args> --sandbox workspace-write "<prompt>"` |
| gemini | `agy <gemini-model-args> -p "<prompt>" --dangerously-skip-permissions` |

### Wait for the agent to complete.

---

## Phase 3: Review

**Goal**: Verify execution results and update the ledger.

### Steps:

1. Check if any blockers were recorded:
   ```bash
   cat docs/sprints/drafts/SPRINT-NNN-BLOCKERS.md 2>/dev/null
   ```

2. Run the project's test suite (if one exists) to verify the work.

3. If the sprint is chapter-linked, verify that:
   - `docs/sprints/SPRINT-NNN.md` still has the correct `Chapter:` line
   - `docs/sprints/ledger.yaml`, when present, still carries `chapter: CHAPTER-XXXX`
   - `docs/chapters/ledger.yaml`, when present, still lists `SPRINT-NNN` under the chapter's `sprint_ids`

4. Report results to the user:
   - Tasks completed
   - Any blockers encountered
   - Test results
   - Chapter link preserved, or any chapter-ledger correction made

5. Mark the sprint completed when validation passed and the user's request clearly asked for full execution. If the user only asked for a partial run or review, ask before completing it:
   ```bash
   go run -C "<loaded-df-sprint-plan-dir>/scripts/ledger" . --project "$PWD" complete NNN
   ```
   If the repo uses YAML status values, use the repo's status command or update the YAML status according to repo convention, preserving chapter fields.
