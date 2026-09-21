# Smart-home demo: speculative evaluation with LLM fallback

## Purpose

The only demo listed in the assigned official documentation shows a single TypeSafe evaluation covering many possible smart-home interpretations, followed by ordinary code that selects relevant answers and calls an LLM only for generation-shaped work.

## Key concepts

- **Ask speculative questions before knowing their relevance.** For “turn off all lights,” the request includes category, domain, device, and light-action questions; the action question assumes a light command before classification is known. The code filters irrelevant answers afterward. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:15-32`
- **Avoid sequential decision round trips.** The documented anti-pattern asks category, then domain/device, then device action. The source says minimizing question count this way is slower and more expensive than batching questions in one upfront call. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:34-49`
- **Pair a decision model with an LLM at generation boundaries.** A Noul detects compound requests; if true, an LLM splits the request into atomic commands, which Jev evaluates individually. General conversation or information requests route to a freeform LLM instead. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:51-57`
- **The demo catalog is sparse.** The official demos index lists only this smart-home example and explicitly solicits more use cases from users. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos.md:5-15`

## Citation bookmarks

- Demo catalog scope: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos.md:5-15`
- Concrete speculative-question explanation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:15-32`
- Sequential-call anti-pattern: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:34-49`
- Compound splitting and conversational fallback: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:51-57`
- Source-code availability caveat: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:59-61`

## Themes

- Evaluate broadly once, then narrow in code.
- Escalate only the subproblem that needs text generation.
- Preserve typed deterministic behavior for known request shapes while retaining an LLM escape hatch.

## Gotchas

- The page says full source “will be available on GitHub at release”; the assigned snapshot therefore does not substantiate implementation details beyond the prose and diagram. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:59-61`
- The claim that sequential calls are slower and more expensive is stated without measurements in this page; treat it as design guidance until measured on the intended question set.
- The demo is about textual user requests. It provides no evidence of audio ingestion, image ingestion, or vision capability.

## Task recipes

- **Continuously scan a Gimble run:** send one compact run-state snapshot with speculative questions such as “is the agent blocked?”, “is it repeating?”, “is the plan drifting?”, and “does this need human authority?”; let Go consume only answers relevant to the current run state. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:15-49`.
- **Keep generation rare:** use a fixed decision to determine whether coaching is needed; invoke a generative supervisor only when the route requires composing a novel message. The architectural analogy is documented at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/demos/smart-home.md:51-57`.
