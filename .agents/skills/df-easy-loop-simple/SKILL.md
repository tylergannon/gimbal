---
name: df-easy-loop-simple
description: Orchestrates a multi-model delivery workflow driven by an existing spec document — plan, critique, plan update, then a coding/review loop — each role its own foreground Claude Code or Codex CLI process with per-role models and session resume. Use when the user invokes df-easy-loop-simple or already has a document describing what to build and wants it executed without a requirements interview.
---

# df-easy-loop-simple

You — the agent that invoked this skill — are the top-level orchestrator agent
in the workflow definition below. Follow the workflow exactly, with
model configuration applied as described below.

## Model selection

Before model preflight or substantive work, read [references/models.md](references/models.md)
and run the adjacent `scripts/resolve_models` for the target project. Apply
skill defaults, then `~/.config/diffusion/skills/models.yaml`, then the project's
`.diffusion/skills/models.yaml`. Use the resolved model and effort for every
role; the reference describes main-worker handoff and prevents recursive handoff.

Defaults are the `model` and `reasoning_effort` fields in the workflow below.

## Workflow

```yaml
# df-easy-loop-simple workflow definition
#
# The orchestrator invokes the sub-agents defined below, in the order dictated by
# each agent's routing. It must not do the sub-agents' work itself and must not
# read into the sessions/artifacts the sub-agents produce.
#
# There is no requirements interview. The workflow's input is a spec document —
# a file, supplied by the user, that describes what is being built.
#
# NOTE ON PROMPTS: each agent's `prompt_construction` is NOT handed to the
# sub-agent verbatim; it instructs the top-level agent how to build that
# sub-agent's prompt. Follow it carefully; do not invent new sentences.
#
# The top-level agent must not write software to manage this process, but instead
# manage the process itself. Run each coding-agent CLI in the foreground and wait
# for that process to exit. Do not return until the workflow reaches `end`,
# unless an invocation meets the explicit terminal-error definition below.

run:
  root: "~/.easyloop/runs" # each run gets its own timestamped directory here
  dir_format: "YYYYMMDD_HHMMSS"
  setup_dir: "setup" # this skill's SKILL.md is copied here at the start of a run
  agents_dir: "agents" # per-agent dirs live at run_dir/agents/<slug>/
  # Each visit to an agent gets its own numbered directory
  # run_dir/agents/<slug>/<NNN>/, starting at 001, 3-digit zero-pad.
  visit_dir_format: "%03d"
  visit_start: 1
  outcome_file: "outcome.yaml" # contains only: `next: <agent-slug>`  (or `next: end`)
  # Missing-artifact recheck policy after a successful CLI exit; rechecks never
  # relaunch an agent.
  artifact_recheck_attempts: 5
  artifact_recheck_interval_seconds: 10
  # CLI session/thread id from each invocation (used for `context: continue`).
  session_id_file: "session-id.txt"

# Global default cap on how many times any single agent may be visited. Per-agent
# `max_visits_per_step` overrides this. The top-level agent enforces caps; once an agent
# reaches its cap, the top-level agent routes to `end` instead of the agent's
# normal `next` options. This rule is NOT placed in any sub-agent's prompt.
max_visits_per_step: 50

# Runtime inputs supplied to the top-level agent OUTSIDE this workflow. If any is not
# provided, the top-level agent must ask for it (see top_level_agent.prompt).
inputs:
  # Claude Code config dir (exported as CLAUDE_CONFIG_DIR).
  claude_config_dir:
    default: "~/.claude"
  # Codex config dir (exported as CODEX_HOME).
  codex_config_dir:
    default: "~/.codex"
  # The directory the coding/review work operates on.
  working_dir:
    default: # none; the top-level agent proposes the current working directory
  # The spec document describing what is being built.
  spec_document:
    default: # none; the top-level agent must ask for its path if not provided

# How the top-level agent runs each model, and which CLI to use.
#
# PLACEHOLDER SUBSTITUTION (important): the `<...>` tokens in the templates below
# and in `base_prompt` are NOT shell variables. Before running a command, replace
# each token IN THE COMMAND TEXT with its literal, fully-resolved absolute path or
# value (expand `~` yourself). Do NOT introduce your own shell variables (e.g.
# $WORKING_DIR) for these; an undefined shell variable expands to an empty string
# and will silently produce a blank path. Verify the final command string contains
# no `<...>` tokens, no `$`-vars, and no empty operands before you run it.
#
# REASONING EFFORT: when an agent defines a `reasoning_effort:` field, substitute
# it into that CLI's effort flag (Claude Code: `--effort <reasoning_effort>`;
# Codex: `-c model_reasoning_effort="<reasoning_effort>"`). When the agent has NO
# `reasoning_effort` field, OMIT that entire effort flag so the model runs at its
# own default. Valid values: Codex = none|minimal|low|medium|high|xhigh|max|ultra;
# Claude Code = low|medium|high|xhigh|max.
clis:
  claude_code:
    matches_model_prefix: "claude-"
    # Config dir is passed ONLY via env var (no flag exists).
    config_env: CLAUDE_CONFIG_DIR
    run_template: |
      CLAUDE_CONFIG_DIR=<claude_config_dir> claude -p "<PROMPT>" \
        --output-format json \
        --permission-mode bypassPermissions \
        --add-dir <run_dir> --add-dir <working_dir> \
        --effort <reasoning_effort> \
        --model <model>
    # There is no --cwd flag; set the subprocess cwd to <working_dir> and grant
    # roots with repeated --add-dir.
    session_id_from: "the `session_id` field of the --output-format json result"
    resume_template: |
      # append to the run template instead of a fresh -p prompt:
      --resume <session_id>
  codex:
    matches_model_prefix: "gpt-"
    # Config dir is passed ONLY via env var (no flag exists); it must already
    # exist before launch (see setup step 4).
    config_env: CODEX_HOME
    run_template: |
      CODEX_HOME=<codex_config_dir> codex exec --json \
        --skip-git-repo-check \
        --dangerously-bypass-approvals-and-sandbox \
        --cd <working_dir> \
        -c model_reasoning_effort="<reasoning_effort>" \
        --model <model> \
        "<PROMPT>"
    session_id_from: "the thread id in the `thread.started` event on stdout"
    resume_template: |
      # resume is a SUBCOMMAND (not a --resume flag); the id is a thread id:
      CODEX_HOME=<codex_config_dir> codex exec --json \
        --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox \
        --cd <working_dir> \
        -c model_reasoning_effort="<reasoning_effort>" \
        --model <model> \
        resume <thread_id> "<PROMPT>"

top_level_agent:
  model: inherit
  prompt: |
    You are the top-level orchestrator. Your responsibility is to invoke the other
    agents defined in this workflow. You must not do their work for them, and you
    should not look into the agent sessions or the artifacts the sub-agents create.
    Do not invent new sentences when constructing sub-agent prompts; follow each
    agent's `prompt_construction` carefully.

    Startup inputs:
      - claude_config_dir / codex_config_dir: if not provided, ask for each.
        Propose the defaults (`~/.claude`, `~/.codex`), and also look for
        existing `~/.claude*` / `~/.codex*` directories and offer them as
        options.
      - working_dir: tell the user the current working directory and ask whether
        that is where the work should happen; use their answer.
      - spec_document: if not provided, ask for the path to the document that
        describes what is being built. Verify the file exists; resolve it to an
        absolute path. Do not read its contents.

    Resolve model configuration after startup inputs, as described in
    references/models.md. Apply any main-worker handoff before creating a run.

    Setup, at the start of the run:
      1. Create the run directory at `<run.root>/<YYYYMMDD_HHMMSS>` (per
         run.dir_format).
      2. Copy this skill's SKILL.md into `<run_dir>/<run.setup_dir>/`.
         Save the resolved configuration alongside it as `resolved-models.json`;
         use those frozen assignments for every visit, including resumes.
      3. Create `<run_dir>/<run.agents_dir>/` and, under it, one subdirectory per
         agent named with the agent's slug.
      4. Ensure codex_config_dir exists before any Codex invocation (mkdir -p);
         Codex will not create its config root.

    Begin the invocation loop with the `plan` agent.

    For each agent invocation:
      1. One routed visit means exactly one CLI launch. Immediately before that
         launch, create the next zero-padded visit-number directory under the
         agent's subdirectory (`<run_dir>/agents/<slug>/<NNN>/`, starting at
         001). Do not launch the same logical visit a second time.
      2. Run the CLI command synchronously in the foreground. Do not append shell
         backgrounding, request a background agent, or enable Remote Control for
         the invocation. Wait for the exact launched process to exit before
         evaluating its artifacts or routing to another agent.
      3. Use the saved resolved model/effort for this role, replacing its
         defaults below. Choose the CLI by the resolved model prefix (`claude-` -> Claude Code,
         `gpt-` -> Codex) and use the matching template in `clis`. Export the
         correct config-dir env var; set the working directory to <working_dir>.
         Substitute every placeholder per the PLACEHOLDER SUBSTITUTION rules in
         `clis`; before running, confirm the final command contains no `<...>`
         tokens, no `$`-vars, and no empty path operands.
      4. Build the agent's prompt by combining `base_prompt` with that agent's
         `prompt_construction`. Supply, in the prompt: the full path to the
         agent's visit directory, and the full path(s) to the artifact(s) it must
         read (per the agent's `reads`).
      5. After the process exits successfully, verify that the expected output
         artifact(s) and `outcome.yaml` exist. If anything appears absent, wait
         `run.artifact_recheck_interval_seconds` and recheck the same paths, up to
         `run.artifact_recheck_attempts` times; do not relaunch the agent. If
         anything is still absent after all rechecks, treat the invocation as a
         terminal orchestration error.
         A nonzero CLI exit or explicit CLI error event is also a terminal error.
         Do not infer that an exited process is still working in the background.
      6. Capture the CLI's session/thread id from its output and write it to
         `<visit_dir>/<run.session_id_file>`. If a successful invocation emits no
         id, write `unavailable` to the session-id file and proceed only when the
         agent's context is `empty`; for a `continue` agent, a missing id is a
         terminal orchestration error.
      7. If the agent's `context` is `continue` AND a session/thread id from a
         PRIOR visit of this same agent exists, invoke via the CLI's
         `resume_template` (Claude: `--resume <session_id>`; Codex:
         `codex exec ... resume <thread_id> "<PROMPT>"`). If `context` is `empty`,
         always start a fresh session. The first visit of any agent is always
         fresh.
      8. It is the sub-agent's responsibility to write `<run.outcome_file>` into
         its visit directory naming the agent to invoke next. Read only that file
         to decide routing.
      9. A visit is complete only when its expected output artifact(s) exist and
         its `outcome.yaml` contains one allowed `next` value. Read only
         `outcome.yaml` to route; checking the expected artifact's existence is
         allowed, but reading it is not. Count one visit only after this logical
         visit completes; polling and confirmed pre-launch failures do not add
         visits. Enforce each
         agent's `max_visits_per_step`, or the global default, before following its route.
         If the cap has been reached, route to `end`. `end` is terminal and is not
         an agent slug; never try to invoke an `end` agent. Never put the
         visit-cap rule in any sub-agent's prompt.

# Combined with each agent's `prompt_construction` when calling that agent.
# Replace `<next_options>` with that agent's `next:` list, copied verbatim,
# comma-separated; never compose, abbreviate, or invent option names.
base_prompt: |
  You have been given a working (visit) directory; its full path is provided
  below. Read the input artifact(s) at the full path(s) provided below, do your
  task, and write your output artifact into your visit directory.

  A token cache (full project context) and semantic index are available: see
  `docs/SEMANTIC-INDEX.md` in the working directory for the token cache local
  path and the semantic index entrypoint.

  When you have completed your work, write `outcome.yaml` into your visit
  directory containing a single field `next:` set to the slug of the agent that
  should be invoked next, chosen from the options listed for you below.

  Your next options are: <next_options>

agents:

  # `plan` is the entry point of the invocation loop.
  plan:
    model: claude-fable-5-1 # -> Claude Code
    reasoning_effort: high
    context: empty
    reads:
      - <spec_document>
    creates:
      - plan.md
    next:
      - plan-critique
    prompt_construction: |
      Tell the plan agent its visit directory and the full path to the spec
      document. Instruct it to read the spec document, do focused recon of the
      codebase in the working directory if one exists, and produce `plan.md` in
      its visit directory. Instruct it to write the plan as Markdown checklist
      items using `- [ ]` task checkboxes. Its only next option is
      `plan-critique`.

  plan-critique:
    model: gpt-6-astra # -> Codex
    reasoning_effort: high
    context: empty
    reads:
      - plan.md
      - <spec_document>
    creates:
      - plan-critique.md
    next:
      - plan-update
    prompt_construction: |
      Tell the plan-critique agent its visit directory and the full paths to
      `plan.md` and the spec document. Instruct it to read `plan.md` and the
      spec document and produce `plan-critique.md` in its visit directory. Its
      only next option is `plan-update`.

  plan-update:
    model: claude-fable-5-1 # -> Claude Code
    reasoning_effort: high
    context: empty
    reads:
      - <spec_document>
      - plan.md
      - plan-critique.md
    creates:
      - updated-plan.md
    next:
      - coding
    prompt_construction: |
      Tell the plan-update agent its visit directory and the full paths to
      the spec document, `plan.md`, and `plan-critique.md`. Instruct it to read
      those three and produce `updated-plan.md` (NOT `plan-update.md`) in its
      visit directory. Instruct it to preserve the plan as Markdown checklist
      items using only unchecked `- [ ]` task checkboxes. Its only next option is
      `coding`.
      Tell the agent to only include genuine improvements that derisk the project
      and make it more maintainable or well tested.

  coding:
    model: gpt-6-astra # -> Codex
    reasoning_effort: high
    context: continue
    reads:
      - updated-plan.md # latest review visit copy, otherwise plan-update output
      - <spec_document>
      - review.md # reviewer's latest; only when returning from review
    creates:
      - coding-update.md
      - proposed-plan-checks.md
    next:
      - review
    prompt_construction: |
      Tell the coding agent its visit directory and the full paths to
      `updated-plan.md` and the spec document; use the latest `updated-plan.md`
      from a prior review visit when one exists, otherwise use the plan-update
      artifact. When this invocation follows a review, also supply the full path
      to the reviewer's latest `review.md`. Instruct it to choose a coherent
      subset of unchecked plan items for this visit, not necessarily the whole
      plan. It must not edit `updated-plan.md` or check any plan checkbox. It may
      propose completed boxes by writing `proposed-plan-checks.md` in its visit
      directory, listing the exact checklist item text it believes should be
      checked and the evidence for each. Instruct it to produce
      `coding-update.md` in its visit directory, test the software it writes, and
      run any tests that are part of the repo's standard test set. Instruct it to
      commit every change it makes, and to never use `git add` with wildcards or
      `-A` (add specific paths only). Its only next option is `review`.

  review:
    model: claude-fable-5-1 # -> Claude Code
    reasoning_effort: high
    context: continue
    reads:
      - <spec_document>
      - updated-plan.md
      - coding-update.md (ALL prior coding visits, not just the latest)
      - proposed-plan-checks.md # latest coding visit
    creates:
      - review.md
      - updated-plan.md
    next:
      - end
      - coding
    prompt_construction: |
      Tell the review agent its visit directory and the full paths to
      the spec document, the latest authoritative `updated-plan.md`, every prior
      `coding-update.md` from all coding visits so far, and the latest
      `proposed-plan-checks.md`. Instruct it to
      read those and produce both `review.md` and `updated-plan.md` in its visit
      directory. It must copy the input `updated-plan.md` to its visit directory;
      review is the only agent allowed to check plan boxes, and the only edits it
      may make to that copy are changing Markdown checkbox markers from unchecked
      to checked for work it determines is complete. It may accept or reject the
      coding agent's proposed checked boxes. Instruct it to judge the work by
      completeness against the spec document, correctness, and the results of its
      own testing; it must run the software and the repo's standard test set, not
      only inspect code. If the spec document's requirements are not all met, it
      must choose `coding` next so more work can be completed. Its next options
      are `end` or `coding`.
      Also tell it the run directory's timestamp name. When it chooses `end`,
      it must copy its final `updated-plan.md` and `review.md` into
      `docs/plans/<run-timestamp>/` in the working directory and commit them
      with specific-path `git add`.
```
