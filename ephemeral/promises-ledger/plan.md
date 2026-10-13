# Promises ledger — implementation plan

## 0. Authoritative request (Tyler, 2026-10-12, restated from conversation)

- Every project has zero or more **promises**: bullet points stating what the
  software promises its external stakeholders — specific functionality,
  performance, and reliability promises. Each may be tagged for filtering and
  sorting and has a description (details) paragraph. Somewhat structured data.
- Model it on Clarity's claims ledger (diffusioninc/clarity), a subset of it.
  The ledger is written in Go, includes the details section, and is viewable as
  a tab within the project.
- When starting a recording in Gimbal, the user can mark it as a
  requirements-gathering recording whose transcript goes through the
  "clarity" functionality into this ledger. On an existing recording, a button
  declares it a requirements discussion too.
- Copy Clarity's functionality as closely as reasonable — streaming a stream of
  text in and getting it sorted out — but **lean**: no demands for exhaustive
  completeness, no proof machinery, nothing that proves precise parity with
  Clarity. Clarity's conflict-resolution behavior is partly wonky; don't
  obsess over perfecting it.
- Done when it actually works: you can talk into Gimbal and promises
  accumulate in the project's Promises tab. Other Clarity functionality may be
  deferred.
- Store the ledger wherever makes sense for what is being built (the
  `df-promise` skill is related but different; not a constraint).

Research backing this plan (citation-dense notes, semantic indexes, and a
token cache of Clarity upstream main @ 9dfa977c plus Gimbal) lives in
`/Users/tyler/src/promises-research/` (`notes/`, `indexes/`, `token-cache/`).
Citations `clarity/…` resolve under `/Users/tyler/src/promises-research/token-cache/`.
Gimbal citations are against this worktree at a8fbba22.

## 1. Goal and done-criteria

Requirements recordings feed a per-project ledger of promises (title, details,
tags, evidence); an LLM keeps it current; a Promises tab shows it.

Done = this demo works end to end:
1. In the macOS app, with project X selected, Recording ▸ "Record Requirements
   Discussion". Speak a few promises ("Exports finish in under two seconds",
   "It never loses an unsaved note", "Users can tag notes"), pause until they
   appear (the 8 s settle), then say "withdraw the second one". Stop.
2. Open X ▸ Promises: within ~10–30 s of each quiet pause, numbered promises
   appear with tags and details; #2 moves to Withdrawn.
3. Open an older ordinary recording ▸ "Treat as requirements discussion": its
   promises merge into the same ledger.
4. Restart `gimbal`: ledger unchanged, no recording reprocessed.

## 2. Non-goals (scope fence)

- No proof/acceptance machinery: no Codex ingestion/FIFO, locks, deferred
  corrections, or conflict interviews (clarity/docs/ARCHITECTURE.md:L361-528).
- No `hold_ambiguous`/clarifications: ambiguous target ⇒ prompt says return no
  op (clarity/Sources/ClarityCore/Persistence/ClarityStore+Deliberation.swift:L1300-1369).
- No separate cutter stage, candidates buffer, or grapheme offsets.
- No version history file. Per-entry evidence is the audit trail.
- Deferred UI: add/edit form, un-marking a recording, tag rename, export,
  status-popover record item (menu only), sidebar badge.
- No `Promise*` name in the root `gimbal` package; no Go type named `Promise`
  (skgo would emit a TS `type Promise` shadowing the global).

## 3. Data model

New package `internal/promises` (`promises.go` + `promises_test.go`):

