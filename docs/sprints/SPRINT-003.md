# Sprint 003: Promises from speech on current main

## Pyramid Index

- **L0:** Selectively port the useful Promises feature onto current main and see real recorded speech populate a project's persisted ledger.
- **L1:** Preserve today's application while adding both recording entry paths; reuse the latest small Go worker and ledger UI; demonstrate fields, tags, sources and a clear withdrawal; stop when the ordinary microphone path works.
- **L2:** [Architecture](#architecture) pins sources and integration seams; [Implementation Plan](#implementation-plan) orders the port; [Definition of Done](#definition-of-done) maps the request to observations and states the microphone stop condition.

## Overview

Start from current main, not the old feature branch. At planning time main and this checkout are `3ea6ecb6e1626bdea1b07928265d5e3042efefc3`; the latest useful feature is `origin/promises-ledger` at `8f101e94ddc49096d41c7f67b9983ba43902c9db`, common base `a8fbba22`. Recheck those identities before implementation. The preserved readable feature tree is `/Users/tyler/src/promises-research/takeover-2026-10-13/sources/branch-current/`; source objects and that snapshot provide local input, so executor prompts need no remote issue or branch URL. The takeover's `MAIN-PORT.md` used older main `9256fd95`; its migration advice remains useful, but current source wins.

This document is the executor's authoritative consolidated plan. Requirement precedence is Tyler's latest clarification, then journal `CORRECTIONS.md` and `REQUEST.md`, then accepted `LEAN-DELIVERY.md` / migration assessment, then historical designs and reviews. Reviewer preferences are not requirements. The [journal source index](/Users/tyler/src/promises-research/takeover-2026-10-13/README.md) preserves originals; raw reviews and competing drafts are optional provenance, not mandatory executor reading.

The feature is largely written. Port its accepted final source delta by hand into current main, preserving useful prompt/tag/UI corrections. Do not replay its development history, bring its old generated output, or reconstruct a supposedly complete first-pass tree. Requirements are settled. No chapter is selected.

## Use Cases

1. Start a requirements discussion using a new Start Requirements Discussion action in the current native Recording menu, speak commitments, and see promises accumulate in that project's Promises tab.
2. Make an ordinary recording; its speech contributes nothing until the user marks it as a requirements discussion in its transcript view.
3. Read a promise's title, details and tags, filter by a tag and use a meaningful tag sort, open its source recording and read its truthful quote.
4. Clearly withdraw a spoken commitment and see the ledger's withdrawn result; use the existing manual Withdraw/Restore controls where useful. Reopen the project and retain entries and processing progress.

## Architecture

Reuse the feature's per-project Go `internal/promises.Store`, project-owned worker, finalized transcript input, text-only adapter extraction and JSON ledger/cursors. Keep ordinary atomic writing, copy/save/publish ordering, small required-field/target validation, text-only tool and hook exclusion, finite model attempts and cancel-before-join. These are small existing safeguards, not a new runtime framework. Do not replace useful code merely to shorten its diff.

| Source revision / surface | Migration into current main |
| --- | --- |
| `8f101e94`: `internal/promises/promises.go` and focused package tests | Bring the latest real core, including prompt and tag-preservation corrections; keep optional grounded quotes and withdrawal behavior. |
| `8f101e94`: `internal/transcript/{store,durable}.go`, native controller/commands/controls, transcript remote | Add the requirements flag and mark-existing action to the existing recording pipeline. Preserve finalized/acknowledged storage and ordinary false default. |
| `8f101e94`: `claude/claude.go` TextOnly changes and tests | Apply the capability delta; retain main's adapter behavior and current tests, including Sprint 002 coverage. Main's production file has no base-side changes at planning time; its tests do. |
| `8f101e94`: `internal/host/host.go` | Add ledger construction/access and worker cancellation/join into current admission/close; retain main's saved project order. |
| `8f101e94`: native `Application.swift` menu delta | Add requirements action to today's Start/Pause/Resume/Stop Recording menu. Keep current shortcuts and Show Today's Transcript. Do not copy the old menu wholesale. |
| `8f101e94`: `PromiseLedger.svelte`, promises route, `RecordingTranscript.svelte`, remote additions | Integrate into today's `ProjectWorkspace.svelte`: remembered tabs, new sessions, live chat control, native project/menu state and responsive sidebar remain intact. Keep current project-reorder remotes. |
| Main's generators and dependencies | Regenerate all derived bindings/types using current main; do not port generated Go/Swift/TS, lockfiles, module versions, Justfile, hooks or release infrastructure from the feature branch. |

Semantic prior art was routed through `/Users/tyler/src/promises-research/indexes/gimbal/README.md` → `routes/ledger-llm-process/README.md` → `leaves/go-transcript-host/transcript-durable-store.md`, and the Clarity index → `routes/ledger-mutations/README.md` → `leaves/go-worker-pipeline/semantic-review-apply-and-early-capture.md`. Targeted source search and Clarity's `server/internal/worker/apply_review.go` confirm the relevant seams. The cache is `/Users/tyler/src/promises-research/token-cache`; it is historical context, not current Gimbal authority. Clarity supplies extraction/mutation/source inspiration; its SQL families, immutable versions, conflict interviews and publication pipeline are not this sprint. Clarity has no tags: tags are Gimbal's explicit requested behavior.

## Implementation Plan

1. **Port the recording/core seams.** Compare final feature source to the common base; apply only the accepted feature into main. Bring focused tests alongside flag/mark, ledger consumption/persistence, capabilities and owner lifecycle. Do not overwrite unrelated tests or shared code.
2. **Integrate the native and project UI.** Add both entry paths and the Promises tab/remotes. Retain the little history pager if it still fits; preserve worker traversal independently. Keep truthful source links/quotes, withdrawals and usable filter/sort. Replace the numeric “10–30 seconds” claim with honest waiting copy and keep failures compact. Avoid quote-count/no-quote clutter and retrying/parked product screens.
3. **Generate and check the candidate.** Prepare the actual native/web candidate once with current `just app` (includes `just build`, `go generate ./...`, Vite+ build and native generation), then reuse its outputs for `go test -count=1 ./internal/promises ./internal/transcript ./internal/host ./claude ./web`, `bash scripts/check-recorder.sh`, `just check-web`, `just vet`, and `cd web && vp run test`; run broader affected Go checks required by current repository/CI. Inspect native generation effects for unrelated churn before native observation. Use Svelte autofixer on written Svelte when connected. Fix integration failures at their actual seams. Inspect the diff for accidental old infrastructure and current-feature regressions.
4. **Run the candidate application and finish.** Use real native menus and microphone, a disposable project/catalog, and Claude Haiku for extraction (set `GIMBAL_PROMISES_MODEL=haiku` in the candidate app environment and confirm the actual resolved model). Keep the existing product model default; no model-settings UI is added. Observe the matrix below in one ordinary session; report candidate SHA, resolved model and what was seen in chat/PR. Preserve the installed personal catalog. No proof program, run dump, screenshot or result file is committed.

## Files Summary

Handwritten changes belong in `internal/promises/`, `internal/transcript/`, `internal/host/`, `claude/`, the existing native recorder/menu files, `web/src/lib/{transcript,workspace}.remote.go`, workspace components and `web/src/routes/projects/[project]/promises/`. Derived bindings/types change only through current generation. Tests live beside the changed packages/components. Historical plans, review rounds, stubs, evaluation output and worklogs stay on the reference branch/journal.

## Definition of Done

| Request | Existing source to port | Migration concern | Observation that closes it |
| --- | --- | --- | --- |
| New requirements recording | Requirements flag, native controller/menu and open remote | Today's menu labels/shortcuts | Start through native requirements action; real spoken commitments appear live in that project's Promises tab. |
| Mark an existing recording; ordinary excluded | `MarkRequirements`, transcript command/button | Current transcript/remote behavior | Record a distinct ordinary commitment; ledger stays unchanged; mark that same recording and see its commitment enter. |
| Title, details, tags, filter and sort | `Entry`, latest prompt/apply, `PromiseLedger` | Current workspace routing/restored tabs | Inspect populated fields; select tag filter and tag sort with multiple tagged entries; visible results match the controls. |
| Basic source audit and withdrawal | `Evidence`, grounded `cite`, source links, model operations/manual action | Links use project paths; no unsupported chunk anchors | Open one source recording and compare its quote; speak one clear retraction and see the correct withdrawn result. |
| Per-project persistence | JSON ledger/cursors and owner wiring | Preserve project order and current shutdown | Reopen candidate/project; prior entries and withdrawal remain without duplicated consumed speech; project identity remains correct. |
| Preserve today's app | Current menus, workspace, host, adapter/tests | Thirteen historical overlap paths are not blanket replacements | During the same session inspect current menu shortcuts/today transcript, remembered tabs, project order and ordinary live chat operation where touched. |

Focused existing regressions support these observations; green tests alone do not prove speech delivery. No exact latency, perfect deduplication, exhaustive ambiguous withdrawals, chronology reconstruction, older-history campaign, fsync failure-injection campaign or conflict-resolution parity gates the sprint.

If native GUI/microphone access is unavailable, state precisely which observation is blocked and stop acceptance work there. Synthetic transcripts cannot close that claim. Fix a concrete observed feature failure and repeat that observation; do not turn a reviewer preference into another requirement. Stop once both entry paths, exclusion, usable ledger/source/withdrawal and persistence are seen working, then follow the repository's normal validation and merge rules.

## Risks & Mitigations

- **Stale assessment main:** use current checked-out main and re-evaluate overlapping source; the pinned journal is a guide, not a replacement tree.
- **Old generated code masks integration errors:** regenerate through today's toolchain and build the actual native/web candidate.
- **Model output or ambiguous speech:** retain observed prompt fixes, basic validation and bounded attempts; demonstrate straightforward commitments/retraction rather than promise perfect interpretation.
- **Native acceptance remains unproven:** prioritize the real menu/microphone session after build; report a specific blocker instead of accumulating substitute proof.

## Evidence boundary and retain/cut decisions

Historical branch package tests and supplied-text HTTP/native fixture runs support existing core behavior; the latter used Haiku. They do not establish real menu/microphone acceptance or current-main compatibility. No port, candidate build or real speech observation has happened in this planning task. All nine historical plan reviews were Sol5.6 medium, with 21 numbered findings; no independent core/native code review was recovered. These facts prevent overstating old evidence; they do not launch another review campaign.

Retain latest core, both entry paths, tag/filter/sort, basic sources/withdrawals, grounded optional quotes, small pager and existing bounded/input/persistence/owner safeguards. Cut numeric timing claims, split empty-state explanations, collapsed no-quote clutter and separate elaborate error/story design obligations. Defer exact audit coordinates, immutable versions, conflict interviews, catalog/badge integration, exhaustive ambiguity/parity and recovery campaigns. First core `6933ee02` already contained most functionality; final core grew 526 → 536 lines with small prompt/tag fixes. Reverting to an assembled first pass would discard useful corrections, not erase prior review costs.

## Dependencies

Current main's Go/skgo/polytype/frontend/native toolchain; local journal at `/Users/tyler/src/promises-research/takeover-2026-10-13/`; pinned feature source; working native microphone/transcription access and Claude Haiku. Read the current AGENTS, web idiom rules and build/release instructions before implementation.

## Open Questions

Feature scope is settled and prior user answers are applied. Final plan approval is pending; no new interview or approval is claimed. At execution, resolve only concrete integration choices and environment blockers from current source and the actual run. The design cards remain reference material, not approved additional product scope.
