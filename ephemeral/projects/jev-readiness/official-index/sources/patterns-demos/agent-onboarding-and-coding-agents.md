# Agent onboarding and coding-agent use

## Purpose

This leaf captures the official boundary between Jev and coding agents, the supported onboarding paths, and the smallest documented API/SDK integration. Jev is a typed decision service used *by code an agent writes*; it is not a conversational model or a replacement for the model driving Codex, Claude Code, Cursor, or similar tools.

## Key concepts

- **Keep the coding agent and Jev in separate roles.** Jev consumes state plus typed questions and returns a Choice, Score, or Noul answer; it does not stream prose, edit files, call tools, or converse. The coding agent remains the code-producing actor, while the resulting program invokes Jev for bounded decisions. [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md)
- **Use Jev where a program needs a fixed decision surface.** The official candidates are fixed-destination routing, rubric scoring, predicate checks over messages/documents/records, and replacing fragile “return JSON” prompts with values typed by construction. [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md)
- **The agent skill teaches integrations, not runtime behavior.** It supplies agents with API primitives, patterns, and evaluation-structuring guidance. Installation is via the TypeSafe Claude plugin, `npx skills add`, or a manual copy of the complete skill directory and references. [agent skill](https://docs.typesafe.ai/agent-skill.md) [agent skill](https://docs.typesafe.ai/agent-skill.md)
- **Recommended discovery starts with the codebase.** The documented prompts ask an agent to find fragile parsing or complex judgment that Jev could replace, optionally run cheap API experiments, or compare project needs with published cookbooks. [agent skill](https://docs.typesafe.ai/agent-skill.md)
- **Human review should concentrate on the decision contract.** TypeSafe advises colocating questions and thresholds, reviewing the plan before implementation, editing agent-written questions collaboratively, and validating assumptions rather than trusting assertions. [agent skill](https://docs.typesafe.ai/agent-skill.md) [agent skill](https://docs.typesafe.ai/agent-skill.md)
- **One request can mix all three primitive types.** The quickstart sends the source state and named Choice, Score, and Noul questions together; the response returns named typed answers, probability distributions/confidence where defined, and token usage. [quickstart](https://docs.typesafe.ai/introduction/quickstart.md) [quickstart](https://docs.typesafe.ai/introduction/quickstart.md)
- **Integration surfaces are ordinary HTTP, Python, and JavaScript/TypeScript.** The HTTP endpoint is `POST /v1/systemone`; the Python client reads `TYPESAFE_API_KEY` and defaults to `jev-latest`; official SDKs provide typed questions/answers and automatic retries under their default policy. [quickstart](https://docs.typesafe.ai/introduction/quickstart.md) [quickstart](https://docs.typesafe.ai/introduction/quickstart.md) [sdk](https://docs.typesafe.ai/sdk.md)

## Citation bookmarks

- Jev-versus-coding-agent boundary: [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md)
- Candidate embedded-agent uses: [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md)
- Agent skill installation and update mechanics: [agent skill](https://docs.typesafe.ai/agent-skill.md)
- Agent experimentation and review guidance: [agent skill](https://docs.typesafe.ai/agent-skill.md)
- Canonical HTTP request and typed response: [quickstart](https://docs.typesafe.ai/introduction/quickstart.md)
- Canonical Python SDK example: [quickstart](https://docs.typesafe.ai/introduction/quickstart.md)

## Themes

- Narrow machine-readable decisions complement, rather than replace, generative agents.
- The application retains orchestration, filtering, thresholds, escalation, and side effects.
- Questions and constants form a small, reviewable policy surface.
- Cheap experiments should precede broad integration claims.

## Gotchas

- There is no `model: "jev-latest"` setting that turns a coding agent into a Jev-backed coding agent; that identifier belongs in a TypeSafe API request. [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md)
- Installing the same agent skill through multiple methods can produce duplicate copies; choose one method. [agent skill](https://docs.typesafe.ai/agent-skill.md)
- A stale skill can cause an agent to invent request/response fields; update the skill and retry. [agent skill](https://docs.typesafe.ai/agent-skill.md)
- The docs warn against universal confidence thresholds: if only the best option matters, use the maximum probability; if a statistical algorithm requires probabilities, do not substitute confidence. [agent skill](https://docs.typesafe.ai/agent-skill.md)
- The SDK landing page lists Python and JavaScript/TypeScript, not Go. A Gimble integration would therefore begin with the language-neutral HTTP API unless a separate Go client is found. [sdk](https://docs.typesafe.ai/sdk.md)
- These assigned pages do not document image input or vision support. Their concrete state examples are text; do not infer multimodality from JSON-structured questions or from the existence of image assets in the docs.

## Task recipes

- **Find a first Gimble integration:** inspect one existing supervisor decision that currently asks a generative agent for a fixed label, scalar rubric, or boolean; define that decision as Choice/Score/Noul; keep the resulting branch and side effect in Go. Start at [coding agents](https://docs.typesafe.ai/introduction/coding-agents.md) and validate a representative sample in the Playground at [quickstart](https://docs.typesafe.ai/introduction/quickstart.md).
- **Make the decision surface reviewable:** put question wording and risk thresholds together, add representative evaluation cases, and have the workflow code own every route. Start at [agent skill](https://docs.typesafe.ai/agent-skill.md).
- **Prototype without an SDK dependency:** POST state, model, and named questions to `/v1/systemone`, then branch on named answer fields. Start at [quickstart](https://docs.typesafe.ai/introduction/quickstart.md).
