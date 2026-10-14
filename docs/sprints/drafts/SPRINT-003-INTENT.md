# Sprint 003 Intent: Port the working Promises ledger to current main

## Seed

Consolidate settled Promises requirements, existing implementation, debrief and migration into one light, cross-checked plan ready to execute. Use Sol6.1 and Opus5.5; Gemini only if a material divergence requires it. Planning only; implementation and final approval remain pending.

## Context

- Current main: `3ea6ecb6e1626bdea1b07928265d5e3042efefc3` (2026-10-14 fetch). Feature source: `8f101e94ddc49096d41c7f67b9983ba43902c9db`, common base `a8fbba22`.
- Most functionality already exists. Selectively port latest good source; retain small observed prompt/tag/UI corrections. Do not replay branch history, stubs, old generators or review campaigns.
- Source traceability and ordinary withdrawal are accepted Clarity functionality. Tags/filter/sort are explicit user requirements, despite no first-class tags in Clarity.
- Historical Haiku fixtures establish downstream supplied-text behavior; real native menu plus microphone remains unproven.
- Preserve current menus/shortcuts, remembered tabs, project ordering and live chat; generate against current skgo/polytype and Vite+ pipeline.

## Pyramid Index

- L0: Deliver the existing per-project spoken Promises feature on current main and prove both real recording paths.
- L1: Reuse small core; adapt current shell/native integration; regenerate bindings; demonstrate ordinary speech and persistence.
- L2: See Context, Constraints and Success Criteria below; final sprint centralizes request/source/migration/proof mapping.

## Semantic Index

Available: token cache `/Users/tyler/src/promises-research/token-cache`; entrypoints `/Users/tyler/src/promises-research/indexes/gimbal/README.md` and `/Users/tyler/src/promises-research/indexes/clarity/README.md`. Follow task routes and relevant leaves, then targeted `rg`; do not scan all tokens. Snapshot Gimbal `a8fbba22`, Clarity `9dfa977c`, index built 2026-10-12: current repository and latest feature source override stale anchors. Retrieved routes expose mark/start, transcript store/remotes, host ownership and tab placement; Clarity source organization has no tags.

## Chapter Context

No chapter link selected. No applicable local chapter ledger exists.

## Recent Sprint Context

001 remains planned token usage by scope;002 completed Claude native-completion waiting. These are context, not work to absorb into this sprint. Next actual file number is003; helper IDs use four digits.

## Relevant Codebase Areas

`internal/promises`, `internal/transcript`, `internal/host`, `native/Sources`, `web/src/lib/workspace`, transcript/workspace/promises remotes, generated bindings. Exact source/adaptation map belongs in final sprint.

## Constraints

User settled scope and correction precedence override earlier drafts and reviewer suggestions. Keep existing lightweight persistence, finalized-input, text-only and cancel/join safeguards; no new recovery/runtime framework, API names, perfect parity, numeric latency promises or proof programs. One neutral empty explanation, optional expanded source context, compact truthful failure message; preserve small existing pager. Real proof uses Haiku and disposable candidate data, never normal installed data.

## Success Criteria

Real speech through native requirements-recording action accumulates entries while recording; ordinary recording saves without ledger changes until marked; marking then adds its commitment; title/details/tags/filter/sort, source link/quote, clear withdrawal and reopen persistence work in the same project. Named main regressions stay intact. Stop at this material outcome; explain exact GUI/mic access block if unavailable.

## Open Questions

None in requirements. A real GUI/microphone/speech permission session is an execution prerequisite, not a new scope question. Existing user answers are applied; no fresh interview claimed.

## Local Sources

Journal `/Users/tyler/src/promises-research/takeover-2026-10-13`, especially CORRECTIONS.md → REQUEST.md → LEAN-DELIVERY.md → MAIN-PORT.md for precedence. Full raw reviews remain optional provenance, not executor-required reading.
