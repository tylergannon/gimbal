# Sprint 003: Promises ledger on current main (Opus 5.5 draft)

## Pyramid Index

- **Goal.** In a selected project, a person starts a requirements discussion or marks an existing recording as one. While the recording is open, their finalized speech builds up a persistent Go ledger of promises in a Promises tab. Each promise has a title, details and tags, and the tab can filter and sort by tag. Ordinary recordings are ignored until marked. Each promise shows its source quote and recording link, a clear withdrawal works, and everything survives a restart.
- **Method.** Start a fresh branch from main `3ea6ecb6`. Reuse the feature's latest good source from `promises-ledger@8f101e94` file by file, and fit it into today's menus, workspace, host and generators. Do not replay history, and do not use old generated output or the old stub.
- **Proof.** One real app session from this candidate build, in a disposable instance directory, using the real menu, the real microphone and Claude Haiku. The tester reports what they saw in chat or the PR. Nothing from the proof is committed.
- **Stop.** End when every row of the Definition of Done has been seen. If real GUI or microphone access is unavailable, report that one unmet claim and stop. Do not substitute a synthetic driver.

## Overview

The feature already exists in near-complete form on `promises-ledger@8f101e94`:
- `internal/promises/promises.go`: 536 lines.
- `PromiseLedger.svelte`: 201 lines.
- Small additions to transcript, host, remote and native code.