```go
// Store owns <project>/.gimbal/promises/ledger.json; the only writer.
type Store struct {
    mu      sync.Mutex
    dir     string
    ledger  Ledger  // the last acknowledged (nil-error) save, or the file at Open
    working bool    // a batch's LLM call is in flight
    errs    map[string]string // recording ID -> its last batch failure; deleted on that recording's success
    version int64   // bumped on every change to ledger, working or errs
}

// Ledger is ledger.json.
type Ledger struct {
    Entries []Entry        `json:"entries"`  // ordered by ID
    Cursors map[string]int `json:"cursors"`  // recording ID -> finalized chunks consumed
    Updated int64          `json:"updated"`  // ms; bumped on every save
}

type Failure struct {
    Recording string `json:"recording"`
    Message   string `json:"message"`
}

// View is what the Promises tab watches: the entries plus in-memory status.
type View struct {
    Entries []Entry `json:"entries"`
    Working bool    `json:"working"` // LLM call in flight
    Errors  []Failure `json:"errors"` // recordings whose last batch failed, oldest first
    Version int64   `json:"version"` // bumped on any change (live-query change signal)
}
type Entry struct {
    ID        int        `json:"id"`        // display number "#3"; max+1, never reused
    Title     string     `json:"title"`
    Details   string     `json:"details"`
    Tags      []string   `json:"tags"`      // lowercase, trimmed, deduped
    Withdrawn bool       `json:"withdrawn"`
    Created   int64      `json:"created"`
    Updated   int64      `json:"updated"`
    Evidence  []Evidence `json:"evidence"`
}
type Evidence struct {
    Recording string `json:"recording"`
    Chunk     int    `json:"chunk"` // index into Recording.Finalized
    Quote     string `json:"quote"` // ≤280 chars, model-supplied
}
```

- One integer ID = stable identity = display number = ordinal the LLM resolves
  "the second one" against (Clarity: display_ordinal from stable_position,
  clarity/server/internal/worker/review.go:L52; max+1 allocation,
  clarity/server/internal/worker/apply_review.go:L190).
- Files: `<project>/.gimbal/promises/ledger.json` and an empty
  `<project>/.gimbal/promises/work/` used as the harness cwd; siblings of
  `recordings/` (`internal/transcript/durable.go:45`). Writes use
  `transcript.WriteJSON` (temp+fsync+rename+dir fsync, `durable.go:95-126`).
- One file ⇒ a batch's entry changes and cursor advance commit in one atomic
  rename: that is the whole crash-idempotency story.

Methods (each has a caller): `Open(gimbalDir)` (load or empty; reset
`Working=false`, as `NewDurable` does, `durable.go:69-78`); `Snapshot() View`
(deep copy as a `View`, live query); `SetWithdrawn(id, bool) (Entry, error)` (UI);
`Run(ctx, recordings *transcript.DurableStore, model gimbal.ModelBinding)`
(worker). Unexported `step`, `apply`, `save`.

Host wiring (`internal/host/host.go`): field `promises *promises.Store` on
`Project` (`:39-50`); in `AdmitProject` after `p.recordings` (`:132-136`),
`promises.Open(state)`; before `go p.cleanupExitedTasks()` (`:149`):

```go
model, err := promises.Model(os.Getenv("GIMBAL_PROMISES_MODEL")) // "" -> "sonnet:low"
// err -> admission error (explicit misconfiguration only)
ctx, stop := context.WithCancel(p.ctx)
p.stopPromises, p.promisesDone = stop, make(chan struct{})
go func() { defer close(p.promisesDone); p.promises.Run(ctx, p.recordings, model) }()
```

`promises.Model(spec)` resolves the spec through `modelalias` (as
`binding.Parse` does, `internal/binding/binding.go:25-46`), requires the
`claude` harness, and binds it to `claude.New(claude.TextOnly())` — see §6
"Text only".

Accessor beside `Recordings()` (`:363`): `func (p *Project) Promises() *promises.Store`.

## 4. Recording flag — `Requirements bool` (`json:"requirements"`)

Go (`internal/transcript`):
- `store.go:18-22`: add the field to `OpenRequest`.
- `durable.go:24-31`: add to `Header` (persisted in `<id>.meta.json` by `save`,
  `:127-136`; cached in `s.headers`). `Open` (`:157`) copies it. The reopen
  conflict check (`:151`) stays source-only so a retried open after a web mark
  does not fail.
- New `func (s *DurableStore) MarkRequirements(id string) (Header, error)`:
  under `s.mu`, read the recording, set the flag, then write `<id>.meta.json`
  **first** (and update `s.headers`), then `<id>.json` — the reverse of
  `save`'s order (`:127-135`). The meta file is what `List` and the worker
  read; the full file is what the recording view reads. So a failure between
  the two writes leaves the worker ingesting while the view still offers the
  button; clicking again rewrites both (idempotent). Does not touch `Updated`
  (liveness signal in `visible()`, `:226-231`). One-way.
