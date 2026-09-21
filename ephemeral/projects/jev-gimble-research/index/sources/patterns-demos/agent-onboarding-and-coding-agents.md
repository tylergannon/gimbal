# Agent onboarding and coding-agent use

## Purpose

This leaf captures the official boundary between Jev and coding agents, the supported onboarding paths, and the smallest documented API/SDK integration. Jev is a typed decision service used *by code an agent writes*; it is not a conversational model or a replacement for the model driving Codex, Claude Code, Cursor, or similar tools.

## Key concepts

- **Keep the coding agent and Jev in separate roles.** Jev consumes state plus typed questions and returns a Choice, Score, or Noul answer; it does not stream prose, edit files, call tools, or converse. The coding agent remains the code-producing actor, while the resulting program invokes Jev for bounded decisions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:9-19`
- **Use Jev where a program needs a fixed decision surface.** The official candidates are fixed-destination routing, rubric scoring, predicate checks over messages/documents/records, and replacing fragile “return JSON” prompts with values typed by construction. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:32-41`
- **The agent skill teaches integrations, not runtime behavior.** It supplies agents with API primitives, patterns, and evaluation-structuring guidance. Installation is via the TypeSafe Claude plugin, `npx skills add`, or a manual copy of the complete skill directory and references. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:7-10` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:13-42`
- **Recommended discovery starts with the codebase.** The documented prompts ask an agent to find fragile parsing or complex judgment that Jev could replace, optionally run cheap API experiments, or compare project needs with published cookbooks. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:57-81`
- **Human review should concentrate on the decision contract.** TypeSafe advises colocating questions and thresholds, reviewing the plan before implementation, editing agent-written questions collaboratively, and validating assumptions rather than trusting assertions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:83-88` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:104-106`
- **One request can mix all three primitive types.** The quickstart sends the source state and named Choice, Score, and Noul questions together; the response returns named typed answers, probability distributions/confidence where defined, and token usage. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:63-94` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:96-136`
- **Integration surfaces are ordinary HTTP, Python, and JavaScript/TypeScript.** The HTTP endpoint is `POST /v1/systemone`; the Python client reads `TYPESAFE_API_KEY` and defaults to `jev-latest`; official SDKs provide typed questions/answers and automatic retries under their default policy. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:31-60` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:141-190` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk.md:5-21`

## Citation bookmarks

- Jev-versus-coding-agent boundary: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:9-30`
- Candidate embedded-agent uses: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:32-41`
- Agent skill installation and update mechanics: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:11-55`
- Agent experimentation and review guidance: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:57-110`
- Canonical HTTP request and typed response: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:31-139`
- Canonical Python SDK example: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:141-192`

## Themes

- Narrow machine-readable decisions complement, rather than replace, generative agents.
- The application retains orchestration, filtering, thresholds, escalation, and side effects.
- Questions and constants form a small, reviewable policy surface.
- Cheap experiments should precede broad integration claims.

## Gotchas

- There is no `model: "jev-latest"` setting that turns a coding agent into a Jev-backed coding agent; that identifier belongs in a TypeSafe API request. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:19-30`
- Installing the same agent skill through multiple methods can produce duplicate copies; choose one method. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:40-42`
- A stale skill can cause an agent to invent request/response fields; update the skill and retry. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:108-110`
- The docs warn against universal confidence thresholds: if only the best option matters, use the maximum probability; if a statistical algorithm requires probabilities, do not substitute confidence. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:96-102`
- The SDK landing page lists Python and JavaScript/TypeScript, not Go. A Gimble integration would therefore begin with the language-neutral HTTP API unless a separate Go client is found. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk.md:9-21`
- These assigned pages do not document image input or vision support. Their concrete state examples are text; do not infer multimodality from JSON-structured questions or from the existence of image assets in the docs.

## Task recipes

- **Find a first Gimble integration:** inspect one existing supervisor decision that currently asks a generative agent for a fixed label, scalar rubric, or boolean; define that decision as Choice/Score/Noul; keep the resulting branch and side effect in Go. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/coding-agents.md:32-41` and validate a representative sample in the Playground at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:9-29`.
- **Make the decision surface reviewable:** put question wording and risk thresholds together, add representative evaluation cases, and have the workflow code own every route. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/agent-skill.md:83-106`.
- **Prototype without an SDK dependency:** POST state, model, and named questions to `/v1/systemone`, then branch on named answer fields. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/introduction/quickstart.md:31-139`.
