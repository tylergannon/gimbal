# Speculative fan-out

## Purpose

Speculative fan-out asks all potentially useful questions in one Jev call—even questions relevant only to some branches—and makes ordinary application code select, combine, and ignore the returned answers.

## Key concepts

- **Batch first, filter later.** The official recommendation is to place all system questions in one request because questions are evaluated in parallel and additional questions usually have little response-time impact. [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- **Speculation is intentional.** The support-ticket example asks category, bug severity, reproducibility, refund intent, and frustration together, even though severity/repro matter only to bug reports and refund intent only to billing. [fan out](https://docs.typesafe.ai/patterns/fan-out.md) [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- **Code owns relevance and branching.** The example branches on category, then reads only branch-specific answers; frustration remains useful across branches. [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- **The payoff is avoiding dependent calls.** One response contains the full decision tree’s inputs; irrelevant answers are discarded while relevant speculative answers save a round trip. [fan out](https://docs.typesafe.ai/patterns/fan-out.md)

## Citation bookmarks

- Pattern recommendation and latency claim: [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- Five-question support example: [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- Exact question definitions: [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- Relevance note: [fan out](https://docs.typesafe.ai/patterns/fan-out.md)
- Code-controlled route: [fan out](https://docs.typesafe.ai/patterns/fan-out.md)

## Themes

- Collapse a decision dependency graph into one inference call without collapsing the application’s explicit control flow.
- Global signals can coexist with branch-local signals.
- One state snapshot yields a coherent bundle of simultaneous judgments.

## Gotchas

- “Usually little effect on response time” is not a capacity guarantee; measure latency, token use, accuracy, and any question-interference effects on a realistic supervision bundle.
- A supplied question can yield an answer even when the premise is irrelevant. Never interpret every returned answer; apply a code-owned relevance condition first.
- The support example uses raw score/Noul thresholds but does not show calibration methodology or confidence gating for every branch.

## Task recipes

- **Make a supervision probe:** define a single state snapshot from the recent run transcript and runtime state; ask in parallel about blockedness, repetition, instruction drift, missing evidence, risky side effects, and whether human authority is required; branch only on relevant answers in Go. Start at [fan out](https://docs.typesafe.ai/patterns/fan-out.md).
- **Preserve a global concern:** evaluate a dimension such as “evidence quality” alongside branch-local coaching diagnoses and apply it after the route, like frustration in the support example. Start at [fan out](https://docs.typesafe.ai/patterns/fan-out.md).
- **Validate batching before adopting it:** compare one batched call with a staged baseline on the same labeled run windows; record latency, total cost, answer agreement, and intervention quality rather than accepting the qualitative performance claim.

