# Upstream Pi behavioral map

Read /Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/BRIEF.md first.

Produce a source-grounded map of the upstream TypeScript Pi implementation for a faithful native Go coding harness. Inspect the pinned local upstream deeply. Identify the current version and differences relevant to the 0.87.1 RPC adapter described in issue-386.md. Map pi-ai, pi-agent-core, and the headless coding-agent path, tracing complete executions across layers instead of listing filenames. Identify which newer upstream packages are truly required by this path.

Explain message/content/tool/event contracts, provider wire formats and streaming assembly, retry/auth-refresh behavior, usage and cache accounting, agent loop termination, tool scheduling, steering/follow-up/abort ordering, errors and idle settlement. Trace session storage, branching/clone/fork/resume, compaction and context overflow recovery. Cover the exact coding tools and their edge cases, prompts, project instructions, skills, model/config discovery, authentication boundaries, and extension participation. Show at least concrete success, tool failure, mid-turn cancellation, queued steering, compaction/resume, and fork traces with references to implementation and tests.

Separate core semantics needed for equivalent headless coding behavior from terminal UI and TypeScript extension compatibility. Record exactly what would be lost if those were omitted; do not silently redefine full parity as a toy loop. Explain dependencies on process-global state and how ownership differs in an embedded Go runtime. Identify useful upstream fixtures/tests and what they prove; identify missing or weak coverage. No invented compatibility promises.

Deliver a behavioral dependency map, evidence-rich semantic index, core versus optional scope table, high-risk seams, and a list of decisive qualification scenarios that a Go implementation must pass. A future frontier planner must be able to use this report to divide work without rereading the entire upstream repository. Keep planning proposals provisional; this report establishes source behavior rather than selecting our architecture.
