# Svelte cleanup: collected research and sequential delivery

Prepared 2026-09-21 against Gimble `7c90f9b`. This is a recommendation and
research index; no implementation or recurring audit has been started.

## Owner clarification (2026-09-21)

Leave #341 out of the active queue. Retain SSE; replacing it with `query.live`
is not a cleanup requirement. This supersedes the blanket SSE-replacement
recommendations in the archived catalogue and issues. The recurring audit must
respect this exception and must not keep reporting the retained SSE endpoint.

Keep resource identity in route parameters and use standard SSE resumption.
The current stream is run-wide: `/api/runs/{runID}/events` already puts the run
in the path; `?stream=` identifies the observation stream generation, not an
agent session. The session page remains `/runs/[runID]/sessions/[sessionID]`.
If stream generation is explicitly addressed in the URL, it belongs in a path
segment, not an identity query parameter.

The server already emits `id: <stream>:<position>` on deltas, but reads resume
state only from query parameters. The page closes and recreates EventSource
on errors with a 250 ms timer. The separate [#347](https://github.com/tylergannon/gimble/issues/347) should read `Last-Event-ID`,
allow native EventSource reconnection, and give replacement snapshots an event
ID too. Preserve snapshot fallback when a cursor is stale or from a previous
stream generation, and stop reconnecting when the run is complete.

Native EventSource sends the header automatically on reconnect after receiving
an `id:`; its constructor cannot set an initial custom header. Initial connection
therefore needs either a fresh snapshot or the existing permitted bootstrap
position, with the header taking precedence on reconnect. Prove reconnect,
snapshot fallback and terminal shutdown before removing the custom retry path.
See the [SSE standard](https://html.spec.whatwg.org/multipage/server-sent-events.html#the-last-event-id-header).

## Collected materials

The complete local packet is at
`/Users/tyler/.codex/worktrees/4f21/gimble/.gimble/issues/svelte-idioms-20260921/`.
Its `README.md` is the entry point. It contains:

- The six published research documents in this directory, including the pinned
  Svelte/SvelteKit documentation, both audit briefs, the 32-pattern catalogue,
  and live-query findings.
- All twenty original reviewer reports, a report index, a documentation section
  index, and the component-factoring skill the reviewers were assigned.
- Local snapshots of GitHub issues #333–344, opened this morning.
- The Claude session's scratchpad: original issue drafts, documentation inputs,
  measurement and browser scripts, and experimental outputs.
- Both live-query experiments as patches and changed source files, identified
  by commit: `claude/live-query-poc` (`d2cca85`) and
  `claude/live-window-poc` (`5e59aa2`). Neither is approved for merge.
- The unfinished session-page changes from `claude/session-page`, including its
  untracked `SessionPage.svelte`, copied without changing the original worktree.

The packet is deliberately local under the existing ignored `.gimble/issues/`
path. Raw agent output, proof scripts, and run output are not repository source.
The curated documents here remain tracked. The original scratchpad is
`/private/tmp/claude-501/-Users-tyler-src-gimble--claude-worktrees-detail-view-redesign-6d9ff0/1461ccea-11c4-48f2-8c47-aae787bcbe43/scratchpad/`;
the original raw reports are under
`/Users/tyler/src/gimble/ephemeral/research/svelte-idioms/round-{1,2}/`.

## Reading order and authority

1. [Owner rules and catalogue](dos-and-donts.md): the owner's rules override
   reviewer suggestions. Catalogue numbers are pattern numbers, not issue IDs.
2. The selected issue's local snapshot: its requested result and acceptance.
3. [Live-query findings](live-query-findings.md) for any streaming work.
4. [Documentation provenance](docs-subset.md) and the packet's `audit/DOCS-INDEX.md`
   to retrieve relevant sections of the pinned reference.
5. Raw reports and experimental source only when investigating a particular
   claim. They are proposals and historical evidence, not new requirements.

Do not feed the entire roughly 115k-token documentation subset to every worker.
Give each role the relevant local issue, owner rules, catalogue entries, and
documentation sections, with absolute paths.

## Recommended order

All twelve issues were open when fetched. This order is a delivery recommendation,
not a claim that every adjacent pair has a strict dependency.

| Order | Issue | Catalogue entries | Reason / required observation |
| --- | --- | --- | --- |
| 1 | [#343 Dead code](https://github.com/tylergannon/gimble/issues/343) | Housekeeping | Remove unused concepts before rewriting them; stories still render. |
| 2 | [#339 Runs list](https://github.com/tylergannon/gimble/issues/339) | 3, 12 | Small first delivery; links work and new runs appear without polling. |
| 3 | [#333 Conversations](https://github.com/tylergannon/gimble/issues/333) | 1, 3, 18, 20, 21, 25 | Establish route/form/small live-query use; observe create, send, reply and navigation. |
| 4 | [#335 Reactive model](https://github.com/tylergannon/gimble/issues/335) | 11 | Fix the underlying dependency model before rewriting its consumers; observe live and finished runs. |
| 5 | [#336 Derived state](https://github.com/tylergannon/gimble/issues/336) | 5, 6, 16, 17, 24 | Preserve selection through updates and reset tabs/scroll at the right identity change. |
| 6 | [#334 Owned forms](https://github.com/tylergannon/gimble/issues/334) | 10, 13 | Observe field errors and actual steer, loop message and interview answer delivery. |
| 7 | [#337 Browser APIs](https://github.com/tylergannon/gimble/issues/337) | 4, 7, 8, 9, 14, 15, 22, 23 | Observe scrolling, focus, resizing and persisted pane behavior. |
| 8 | [#338 Rendering](https://github.com/tylergannon/gimble/issues/338) | 13, 18, 19, 27–30 | Finish shared rendering after structural edits; demonstrate error containment. |
| 9 | [#340 Transport](https://github.com/tylergannon/gimble/issues/340) | 0, 26 | Typed graph transport and deletion of the unused snapshot route; preserve events. |
| 10 | [#342 Session route](https://github.com/tylergannon/gimble/issues/342) | Routing / feature | Reuse repaired leaves and inspect preserved WIP; prove direct URL, Back and streaming. |
| Deferred | [#341 Live data](https://github.com/tylergannon/gimble/issues/341) | 0, 2 | Explicitly excluded by Tyler; retain SSE. |
| 11 | [#347 Standard SSE](https://github.com/tylergannon/gimble/issues/347) | SSE follow-up | Keep SSE; route stream identity and prove native Last-Event-ID resumption and snapshot recovery. |
| 12 | [#344 Guidance and recurring audit](https://github.com/tylergannon/gimble/issues/344) | 0–3, 10, 20, 26, 31 | Promote corrected guidance, including the SSE exception, and enable recurring reports after the active cleanup. |

#344 combines several stages. Its window/map work associated with #341 is also
deferred; it does not block finishing this queue or scheduling the recurring
audit. Generated argument validation belongs to skgo, not hand edits to Gimble's
generated remote stubs.

## Workflow recommendation

Write one ordinary Go workflow over the ordered local issue files with
`gimble.Iterate`. For each issue: refresh its local text and inspect current main,
create one worktree, implement, run relevant checks, independently observe the
requested behavior, repair substantial failures within a proposed three-attempt
limit, then have the delivery agent commit, push, open and squash-merge the PR.
Confirm the merge before starting the next issue from updated main. Stop and
retain the worktree on an execution failure or exhausted attempts; a blocked
issue stays explicitly pending. No parallel issue writers or candidate bake-offs.

Use GPT Terra for coding, Claude Sonnet for independent validation and difficult
local reasoning, and Gemini Flash for documentation retrieval, candidate scans
and deduplication. Configure these roles explicitly: existing workflow defaults
include more expensive models. Keep scope coaching advisory. Use Haiku/Luna/Flash
for live application probes and report the model actually used.

The existing `internal/workflows/implementation/implementation.go` supplies a
useful bounded worker/validator pattern, but explicitly does not commit, push or
merge. It is not already a complete issue-delivery workflow. Keep the new
sequence visible in its source; GitHub actions belong to agent turns under the
repository's definition of done. Regenerate its CLI and graph when implemented.

After the active list is resolved, enable a weekly audit of current main: Flash finds
candidates using the catalogue and relevant documentation; Sonnet independently
checks the actual code, rejects duplicates and unsupported suggestions, and
opens a report only for a confirmed new pattern or recurrence. Reports need a
name, exact sites, rule/doc basis, concrete consequence, idiomatic replacement,
and observable acceptance. Keep known exceptions and rejected suggestions in
the catalogue. No fixed finding quota; no notification when nothing changed.

## Findings that affect the plan

- The live-query results are Claude's recorded findings, not experiments rerun
  during this collation. Delta batching failed; the open-window spike passed
  limited checks but did not prove deliberately dropped frames, long-outage
  recovery, or bytes versus the existing endpoint on the same run. #341 and its
  prerequisites are now deferred by the owner, rather than completion gates.
- The raw reports are noisy. Round 1 reviewer 01 recommends query-string filters
  that the owner rejected. Round 2 reviewer 01 still offers `command` for the
  loop's real form. Round 2 reviewer 09 finding 10 explicitly describes correct
  code and a speculative maintenance risk. These are not additional work items.
  The briefs' demand for exactly ten findings should not survive into automation.
- Existing catalogue doc references after line 2442 are shifted by 71 lines;
  use section headings and the new section index. Verify a replacement against
  the installed dependency versions before editing, especially new syntax.
- #342 includes an unresolved product decision about selection/dialog/pane
  persistence. Deliver its explicit session route without silently deciding the
  broader persistence behavior. Preserve the existing work-in-progress as input.
- Official references checked while collating:
  [remote functions](https://svelte.dev/docs/kit/remote-functions) and
  [$derived](https://svelte.dev/docs/svelte/$derived). The pinned local subset
  remains the reference for interpreting this morning's reports.
