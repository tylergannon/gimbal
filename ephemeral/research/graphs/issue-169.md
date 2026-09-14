# Serve finished runs from their logs, not from observation.json

URL: https://github.com/tylergannon/gimble/issues/169
State: closed
Updated: 2026-09-14T00:38:43Z

## Problem

A finished run is served from `runs/<id>/observation.json`, a file written once at run close (`internal/observation/checkpoint.go`, read by `registry.go:91`). It is `json.Marshal` of the live store's whole in-memory snapshot: run info, sessions, session totals, scopes, and every turn's reduced transcript projection.

That makes two representations of run state: the logs (`run.jsonl`, `sessions/*/*.jsonl`) and this file. The design record says there is one: `ephemeral/research/api/API.md:332` ("from then on the run is served the way every past run is: from its log") and `:841` ("a live watch and a replay are the same code"). #130 asked for continuous server-side reduction and a snapshot for the live SSR path; it did not ask for finished runs to be read from a frozen file instead of their logs. PR #138 added that.

Consequences:

- A finished run shows what the reducer knew the day it ended. Any change to the fold (scope times, turn placement, per-turn usage in #157) is invisible on every existing run. Both current proposals for #157 carry a "no shim for old checkpoints" caveat because of this.
- The name `observation.json` says nothing about what it is.
- It is one file holding every transcript, loaded whole for any view of a past run.
- Nothing reads it except the registry.

Saved example: `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/` has an 11 KB `observation.json` beside 157 KB of logs.

## Ask

Propose and implement a better solution. The direction the design record already sets:

- The log is the one representation. A finished run is reduced on request through the same `Store.Lifecycle` and `Store.Event` the live path uses, from `run.jsonl` and `sessions/*/*.jsonl`. The log's `complete` record marks the end. `internal/runlog/reader.go` exists and #130 asked for its role to be resolved; this is where.
- Delete `checkpoint.go`, the write at `Store.Close`, `loadCheckpoint`, and the file.
- If reducing a large run on open measures as slow, cache the reduced snapshot in memory in the registry. A cache is rebuildable and deletable; it is not a second source.

Decide and state: whether the in-memory store for a live run and the on-request reduction of a finished run share one code path (they should), and what the registry holds after a run ends.

## Proof

- Open a finished run whose logs predate the change and see it rendered from its logs with whatever the current fold produces.
- Delete nothing but confirm no `observation.json` is written by a new run.
- The live path (SSR snapshot, then SSE stream) is unchanged and shown working on one cheap-tier run.

## Relation to #157

#157 (and its rewrite as the single coverall issue) should treat this as edit zero: it removes the old-checkpoint caveat and the "checkpoint alone" proof step from that work.