- `durable_test.go`: flag via Open survives `NewDurable` reload;
  `MarkRequirements` persists; unknown id errors; with `<id>.json` replaced by
  a directory so the second write fails, the call errors, `List` reports the
  flag, and a retry after removing the obstruction sets both.

Remote (`web/src/lib/transcript.remote.go`, beside its siblings):
`func markRequirements(ctx, event, r RecordingRequest) (transcript.Header, error)`
registered `skgo.Command`. Web-only; not in the Swift remote list.

Swift:
- `native/Sources/RecorderController.swift:17` protocol `open(sessionID:source:requirements:)`;
  `:101` `func record(requirements: Bool = false)`; `:146` passes it.
- `native/Sources/NativeCommands.swift:23,33` signature + `OpenRequest(..., requirements:)`.
- `native/Sources/Application.swift:79` `action: controller.record` →
  `{ controller.record() }` (defaulted param breaks the method reference), plus
  `Button("Record Requirements Discussion") { controller.record(requirements: true) }.disabled(!controller.canRecord)`.
- `native/Sources/RecorderControls.swift:27` same closure fix.
- Test doubles: `native/Tests/ControllerTests.swift:65`,
  `native/Tests/ScriptedControllerTests.swift:19`.

Codegen: `go generate ./...` (`internal/skgo/config.go:18`) regenerates
`native/Generated.swift`, `web/src/lib/*.remote.ts`, `web/src/lib/skgo/*/types.ts`.
`native/skgo-native.json:61-68` unchanged (`openSession` already listed).

## 5. Ledger worker

One goroutine per project from `AdmitProject` (like `cleanupExitedTasks`,
`host.go:149`, `:334-355`); exits with `p.ctx`. It is joined, not abandoned:
`Project` gains `stopPromises context.CancelFunc` and `promisesDone chan struct{}`
(closed when `Run` returns); `Owner.Close` (`host.go:177-183`) calls
`p.stopPromises()` then waits `<-p.promisesDone` before `p.ownerLock.Close()`.
Close cancels the worker itself, so callers that close with a live context
(`web/src/lib/workspace.remote_test.go:13-37`, `internal/host/routes_test.go:115-138`)
do not hang; the wait is bounded by one in-flight turn's cancellation grace
(`claude/claude.go:320-328`) plus one save, and a second Gimbal cannot take
`owner.lock` while the old worker can still write `ledger.json`. Writers: this goroutine and
`SetWithdrawn`, both under `Store.mu`.

Cadence: `Run` = 2 s ticker calling `step(ctx, recordings, model, now)`. One step:
1. Backoff is per recording: `retryAt[id]` (in memory). A recording in
   backoff is skipped and the step considers the next one.
2. Page `recordings.List(after)` until `Next == ""` (`durable.go:239-271`,
   25 per page); keep `Requirements` headers. Tested with more than 25
   recordings.
3. Per recording: `r := recordings.Get(id)` (`:232-238`), `done := Cursors[id]`,
   `n := len(r.Finalized)`; skip if `n <= done`. Finalized is append-only
   (`:194`); provisional is never sent (replaced every update, `:195`).
4. Settle: in-memory `seen[id] = {count, at}`; process when status is
   `stopped`/`interrupted`, or `n-done >= 12`, or the count is unchanged for
   8 s. (`Header.Updated` can't be the quiet signal: heartbeats bump it every
   5 s, `durable.go:223`, `web/src/lib/transcript.remote.go:67-73`.) 8 s mirrors
   Clarity's settle grace (clarity/Sources/ClarityCore/Deliberation/ClaimDeliberationCoordinator.swift:L4-21).
5. Process one batch per step — the first eligible recording not in backoff:
   new `[done, min(n, done+12))`, overlap
   `[max(0,done-2), done)` (Clarity overlap 2, same file L11). Serial ⇒ one LLM
   call in flight per project.

Batch transaction: (1) lock, snapshot active entries + tags in use, set
`Working=true` (in memory), unlock; (2) LLM call, no lock; (3) lock, build
`next` = deep copy of the current ledger, `apply(ops)` to `next` (absorbs UI
withdraws made during the call), set `Cursors[id]=end`, save
`next`; only if the save returns nil does `next` become the in-memory ledger.

Live status transitions (all under `mu`; each bumps `version`):

