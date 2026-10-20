# Zoom Connect recording (POC)

`gimbal zoom-connect record` follows one Zoom Connect call and appends its
text events to a saved Gimbal conversation. It is a proof of concept: a small
client over Zoom Connect's existing SSE endpoint and the existing conversation
store. Code: `internal/zoomconnect/` (client and event mapping),
`internal/conversation/record.go` (idempotent append), `cmd/gimbal/zoom_connect.go`.

## Run it

```sh
export ZOOM_CONNECT_API_KEY=...        # a Zoom Connect API key; never a flag
gimbal zoom-connect record \
  --url https://zoom-connect.example.com \
  --call 01J...                        # Zoom Connect call ID, not Zoom's meeting UUID
```

Run it from the project directory (or pass `--work-dir`). It follows the call
until Zoom Connect sends `event: end` or you press Ctrl-C. `--retry` sets the
reconnect wait (default 2s).

**With the normal host running** (`gimbal` serving the project, or any instance
that admitted it), the command finds the host's control socket under
`<project>/.gimbal/control/`, streams from Zoom Connect itself, and sends every
event to `POST /control/conversations/record` on that socket. The host's own
conversation manager creates, resumes and writes the conversation, so the
recording is visible in the host's Conversations list immediately and nothing
else touches the files. The project must be admitted by that host.

**With no host running** it takes the project's `owner.lock` and writes
`<project>/.gimbal/conversations/` itself; start `gimbal` afterwards to read it.
It refuses if a host owns the project but its socket does not answer.

The call has one record-only conversation per `host/call`, titled `Zoom call
host/call`, found again by its saved cursor on every run, so running the command
again resumes it rather than creating another. `--conversation ID` records into
an existing record-only conversation instead.

Repeatable local demo, no meeting, key or network: the existing tests run the real
client and the real `gimbal` command against a fake server that speaks the stream
contract below.

```sh
# ingestion, restart from the saved cursor, drop/reconnect with server replay,
# a lost write reply retried without duplicates or a second conversation, rejected key
go test -count=1 -race ./internal/zoomconnect
# idempotent append, find-or-create by source, restart persistence, workflow-backed and chat refusal
go test -count=1 -race ./internal/conversation -run Record
# the host's control endpoint over its real Unix socket: create, replay, 404s
go test -count=1 ./web -run ControlRecord
# the CLI end to end: offline recording; recording through a running host while it holds
# owner.lock, read through that host's manager, resumed after the host is closed and reopened
go test -count=1 ./cmd/gimbal -run 'ZoomConnect|NeedsSecrets'
```

## Interface mapping

Zoom Connect `GET /api/v1/calls/{id}/stream?from=N`, bearer auth, SSE frames
`id: <seq>`, `event: <kind>`, `data: <Event JSON>`, `: keepalive` comments, and a
final `event: end`. `Event` is `{seq, ts, recv, kind, participant{id,name,email}, data}`.

| Zoom Connect event | Conversation message (role `zoom`), first line |
| --- | --- |
| `transcript` (final, non-empty) | `Name: text` (`text` alone if no speaker) |
| `chat` (public chat) | `Name (chat): text` |
| `participant.joined` / `.left` | `Name joined` / `Name left` |
| `stream.started/paused/stopped/interrupted`, `call.cancelled` | `[kind] reason` |
| `speaker.active`, interim transcript, unknown kinds | not recorded |

The actual RTMS bridge (`cmd/rtmsrec/bridge.go`) currently emits mainly transcript,
chat and stream lifecycle events; participant kinds are handled when present.

Every message has a second line of provenance, built only from what the event
supplied (nothing is invented; absent fields are omitted):

```
Ana: Ship it on Friday.
(zc.example.com/01J...#4 transcript ts=1760550004000 recv=1760550004001 zoom_timestamp=1760550003987 participant=p1 email=a@example.com)
```

- `host/call#seq` is the origin (server host plus call ID) and Zoom Connect's event seq.
- `ts` and `recv` are Zoom Connect's ingestion times: `appendNext` in
  `internal/httpapi/hub.go` sets both to the time it stored the event. They are not
  the meeting clock.
- `zoom_timestamp` is Zoom's original content timestamp, which the bridge stores
  separately in `data.zoom_timestamp`. It appears only when the event carries it.
- The message's `created` is `ts` (0 if absent), so it is also ingestion time.
- The speaker is in the text because `Message` has only role, text and time.
- The default title is `Zoom call host/call`.

## Persistence, dedup, reconnect

- The highest recorded `seq` per source (`zoom-connect:<server host>/<call>`, so the same call ID on another server is a separate source) is saved in the
  conversation file next to the messages, in one atomic write, and is not shown to
  the page. `Manager.Record` skips anything at or below it, so replays, overlapping
  reconnects and restarts do not duplicate.
- A dropped or cleanly closed stream reconnects after `--retry`, from the saved
  cursor. 401, 403 and 404 from Zoom Connect stop the recorder instead of retrying.
- A host write whose reply is lost (timeout, 5xx) is retried by reconnecting and
  asking the host for its cursor first; the write itself is deduplicated by seq.
- The conversation is created in a single save that already holds the source's
  cursor (and `live: false`), and it is found again by that cursor, so an uncertain
  create does not make a second conversation. The one exception is a failure inside
  `Create` before its save (for example a checkout error), which leaves nothing behind.
- A recording refuses chat input: `Manager.Submit` and `Manager.Associate` return
  `ErrRecordOnly` for any conversation that has a source cursor, so no workflow
  starts and the recorded messages stay. It is saved and restored with `live: false`,
  which the page already treats as not accepting input. A conversation that does have
  a workflow run (an ordinary chat) refuses records: the host answers 409 and the
  recorder exits with that error.

## Limits

- Text only. Zoom Connect exposes transcript, public chat and lifecycle events;
  no audio or video is captured, and private chat is not available.
- Live use needs a Zoom Connect deployment, an API key, and a call the key's owner
  may read; none was used here. Behavior against a real Zoom Connect server is
  untested; the contract above was read from its source and exercised with fakes.
- Through a host, the control socket is local (mode 0600) and unauthenticated, like
  the rest of `/control`; do not expose it.
- The recorder uses the control socket it found when it started. A restarted host
  listens on a new socket, so a running recorder does not find it: it keeps retrying
  the old one until you stop it. After a host restart, stop the recorder and run the
  same command again; it finds the new socket and resumes the same conversation from
  the saved cursor. Tests restart the host between recorder runs, not during one.
- Offline mode and a host never write at once: offline needs `owner.lock`.
- Skipped event kinds do not advance the cursor, so a resume re-reads trailing skipped
  events; harmless.
- The recording is a read-only, record-only conversation with no harness run, and
  there is no chat or agent integration. `Manager.Get` replaces a workflow-backed
  conversation's messages with the run's transcript, which would discard recorded
  ones, so the recorder refuses any conversation that has a workflow run
  (`ErrWorkflowBacked`) and chat is refused on a recording (`ErrRecordOnly`).
- `gimbal zoom-connect` does not read `~/.gimbal` config or call Doppler.
- Each recorded event rewrites the conversation file. Fine for a call, not for
  hours of dense transcript.
- Overlaps in-flight work on conversation projection (#622), Stop (#623) and
  harness routing (#624): this touches `Manager`'s save path and `conversationRecord`.
