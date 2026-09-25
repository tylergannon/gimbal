# Owned Pi port: planning intent

## Seed and settled direction

Tyler: "yeah get that plan ready eh. I could see writing a special gimbal workflow just so that we can get that fan-out to all these different implementations, and it ought to work fine as long as you plan separate internal Go packages for each of the typescript modules."

Produce a practical plan for an owned semantic port of Pi's headless coding engine inside Gimbal. This turn is planning only, including the design of a dedicated Gimbal fan-out workflow. Do not implement or launch that workflow or the port. Tyler and the coordinator decide; drafts and critiques are input to the coordinator's synthesis.

## Orientation

- Worktree: /Users/tyler/src/gimbal-pi-research. Existing Gimbal API is authoritative: harness.go, session.go, events.go, usage.go, `go doc`, and internal/workflows examples. Preserve current public APIs. No new exported root-Gimbal API or high-level orchestration wrapper.
- Source: /Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi at d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31. Inspect actual imports and tests. That tree has both established coding-agent sessions and newer agent/harness modules; do not accidentally combine two alternative runtime stacks or treat re-export facades as independent implementations.
- Existing Go sources: /Users/tyler/.local/share/gimbal-pi-research/sources. SOURCES.md in this planning directory's parent gives all pins. Especially inspect sky-valley--pi as a translation donor, not a trusted specification.
- Existing reports/indexes contain known factual errors. Read RECOMMENDATION.md and REVIEW-STATUS.md in the parent directory first. Use indexes to route to original code. Do not inherit estimates, blanket reuse rejections, or source-parity claims. Research bug #389 is separate from this port.
- Earlier sprints docs/sprints/SPRINT-001.md and SPRINT-002.md concern usage and Claude lifecycle, not this port. No chapter applies. Keep this plan and its drafts under ephemeral/projects/pi-go-research/planning; do not edit docs or the repository sprint ledger during this planning task.

## Semantic prior art

Token cache: /Users/tyler/.local/share/gimbal-pi-research/sources.
Routing entrypoints: /Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/{upstream,candidates,port-plan}/corpus/INDEX.md. Follow at least one relevant route, then check its original code/test citations. The upstream topic-003 route points to loop/steering behavior. Prior source checks confirmed pi-llm-go custom URL/header options, Pi Agent Go's Raw tool path, and its shallow snapshot copies. Sky-valley's ai, ai/providers, and agent race tests passed; coding tests failed for late shell output and ambient-home skill counts. No Go candidate has been qualified as a complete Gimbal harness.

## Desired result

A future implementation run should start native coding sessions within Gimbal's process, read/edit files, run shell commands, stream activity, continue conversations, steer/cancel, fork independently, persist/resume, compact history, and return schema-conforming output through Gimbal's existing validation. Tool subprocesses remain normal. First live milestone targets pi/diffusion/deepseek-4.1-flash through the existing Router model route; do not duplicate the in-progress #386 RPC adapter's namespace or assume it has landed. Plan an explicit cutover owner after rebasing onto its actual result. Preserve other harnesses.

The initial scope proposed in the conversation is headless Pi, excluding terminal UI and execution of TypeScript extensions. Provider breadth, native Pi session import compatibility, and other consequential scope choices should be explicit recommendations rather than silently dropped. An initial Router vertical slice is a milestone, not proof of full headless parity.

## What the plan must settle

Map actual TypeScript modules to separate internal Go package responsibilities with exclusive file ownership. A module may be several tightly related TypeScript files; avoid translating every helper into a new Go package or creating import cycles. Show all shared-contract ownership and classify source surfaces as ported, reused/adapted, provided by Gimbal, or deliberately deferred. Give exact upstream paths and relevant test references, not just prose labels.

Identify the dependency contracts that must settle before fan-out. Define the message/event/tool/provider, history/compaction, cancellation/steering and adapter boundaries sufficiently that independent workers can compile and test against real contracts. Show wave dependencies, actual achievable concurrency, integration ownership, and qualification responsibilities. Aim at roughly 10–20 sensible assignments; correctness decides the count. No line-count or same-day guarantee.

Sketch a special Gimbal workflow using existing Group, Iterate, Generate, Check/RunCommand, and ordinary Go. Use constant call-site names and correct Group cancellation/join behavior. Separate worktrees per worker; serialize integration against a known baseline. Prevent workers from owning shared go.mod/go.sum, contracts, generated files, or the same integration file. Explain source/handoff locality, scope coaching, independent checks/review, bounded revision, actual failure propagation, and restart of a partially completed run without inventing a generic DAG engine or workflow API. Use inexpensive implementation models when suitable; frontier judgment owns uncertain interfaces and final acceptance. Do not make the port require its own unfinished harness to build itself.

Prove individual modules with relevant translated upstream tests and direct differential comparisons, then prove real integrated coding behavior. Specify what observations establish completion. Keep runnable fixtures/tests beside production packages; no committed proof programs or run output. Preserve primary source license notices for copied/translated material. Do not make undocumented Go-donor behavior the oracle.

## Draft deliverable

Write a source-backed draft of approximately 2,000–3,500 words to your assigned file only. Include overview/use cases, scope, package/assignment table, shared contracts, dependency waves, the dedicated workflow sketch, proof, risks, and meaningful open decisions. You are not alone in this worktree: do not revert others' changes. Do not commit, update ledgers, modify implementation, or launch subagents. Logs and temporary tooling belong outside the worktree. The coordinator owns synthesis and Git.
