# Coordinator recommendation: own the Pi port

Tyler leans toward porting and welcomes pushback. My recommendation is an owned semantic port of Pi's headless coding engine, using a pinned TypeScript Pi version as the behavioral reference. This is a proposal for Tyler and the coordinator to decide together, not a decision delegated to researchers and not authorization to begin implementation.

The reason is coherence and ownership. Gimbal needs provider streaming, coding tools, the agent loop, session persistence and forking, compaction, resources, and lifecycle controls to work together. Pi Agent Go supplies a smaller loop with different surrounding contracts; it does not remove the need to implement that coding-harness layer. Following one upstream's behavior gives us a clearer reference than assembling a loop, tools, and session machinery from unrelated projects.

Porting should not mean discarding useful Go code. Inspect existing translations package by package, retain code and tests that actually match the selected Pi behavior, and adapt or rewrite the rest into cohesive internal packages. sky-valley/pi is the strongest translation reference to investigate first because its source includes providers, the agent, coding sessions/tools, and differential fixtures. This is not a claim that the whole project is qualified for adoption. Pi Agent Go and pi-llm-go remain useful secondary references.

The proposed ownership boundary is Gimbal's internal implementation, with explicit per-session state and direct HarnessAdapter integration. Preserve Pi semantics at the provider, tool, history, and lifecycle seams; do not require identical TypeScript class structure, process transport, or incidental implementation choices. Resolve what headless fidelity includes before splitting work. Provider coverage, resource behavior, and TypeScript-extension compatibility must not disappear silently to meet a speculative size or delivery estimate.

First settle shared message/event/tool contracts and lifecycle ownership; then partition the real source responsibilities into independently testable package assignments. The useful concurrency count follows those dependencies. An assertion of exactly fifteen independent tasks is not evidence of achievable parallelism. Integration and real coding qualification remain necessary after individual packages pass.

## Corrections from direct source inspection

- The candidate report says pi-llm-go lacks custom endpoint/header configuration. Its pinned providers/openai/openai.go Options exposes BaseURL, URL, HTTPClient, and Headers (lines 55–86). That stated rejection reason is false.
- The blanket reflection rejection is too broad. pi-agent-go/tool.go Raw accepts a supplied JSON schema and handler (lines 148–158), while Typed uses reflection. A specific reflection-using helper does not prove reflection is unavoidable throughout reuse.
- sky-valley/pi's root go.mod has only golang.org/x/image and golang.org/x/text as external requirements. Its coding package does import its own client/protocol packages, and ai imports telemetry; actual extraction costs require dependency inspection. Repository size alone does not establish the runtime footprint or cost of an owned subset.
- Pi Agent Go's Snapshot shallow-copies Message values in agent.go (lines 420–422), while each Message has a Content slice in pi-llm-go/message.go. Restore likewise copies only the outer message slice. This needs explicit treatment for Gimbal's independent forks; an aggregate green suite does not establish that contract.

All paths above resolve under the pinned source copies in SOURCES.md. The reports' exact size/performance estimates and the claim that a clean rewrite is the only viable path remain unaccepted. My recommendation rests on a coherent behavioral reference and control of the implementation, not those estimates or a maintainer's star count.

## Direct qualification sample

Ran `go test -race ./ai/... ./agent/... ./coding/...` in a separate scratch clone of sky-valley/pi at e6b56e7223cfab707b21a0a19e0a9a2417ac960a, with credential-related environment variables removed. The ai, ai/providers, and agent packages passed. The coding package failed: TestBashCapturesOutputPastExit expected late TICK6 but captured only HEAD and TICK1; TestLoadSkillsAndFormat expected two skills but loaded 34. The original home directory remained visible, so the latter result does not establish a production discovery defect. The bash result needs reproduction and diagnosis before classification. The whole command exited 1. No real model workload or complete TypeScript differential suite was run in this sample.