| event (recording R) | working | errs | ledger |
|---|---|---|---|
| `Open` | false | empty | loaded from disk |
| batch for R starts (before the LLM call) | true | unchanged | — |
| batch for R saved | false | delete `errs[R]` | `next` |
| LLM/validation failure for R | false | `errs[R]` = message | — |
| save failure for R | false | `errs[R]` = message | unchanged (old) |
| `SetWithdrawn` saved / failed | unchanged | unchanged | `next` / unchanged |

A success for one recording never clears another recording's failure.
`SetWithdrawn` returns its save error to the caller instead of setting `errs`.
`working`/`errs` never touch the file. `Snapshot` copies entries plus these
three fields into a `View`.

Save failure: only a nil return from `WriteJSON` (which syncs the file and the
parent dir, `durable.go:95-125`) acknowledges a batch. Any error — before or
after the rename — discards `next`: memory keeps the old ledger and old cursor,
`errs[R]` is set, R backs off, and the retry reruns the batch against the old
ledger and rewrites the whole file. The file always holds either the old or the
new complete ledger, each self-consistent (cursor and entries in one rename),
so a crash in this window at worst redoes one batch; it never duplicates or
loses an acknowledged one. No reload, no post-hoc cursor inspection. Tested by
making the promises dir unwritable for one step. `SetWithdrawn` uses the same copy-save-publish rule.

Idempotency: crash/cancel before (3) leaves cursor and entries untouched ⇒
batch is redone, no duplicates; an applied batch can't replay because cursor
and ops commit in one rename (Clarity "replaying an applied pass is a no-op",
clarity/docs/ARCHITECTURE.md:L355-360, without pass ids). Marking an old
recording = absent cursor = 0; backlog drains 12 chunks per call.

Failures (adapter error, invalid JSON, 3 min timeout): lock, `working=false`,
`errs[R]` set; cursor not advanced; `retryAt[id] = now+backoff[id]` (15 s,
then 30 s). Attempts are bounded like Clarity's two-attempts-per-drain
(ClaimDeliberationCoordinator.swift:L8-19): after 3 consecutive failures R is
parked for the life of the process (`retryAt[id]` = never) and `errs[R]` reads
"…stopped after 3 attempts; restart Gimbal to retry". Restarting is the retry
button, so a persistent fault costs at most 3 turns per recording per run.
A success resets R's count. Other recordings keep progressing. The recording
itself is never touched.

Session per batch: `CreateSession(ctx, model.Model, model.Effort, <dir>/promises/work)`
→ `RunTurn` → `Close`. The worker only ever holds the Claude adapter, whose
Claude Code process is scoped to one turn and whose `Close` just forgets the id
and returns nil (`claude/claude.go:189-196, 244-252`), so there is no native
session left to own or retry after a batch.

## 6. LLM call

Model: default `sonnet:low` (`internal/modelalias/aliases_gen.go:31`,
`modelalias.go:66-81`); override `GIMBAL_PROMISES_MODEL` with any Claude alias
spec (`haiku`, `sonnet:low`, `opus:medium`; `haiku` for proof runs per
`AGENTS.md:73-75`). A non-Claude spec fails admission.