Current main has moved on in several places:
- New menu names and shortcuts (Start/Pause/Resume/Stop Recording, plus Show Today's Transcript).
- Remembered window tabs, New session, live chat state, project ordering.
- A new skgo/polytype generation pipeline and Vite+.

`claude/claude.go`, `internal/transcript/*`, `RecorderController.swift` and `NativeCommands.swift` have not changed on main since the shared base `a8fbba22`. The core can be lifted almost verbatim. The integration files have to be adapted by hand.

Scope comes from the settled request and the journal. Source audit and withdrawal are ordinary Clarity functionality and are in scope. Reviewer findings are not requirements.

## Use Cases

1. **New discussion.** Recording ▸ **Start Requirements Discussion**. The person speaks two commitments and pauses. Promises appear in the project's Promises tab while the recording is still open. Stop works normally.
2. **Mark existing.** The person opens an ordinary recording and clicks **Treat as requirements discussion**. Its commitments enter the same ledger.
3. **Ordinary stays ordinary.** Plain Start Recording saves a transcript and leaves the ledger unchanged.
4. **Browse.** The person expands a promise to see its title, details, tags and source quote, then follows the link to the recording. They filter by one tag and use Tag sort.
5. **Withdraw.** Saying "withdraw promise N", or clicking Withdraw, moves that promise to Withdrawn. Restore brings it back.
6. **Persist.** After quitting and reopening the app and project, the same entries are there and nothing is reprocessed.

## Architecture

```
native menu ─ record(requirements:) ─ NativeCommands.open ─ openSession{requirements}
web transcript tab ─ markRequirements ──────────────┐
                                                     ▼
            internal/transcript DurableStore (Header.Requirements, MarkRequirements)
                                                     │ List/Get, finalized chunks only
                                                     ▼
 host.AdmitProject ─ promises.Open(.gimbal) ─ go Store.Run(ctx, recordings, Model)
   (Close: cancel → join → release owner lock)       │ claude.TextOnly() turn, schema-validated ops
                                                     ▼
            .gimbal/promises/ledger.json {entries, cursors}  (transcript.WriteJSON, save-before-publish)
                                                     │ Snapshot/Version
                                                     ▼
 workspace.remote.go watchPromises (LiveQuery 1 s) / setPromiseWithdrawn (Command)
                                                     ▼
 ProjectWorkspace tab `/projects/{id}/promises` → PromiseLedger.svelte
```

What carries over unchanged:
- A single ledger file with cursors.
- An 8 s settle time and batches of 12.
- A fixed limit of 3 attempts on model calls, then parked until restart.
- `cite` drops any quote that is not found in the transcript.
- The worker walks every history page, regardless of which sidebar page is showing.
- The worker uses no database, queue, candidate staging or framework.

## Implementation Plan

Work in one isolated branch and keep it to one or two feature commits.

1. **Core (reuse).** Copy `internal/promises/{promises.go,promises_test.go,live_test.go}` from 8f101e94 unchanged. Keep the prompt and the tag-preservation fix that came from observed Haiku runs. Main still has every API the core calls: `gimbal.ModelBinding`, `Adapter.CreateSession/RunTurn`, `modelalias.Resolve`, `transcript.WriteJSON/List/Get`.
2. **Claude adapter (reuse).** Apply the `TextOnly()` option and `isolate(extra, textOnly)` change to main's `claude/claude.go` (`isolate` is at L92). Re-add the two test edits to main's `claude_test.go` by hand, because that file conflicts.
3. **Transcript (reuse).**
   - Add `Header.Requirements` and `OpenRequest.Requirements`.
   - Copy `Recording` creation in `Open`.
   - Add `MarkRequirements`, which writes the meta file first.
   - Add `TestRequirementsFlag`.
4. **Host (adapt).** In main's `AdmitProject` (L77–154):
   - After `transcript.NewDurable`, call `promises.Open(state)` and `promises.Model(os.Getenv("GIMBAL_PROMISES_MODEL"))`. Both run before `o.projects`/`o.order` are changed, so there is no rollback to add.
   - Start the worker after `claimed = true`.
   - In `Close` (L210), call `stopPromises()` then wait on `<-promisesDone`, before `ownerLock.Close()`.
   - Add a `Promises()` accessor.
   - Copy `host_test.go`.
   - Do not touch order, admission or reordering code.
5. **Remotes (adapt).**
   - Add `markRequirements` (Command) to main's `transcript.remote.go`.
   - Add `watchPromises` (LiveQuery) and `setPromiseWithdrawn` (Command) to main's `workspace.remote.go`, next to `reorderProjects`.
   - Do not replace either file.
6. **Generate (regenerate).** Run `go generate ./...` through `just build-assets`, then `just app` for the native SKGo client. This produces `skgo_gen.go`, `*.remote.ts`, `skgo/*/types.ts`, `types.ts` and `skgo.remotes.json`. Never copy old generated files.
7. **Native (adapt).**
   - Change `RecorderController.record(requirements: Bool = false)` and `RecordingCommands.open(…, requirements:)`, and pass the flag in `NativeCommands` (direct reuse).
   - In main's `RecordingMenu` (`Application.swift` L80), add **Start Requirements Discussion** directly under Start Recording, with no shortcut. It is disabled under the same `!controller.canRecord` condition.
   - Leave ⌘R, ⇧⌘R and ⌥⌘R exactly as they are.
   - Make Start Recording and the `RecorderControls` button call `controller.record()` through a closure.
   - Update the fakes in `ControllerTests` and `ScriptedControllerTests`, and keep the `[false, true]` assertions.
8. **Web (adapt).**
   - Add `PromiseLedger.svelte` (reuse, with the presentation cuts below).
   - Add `routes/projects/[project]/promises/+page.svelte`, following the pattern of `recordings/+page.svelte`.
   - Add the mark button to `RecordingTranscript.svelte` (reuse).
   - In main's `ProjectWorkspace.svelte`:
     - Add a `/promises` branch to `initialTab` (L39) and `visit` (L69).
     - Add a `PromiseLedger` branch to the content snippet (L154).
     - Add a Promises link in the tools nav (L178).
     - Add the small Newest/Older pager by adding an `after` state to `recordingHistory` (L31, L146), so older recordings can be marked.
   - Remembered tabs need no change: `reopened` already maps the tab to `kind: 'file'`.
   - Presentation cuts:
     - Drop the "10–30 seconds" timing copy.
     - Show one neutral empty explanation that names both entry points. This also drops the `discussed` branch.
     - Remove the collapsed "no quote" marker and the expanded "No quote found" line. Show quotes only when there are some.
     - Keep the voice-withdraw hint, the per-recording error alert and the disconnection banner.
9. **Checks.**
   - `go test ./internal/promises ./internal/transcript ./internal/host ./claude ./web`
   - Run the live test with `GIMBAL_LIVE=1` and Haiku once.
   - The native controller tests.
   - `just check-web`, `just vet` and `just build`.
10. **Live proof.** Build with `just app` and launch with `open -n --env GIMBAL_INSTANCE_DIR=<tmp> --env GIMBAL_PROMISES_MODEL=haiku …/Gimbal.app` against a scratch project. Walk through the Definition of Done table, fix only seams that actually fail, repeat the same observation, and report the SHA and model.

## Files Summary

| Area | Files | Mode |
|---|---|---|
| Core | `internal/promises/promises.go`, `promises_test.go`, `live_test.go` | reuse |
| Adapter | `claude/claude.go`, `claude/claude_test.go` | reuse hunk / re-add test |
| Transcript | `internal/transcript/durable.go`, `store.go`, `durable_test.go` | reuse |
| Host | `internal/host/host.go`, `host_test.go` | adapt / reuse |
| Remotes | `web/src/lib/transcript.remote.go`, `workspace.remote.go`, `web/workspace_test.go` | adapt |
| Generated | `internal/skgo/**`, `web/src/lib/skgo_gen.go`, `*.remote.ts`, `skgo/*/types.ts`, `types.ts`, `web/skgo.remotes.json` | regenerate |
| Native | `RecorderController.swift`, `NativeCommands.swift`, `RecorderControls.swift`, `Application.swift`, two test files | reuse / adapt menu |
| Web UI | `PromiseLedger.svelte` (new), `RecordingTranscript.svelte`, `ProjectWorkspace.svelte`, `routes/projects/[project]/promises/+page.svelte` (new) | reuse+cut / adapt |
| Excluded | old `ephemeral/promises-ledger/**` (plan, reviews, worklogs, postmortem), stub, old Justfile/go.mod/hooks/generated, `RecordingHistory.svelte` | — |

## Definition of Done

Each row is a short cross-check: the request, the existing code, what migrating it involves, and what has to be seen.

| Request | Existing code (8f101e94) | Migration | Proof that must be seen |
|---|---|---|---|
| Per-project Go ledger with title, details and tags | `promises.go` Store26/Entry65/apply389 | reuse | Expanded promise shows title, details and tags |
| Filter and sort by tag | `PromiseLedger` `shown`/`orders` | reuse, cut copy | One tag filter and Tag sort, used in the live tab |
| New requirements recording | `record(requirements:)`, OpenRequest flag | adapt to current menu | Real menu item plus real microphone; promises appear while still recording |
| Mark an existing recording | `MarkRequirements`, `markRequirements`, transcript button | reuse + regenerate | Click on an ordinary recording; its commitment enters the same ledger |
| Ordinary recordings excluded until marked | worker filters `h.Requirements`; `TestRequirementsFlag` | reuse | Plain recording saved; ledger unchanged before marking |
| Speech accumulates while open | settle/batch on finalized chunks only | reuse | Promises arrive before Stop; pause, resume and stop are unaffected |
| Source link and quote | `cite`, Evidence, quote link | reuse, remove "no quote" marker | Quote and recording link shown on one promise |
| Clear withdrawal | `withdraw` op, `SetWithdrawn` | reuse | Spoken "withdraw promise N" moves it to Withdrawn; Restore works |
| Persistence | ledger.json + cursors, save-before-publish | reuse | Reopen app and project: same entries, no duplicate processing |
| Current main preserved | — | adapt host, workspace, menu | ⌘R/⇧⌘R/⌥⌘R, Today's Transcript, remembered tabs (including Promises), project order and live chat still behave as on main in the same session |
| Ownership and safety | cancel→join in Close; TextOnly | adapt host / reuse | Package tests pass; quitting is clean |

The required checks in Implementation Plan step 9 must pass. The report must name the candidate SHA and Haiku. Nothing written to perform or record the proof is committed.

## Risks & Mitigations

- **Real microphone/GUI access is unproven.** No earlier run exercised the real menu with real speech. If the executor cannot operate the GUI, microphone and speech permissions, they report that exact gap at once. Further synthetic runs cannot close it.
- **Model environment may not reach the server.** `GIMBAL_PROMISES_MODEL` might not reach the bundled server started by the app. Check the model in the first live batch. If needed, launch with the environment variable set on the app process. Do not add configuration UI.
- **Regeneration drift.** `just app` republishes SKGo outputs. Follow its documented order and inspect the diff for unrelated churn.
- **Workspace regressions.** Only add branches to `ProjectWorkspace.svelte` and never paste the old component. Check the current-main behaviors in the same live session.
- **Model call failures.** The fixed limit of 3 attempts, then park until restart, stays as it is. It is clumsy but limits spend, and improving it is deferred.
- **Scope creep.** Reviewer or design-card suggestions do not join this sprint. Specifically out:
  - candidates and conflict interviews
  - immutable versions
  - exact quote offsets
  - retry or parked UI
  - an errors design card
  - latency promises

## Dependencies

- Current main `3ea6ecb6` toolchain: Go, skgo 0.33/polytype 1.9 generators, Vite+ (`vp`), `just`, and Xcode for `just app`.
- Source objects from `origin/promises-ledger@8f101e94`, with the copy at `takeover-2026-10-13/sources/branch-current/` as a fallback.
- Claude Code CLI login, with the `haiku` alias resolving to `claude-haiku-4-5`.
- A Mac where GUI, microphone and Speech Recognition permissions can be granted to the Debug app.

## Open Questions

None block the work. Two judgment calls are made in this draft:
- **Menu label.** The menu item is named "Start Requirements Discussion" to match main's verbs. Earlier material called it "Record Requirements Discussion".
- **Product default model.** It stays `sonnet:low`, while proofs use Haiku through the environment variable.

The critique round may overturn either one without reopening requirements.

## Companion: requirements and source map (authoritative, short)

**Requirements.** These are settled and are not to be re-asked:
- A persistent per-project Go ledger with title, details and tags, plus tag filtering and sorting.
- Two entry points: a new requirements recording, and marking an existing recording.
- Finalized speech accumulates promises while the recording is open.
- Ordinary recordings are excluded until marked.
- Each promise has a source quote and recording link.
- Clear voice or manual withdrawal, with restore.
- Persistence across restarts.

Must keep from current main:
- menus and shortcuts
- remembered tabs
- project ordering
- live chat
- the current toolchain and generators

Out of scope:
- runtime edits
- frameworks
- exotic recovery
- parity or proof programs
- history replay
- old stub or generated code

**Source revision map.**

| Source (8f101e94) | Action on main 3ea6ecb6 |
|---|---|
| `internal/promises/*` | reuse verbatim |
| `claude/claude.go` TextOnly hunk | reuse (main is unchanged since a8fbba22) |
| `claude/claude_test.go` | adapt (conflict) |
| `internal/transcript/{durable,store}.go` + test | reuse |
| `internal/host/host.go` | adapt into current admission/order/Close |
| `native/Sources/{RecorderController,NativeCommands,RecorderControls}.swift` + tests | reuse; closure call sites |
| `native/Sources/Application.swift` | adapt: add item to current `RecordingMenu` |
| `web/src/lib/{transcript,workspace}.remote.go` | adapt: append remotes |
| `PromiseLedger.svelte` | reuse with presentation cuts |
| `RecordingTranscript.svelte`, promises `+page.svelte` | reuse |
| `ProjectWorkspace.svelte` | adapt: tab, link and pager branches only |
| all `*_gen.go`, `*.remote.ts`, `skgo/*/types.ts`, `types.ts`, `skgo.remotes.json` | regenerate |
| `ephemeral/promises-ledger/**`, `RecordingHistory.svelte`, stub | drop |

**Reference only (not required reading for the executor):** the takeover journal's `ASSESSMENT.md`, `LEAN-DELIVERY.md` and `MAIN-PORT.md`; Clarity routes `ledger-mutations` (withdraw) and `claim-data-model` (sourceRefs). Clarity has no tags, so tags here are new design and not a parity target.
