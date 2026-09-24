# Smart-home demo: speculative evaluation with LLM fallback

## Purpose

The only demo listed in the assigned official documentation shows a single TypeSafe evaluation covering many possible smart-home interpretations, followed by ordinary code that selects relevant answers and calls an LLM only for generation-shaped work.

## Key concepts

- **Ask speculative questions before knowing their relevance.** For “turn off all lights,” the request includes category, domain, device, and light-action questions; the action question assumes a light command before classification is known. The code filters irrelevant answers afterward. [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- **Avoid sequential decision round trips.** The documented anti-pattern asks category, then domain/device, then device action. The source says minimizing question count this way is slower and more expensive than batching questions in one upfront call. [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- **Pair a decision model with an LLM at generation boundaries.** A Noul detects compound requests; if true, an LLM splits the request into atomic commands, which Jev evaluates individually. General conversation or information requests route to a freeform LLM instead. [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- **The demo catalog is sparse.** The official demos index lists only this smart-home example and explicitly solicits more use cases from users. [demos](https://docs.typesafe.ai/demos.md)

## Citation bookmarks

- Demo catalog scope: [demos](https://docs.typesafe.ai/demos.md)
- Concrete speculative-question explanation: [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- Sequential-call anti-pattern: [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- Compound splitting and conversational fallback: [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- Source-code availability caveat: [smart home](https://docs.typesafe.ai/demos/smart-home.md)

## Themes

- Evaluate broadly once, then narrow in code.
- Escalate only the subproblem that needs text generation.
- Preserve typed deterministic behavior for known request shapes while retaining an LLM escape hatch.

## Gotchas

- The page says full source “will be available on GitHub at release”; the assigned snapshot therefore does not substantiate implementation details beyond the prose and diagram. [smart home](https://docs.typesafe.ai/demos/smart-home.md)
- The claim that sequential calls are slower and more expensive is stated without measurements in this page; treat it as design guidance until measured on the intended question set.
- The demo is about textual user requests. It provides no evidence of audio ingestion, image ingestion, or vision capability.

## Task recipes

- **Continuously scan a Gimble run:** send one compact run-state snapshot with speculative questions such as “is the agent blocked?”, “is it repeating?”, “is the plan drifting?”, and “does this need human authority?”; let Go consume only answers relevant to the current run state. Start at [smart home](https://docs.typesafe.ai/demos/smart-home.md).
- **Keep generation rare:** use a fixed decision to determine whether coaching is needed; invoke a generative supervisor only when the route requires composing a novel message. The architectural analogy is documented at [smart home](https://docs.typesafe.ai/demos/smart-home.md).
