# Coordinator synthesis notes

The authoritative planning output is [PLAN.md](PLAN.md), [MODULES.md](MODULES.md) and [WORKFLOW.md](WORKFLOW.md). The three `*-DRAFT.md` files are preserved as non-authoritative inputs and contain proposals and errors that were not accepted.

Drafts completed with Claude Fable 5.1, Codex GPT-6 Astra and agy's configured Gemini default. The coordinator inspected all three and checked source/API details independently. Tyler then pointed out that the research was already done and the planning process was becoming excessive. We let the existing drafts finish and synthesized directly; no additional model critique/research round or mandatory interview was launched. This is a deliberate shortening of the sprint-planning skill, not a claim that its full seven-phase process was completed. Recommended scope choices remain visible for review.

Accepted across drafts: coherent private packages, shared contracts before fan-out, a single upstream runtime as oracle, optional verified donor reuse, independent worktrees, one integration owner, and behavior-based qualification.

Changed or rejected:

- Retained provider families as explicit assignments instead of silently treating a Chat-Completions-only milestone as the full port. Also retained image reads, branch summaries, configuration and auth behavior in the target.
- Rejected new `pigo`/`pinative` model namespaces. Coordinate one replacement with #386's actual landed implementation.
- Rejected the nested range/`Group.Go` sketch in all drafts for the current graph reader. Explicit named branches fit this finite program. Checked `internal/generate/expr.go` and `stmt.go`.
- Rejected helpers hiding worker/review/integration tactics, success based only on `Check` returning nil, and acceptance tags alone as restart proof.
- Rejected claims that non-overlapping package ownership makes conflicts impossible, current-format storage makes imports free, no new dependency could be necessary, or the sampled shell failure's exact cause/regression identity was established.
- Preserved images/replay metadata and provider-specific accounting rather than simplified invented contracts. Contract implementation must settle concrete Go signatures; this plan specifies behavior and ownership without pretending uncompiled draft code is an API.
- Current v3 history interoperability is recommended; historical migration compatibility and unsupported extension replay are not silently promised. Native reopen and Gimbal workflow restart remain distinct.
- Scope trust and file mutation locks explicitly for embedding. Sessions cannot change process cwd/environment, and a session-local lock cannot coordinate two sessions writing the same physical file.
- Removed unsupported performance/size claims and exact coverage counts. Verified all 111 primary and abbreviated source/test paths in the final module table exist locally; existence is not a claim that all their behavior has already been qualified.

The main implementation risk is still lifecycle assembly, followed by provider replay and tool/history fidelity. Package independence reduces editing contention; it does not eliminate shared semantic contracts or integrated validation. No same-day completion promise is made.