Text only: transcript speech is untrusted input, and the Claude adapter runs
Claude Code with bypass permissions, its built-in tools, and the repository's
`project,local` settings — which can define command hooks that receive the
prompt (`claude/claude.go:70-95, 144-150, 164-167`). A prompt sentence is not a
control, so the worker's adapter is built with a new option
`claude.TextOnly()` that passes `--tools ""` and `--setting-sources ""`
(replacing `project,local` in the adapter's `extra` map), keeping
`--strict-mcp-config`: no tools, no settings, no hooks, no MCP servers. The
CLI's own login is unaffected. Checked by hand in a directory whose
`.claude/settings.json` has a `UserPromptSubmit` command hook: with
`project,local` the hook fires; with `--tools "" --setting-sources ""
--strict-mcp-config --json-schema …` it does not, auth works, and
`structured_output` is returned. The option sits beside
`claude.WithUserConfiguration` (`claude/claude.go:41-55`).
`~/.gimbal/config.json` secrets reach the env (`cmd/gimbal/secrets.go:49-56`).
Not `Session.Generate`: every batch would become a Runs-list entry.

Prompt (const in `promises.go`, followed by `"\n\nRequest:\n" + json(request)`):

```
You maintain the promise ledger for one software project. A promise is one complete, judgeable
statement of what the software promises its external stakeholders: functionality it provides,
performance it achieves, or reliability it guarantees. A prohibition (what it must never do) is a
promise too.

The request holds the ledger's active promises (id is also the number people see and say), the tags
already in use, overlap_chunks you have already processed, and new_chunks of a finalized speech
transcript from a requirements discussion. Overlap chunks are context: act only on intent completed
in new_chunks, but a thought that began in overlap and finishes in new_chunks is new.

New speech may propose, restate, refine, correct, or withdraw promises. Return only operations over
the exact ids supplied. Never invent an id.
- add: new durable intent not already represented by an active promise. title is one sentence
  stating the promise in the speaker's vocabulary, preserving qualifications, negation, numbers,
  and examples. details is a short paragraph of the context, rationale, constraints, and examples
  the speech gave; add nothing the speech did not support. id is 0.
- update: speech materially refines or corrects an active promise. Return its complete replacement
  title, details, and tags. For a meaning-equivalent restatement, return the current title,
  details, and tags unchanged so the evidence is recorded without changing the promise. Never add a
  duplicate of an active promise.
- withdraw: speech clearly retracts one exact active promise. Resolve "the second one", "promise 4",
  or a description from the supplied ids and texts.
If a correction or retraction could target more than one promise, return no operation for it. Do not
mutate a best guess. A promise that new_chunks both propose and retract is not added; one they propose
and then refine is added once, in its refined form.

Transcription turn boundaries can cut a sentence. A dangling clause without its object or
qualification is unresolved speech, not a complete promise; a later request will show it again as
overlap. Never record a promise about the speaker being incomplete. Capture intent generously while
the ledger is taking shape: a standalone product description, aspiration, example, or incomplete
design is valid durable intent. Ignore chit-chat and talk about the meeting itself. Do not use
keyword rules.

Tags: one to four short lowercase tags per promise. Reuse tags_in_use where they fit. Include one of
functionality, performance, or reliability when one applies.

Every operation names the new_chunks chunk number it rests on, a short verbatim quote from that
chunk, and a one-line reason. Return operations in source order. Returning no operations is valid
when the speech adds no promise.
```

Rules lifted from clarity/Sources/ClarityCore/Claims/ResponsesClaimReviewProvider.swift:L271-304
plus clarity/server/internal/formation/instructions.txt:L19-23 and
review-instructions.txt:L38.

Request: `{"promises":[{id,title,details,tags}], "tags_in_use":[], "overlap_chunks":[{chunk,text}], "new_chunks":[{chunk,text}]}`
— all active entries, uncapped (Clarity's 24 cap broke dedup); `chunk` is the
absolute index into `Finalized`.

Schema (raw JSON const like `conversationReplySchema`, `manager.go:604-611`;
all fields required for OpenAI strict mode):

```json
{"type":"object","additionalProperties":false,"required":["operations"],"properties":{
 "operations":{"type":"array","maxItems":64,"items":{"type":"object","additionalProperties":false,
  "required":["op","id","title","details","tags","chunk","quote","reason"],
  "properties":{"op":{"enum":["add","update","withdraw"]},"id":{"type":"integer"},
   "title":{"type":"string"},"details":{"type":"string"},
   "tags":{"type":"array","items":{"type":"string"}},
   "chunk":{"type":"integer"},"quote":{"type":"string"},"reason":{"type":"string"}}}}}}
```

Adapters validate against the schema (`harness.go:38-40`); decode
`result.Output` as at `manager.go:428-431`.

`apply` — one switch, after clarity/server/internal/worker/apply_review.go:L186-247:
- Evidence is only ever a quote found in the transcript: the model's quote is
  looked up (case- and whitespace-insensitive) first in its named chunk, then
  in every other new chunk; the chunk where it is found is recorded, truncated
  to 280 chars. A quote found nowhere ⇒ no evidence is appended for that op
  (the op itself still applies). Evidence never invalidates an op and never
  attaches unrelated speech.
- `add`: `ID=maxID+1`; normalize tags; append evidence.
- `update`: an empty title or details keeps the current value; identical title/details/tags ⇒ evidence only (fixes Clarity's
  "equivalent records nothing" wonk); else replace, bump `Updated`, append
  evidence. A user-withdrawn entry stays withdrawn.
- `withdraw`: already withdrawn ⇒ no-op; else set `Withdrawn`, bump, evidence.
- Empty ops ⇒ only the cursor advances.
- Validation before apply (the whole reply is accepted or rejected, as Clarity
  rejects a whole pass, `ResponsesClaimReviewProvider.swift:100-182`): an add
  with an empty title or details, an update/withdraw naming an id not in the
  request's `promises`, or an unknown `op` fails the batch — the error names the
  op (in `errs[R]`), cursor stays, that recording backs off and the batch is retried. A
  withdraw of an entry the user withdrew during the call is a valid no-op.
  With 12 chunks per batch and 64 operations allowed, capacity is not the
  limit.

## 7. Web

Remotes in `web/src/lib/workspace.remote.go` (existing shared module,
`web/runtime.go:270`; `project()` helper `:19-29`):

```go
func watchPromises(ctx context.Context, id string, yield func(promises.View) error) error
// skgo.LiveQuery; 1 s ticker like watchRecording (transcript.remote.go:91-112), yields only when Version changes
type PromiseRequest struct { Project string `json:"project"`; ID int `json:"id"`; Withdrawn bool `json:"withdrawn"` }
func setPromiseWithdrawn(ctx context.Context, event app.RequestEvent[params.Params], r PromiseRequest) (promises.Entry, error)
// skgo.Command
```

Route stub `web/src/routes/projects/[project]/promises/+page.svelte` (title
only, like `recordings/+page.svelte`).

Older recordings: the sidebar's history (`ProjectWorkspace.svelte:24-31,
149-150`) shows the newest 25 and ignores `History.Next`. Add a "Show older"
button under the list while `next` is non-empty; it calls the existing
`recordingHistory` query with `after: next` and appends the page to a local
`$state` list. No new remote. Proof: a scratch project with 30 recordings, the
oldest reachable and markable.

`web/src/lib/workspace/ProjectWorkspace.svelte`: `initialTab` (`:34-38`),
`visit` (`:60-63`), content branch before the `RecordingTranscript`
fallthrough (`:135`), tools-nav link (`:152`). Tab `kind: 'file'` (existing
glyph), no new kind.

New `web/src/lib/workspace/PromiseLedger.svelte`: `watchPromises(project)`;
local `$state` tag filter (chip toggle) and order (`number | updated | tag`);
status line while `working`; each of `errors` in `role="alert"` naming its recording (link); active list as
`<details>` rows (`#id title` + tag chips; body: details paragraph, evidence
quotes linking to `${root}/recordings/{recording}`, Withdraw); a "Withdrawn
(n)" `<details>` with Restore; empty state "Mark a recording as a requirements
discussion; its promises appear here."

`web/src/lib/workspace/RecordingTranscript.svelte:14-15`: if
`!record.requirements` a "Treat as requirements discussion" button
(`markRequirements`); else "Requirements discussion · feeds Promises" link.
`watchRecording` re-reads every 1 s, so no refresh wiring.

## 8. Tests (beside the code)

- `internal/transcript/durable_test.go`: flag via Open, `MarkRequirements`, reload.
- `claude/claude_test.go`: `TextOnly` passes `--tools ""` and `--setting-sources ""`.
- `internal/promises/live_test.go` (below) runs its turns in a work dir whose
  `.claude/settings.json` holds a `UserPromptSubmit` hook that touches a file,
  and asserts the file never appears.
- `internal/promises/promises_test.go` with a test-local fake
  `gimbal.HarnessAdapter`, a real `DurableStore` in `t.TempDir()`, `step` with
  explicit `now`: apply cases (ids, equivalent update ⇒ evidence only, changed
  update, withdraw, unknown id, empty title or details on add, empty fields on
  update keep current values, tag normalization, a quote found in a different
  new chunk is attributed to that chunk, a fabricated quote ⇒ no evidence but
  the op applies);
  settle (live not processed at t+0 but at t+8 s; stopped immediately;
  non-requirements ignored; 15 chunks ⇒ 12 then 3 with 2 overlap);
  validation (empty-details add or unknown-id update ⇒ batch fails, cursor
  stays); per-recording backoff (recording A failing persistently while
  recording B is processed; A gets exactly 3 calls then is parked with the
  "restart Gimbal to retry" error); status transitions (`working` true during the
  call, false after success and after failure);
  idempotency (no call after success; error ⇒ cursor unchanged + backoff;
  a later success for that recording clears its error, and a success for another recording does not; reopen restores; unwritable promises dir ⇒
  in-memory ledger unchanged, cursor not advanced, retried after backoff;
  more than 25 recordings ⇒ a marked recording on page 2 is processed); concurrency
  (`SetWithdrawn` during a blocked RunTurn survives apply); shutdown (cancel
  ctx during a blocked RunTurn ⇒ `Run` returns only after the turn unwinds,
  cursor unchanged, no save after return).
- `internal/promises/live_test.go`, only with `GIMBAL_LIVE=1`, haiku: Clarity
  fixtures 03-restatement-equivalent, 04-restatement-refinement,
  07-withdrawal-second inlined (one batch per turn); plus one combined batch
  (three promises then "withdraw the second one" in the same chunks) asserting
  the retracted promise is absent; assert entry/withdrawn counts and keywords.

## 9. Build order (each step's proof)

1. Transcript flag + `MarkRequirements` + tests — `go test ./internal/transcript`.
2. `internal/promises` types, Open/save/apply/SetWithdrawn + tests — `go test ./internal/promises`.
3. `step`/`Run`, prompt, schema, fake-adapter tests — `go test -race ./internal/promises`.
4. Live eval — `GIMBAL_LIVE=1 go test ./internal/promises -run Live` on haiku; tune prompt.
5. Host wiring + model default — build; `gimbal` on a scratch project creates `.gimbal/promises/`; bogus model fails admission clearly.
6. Remotes + `go generate ./...` — `just build`, `just vet`, `go test ./internal/skgo`.
7. UI — real server on a scratch project with a copied recording: mark it, watch promises appear live; withdraw/restore, filter, sorts; restart without reprocessing.
8. Native flag + menu item + doubles — `scripts/check-recorder.sh`; synthetic-source native build: the menu item yields `"requirements":true` in `.meta.json` and promises appear.
9. Real demo (§1) with the microphone and `GIMBAL_PROMISES_MODEL=haiku` (`AGENTS.md:70-75`); report what appeared and the model used.

## 10. Decisions and risks

- One JSON file for entries+cursors: atomic batch commit, no pass ids (`durable.go:95-136`; clarity/server/internal/api/voice.go:L176-199).
- Integer ID = display number = ordinal (clarity/server/internal/worker/review.go:L52).
- Direct `adapter.RunTurn`, fresh session per batch: keeps the Runs list clean (`manager.go:405-424`). No schema re-ask: invalid reply ⇒ that recording's error + backoff + retry (at most 3 per run).
- Claude-only, text-only adapter for the ledger call: speech is untrusted; no tools, settings, hooks, or MCP rather than asking the model not to act.
- Commits: copy → apply → save → publish only on a nil save; any save error keeps the old in-memory ledger and the batch is redone.
- Sidebar "Show older" over the existing paged `recordingHistory`, so every existing recording can be opened and marked.
- `sonnet:low` default (same tier as Clarity's gpt-6.1-sol low, clarity/Sources/ClarityCore/Claims/ResponsesClaimReviewProvider.swift:L16, L52-53). Each batch spawns a Claude Code process (~5–15 s); settle + serial batches bound it.
- Settle signal is an in-memory unchanged chunk count (heartbeats bump `Updated`).
- Finalized chunks only; overlap + dangling-clause rule handle split sentences.
- Single merged LLM stage, no offsets (Clarity's two-stage offset validation is its main wonk).
- All active promises in context, uncapped; prompt size is fine at project scale.
- Ambiguity ⇒ no op; the Withdraw button is the remedy.
- Remotes in existing shared modules; `web/runtime.go` untouched.
- Native entry point is a menu item; popover deferred; the web button covers it.
- Bad `GIMBAL_PROMISES_MODEL` fails admission loudly.
- Shutdown: worker exits on `p.ctx`; `Owner.Close` cancels and joins it before releasing `owner.lock`; an in-flight call is cancelled without advancing the cursor; writes are atomic.
