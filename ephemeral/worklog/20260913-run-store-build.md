# Worklog: the run store build (2026-09-13)

doc_bug: RUN-STORE.md Dependencies says the branch is behind main and must
merge it; `git rev-list --count HEAD..main` is 0 and #156/#146 are both in
the branch history, so no merge was done.

decision: `Open` returns `(*Store, error)`, because the six empty files are
written there and that write can fail; `run.go` feeds the error to
`recordFailure`, so a run whose recording broke says so.

decision: `observeLifecycle` keeps its five parameters with `_` placeholders.
`event_persistence.go` is out of scope, and it is the caller.

decision: `ModelCallRow.Run` and `.Turn` are filled where the row is
appended, so `model_calls.json` rows stand on their own; `TurnUsageRow`'s
three keys are filled where the table is flattened, since the map holds only
the usage.

decision: a frame is marshalled only when the store has a subscriber. A
replay of a few hundred turns otherwise encodes thousands of frames nobody
reads.

decision: a model call is identified by its turn and its assistant message.
Folding the same step twice rewrites its row and is not accounted twice, so a
log can be read over facts already held (coordinator's instruction, ahead of
the revised finished-run path).

fact: `data.assistantMessageID` on a step event is already the normalized
message id: it equals `native_ref.normalizedMessageID` in every step record
of the saved issue-149 run, so `model_call.message` needs no lookup.

fact: `sessionstate.DecodeValue` decodes with `UseNumber`, so an event's
`created` is a `json.Number` and not a `float64`. Reading it as `float64`
silently yields 0, which is what the first model-call test caught.

friction: the plan's Phase 1 gate cannot be green by itself.
`web/observation_ssr_test.go` renders the real Svelte page, and the page that
reads the new snapshot shape is Phase 2's `RunViewer`, so `go test ./web`
fails on that one test until Phase 2 lands. Everything else in the gate
passes.

decision: two page lines moved into Phase 1 because the Go gate needs them:
`+page.svelte` no longer calls `scopeUsage` (the binary no longer serves that
remote, and the built frontend refuses to start against it), and
`index.ts` declares the nested `Usage` type it used to import from the
deleted generated file. Phase 2 replaces both properly.

friction: the coordinator put `replay.go` and the registry open path on hold
after both were already written and passing. They are left as they are, and
`registry_test.go` is not written, pending the revision.

decision: reading a finished run loads the six tables and does not fold its
run log. The tables are the data model and Gimble reads them; the log is the
transcript store and the source a rebuild reads. The session logs are still
read on that path, for the transcript projections only: a turn that ended
takes no usage from a step, and a model call the file already holds is
rewritten rather than added, so the facts stay the files'. A directory
missing any of the six is rebuilt from the logs and the six are written.
(Coordinator's revision, mid-build; the plan's § Replay describes the
rebuild path only.)

friction: the Svelte MCP tools are not in this session's tool list, and a
ToolSearch for them finds nothing, so `svelte-autofixer` could not be run.
`pnpm run check` (svelte-check, 353 files) reports 0 errors and 0 warnings on
the changed components instead.

decision: `MessageRow.svelte` is unchanged. `usageOf` already returns the flat
`Usage`, and `usageText` reads it, so the call site needed nothing.

decision: an absent roll-up renders through `usageOf(undefined)` rather than a
new exported zero constant. A scope or session Go has not summed yet reads as
five zeros and a cost of 0, which is what it spent.

decision: a store filled from the tables accounts for nothing from a log
(review finding 1). `loadTables` sets `fromTables`, and `foldStepLocked`
returns immediately when it is set, so the session logs read after a load
build transcripts and provenance only. The ended-turn guard protected
turn_usage but not model_calls: an ended step in the log replaced or appended
a row the table was the authority for. The rebuild path keeps the full fold.
`TestFinishedRunIsReadFromItsTables` now empties model_calls.json and asserts
the snapshot holds none, which fails without the guard.

fact: the two probe programs were stale in two ways, not one (review finding
2). Their fake adapters lacked `Close`, and their `RunTurn` still returned
`json.RawMessage` rather than `gimble.TurnResult`, so adding `Close` alone
left `go vet ./...` red. Both signatures are now current and nothing else in
those files changed.
