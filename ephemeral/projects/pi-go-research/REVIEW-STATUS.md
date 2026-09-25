# Research completion and review status

All three research-document workflows completed on 2026-09-25 without a terminal error. Their final editorial verdicts report only nitpicks: candidates after three editorial passes; upstream and port-plan after two each. Reports and combined semantic indexes exist for all three. Models were the workflow defaults recorded in README.md.

These are completed research outputs, not an accepted implementation contract. Before selecting code or dispatching implementation, the frontier review must reconcile the candidate report's selective-reuse recommendation with the port plan's blanket rejection of reuse.

Initial coordinator inspection found reasons to distrust the port plan's certainty. Its approximately 4,500-line implementation size, exact assignment count, and claimed memory savings are not established by a qualified implementation. Its performance sources mix empty-process measurements with estimates for a Go harness that has not been built. Do not use those numbers as measured product benefits. A reflection-based typed-tool helper also does not establish that every reuse path requires reflection: pi-agent-go/tool.go exposes Raw alongside Typed.

The original source clones remain the primary evidence. The research reports, source notes, and indexes contain researcher interpretations that still require checking where consequential. No adoption or rewrite decision has been made, and no implementation was started by these runs.
