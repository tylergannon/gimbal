# Scoped context and runtime artifacts: proposal

Status: proposed design. Only the scope-isolation regression test is implemented.

## Desired result

Agents receive a bounded view of the values visible from the current scope,
with readable previews and absolute paths to complete data. Large values and
command output have durable files under the run. Leaving a child scope restores
the parent's visible values and budget; it does not delete the child's record.

These are three related responsibilities: artifact storage, prompt budgeting,
and bounded command capture. An artifacts directory supports the other two,
but moving output to disk alone does not bound prompts or returned Go strings.

## What exists

- `scope.go` marshals each Set/SetJSON value into a scope-local byte slice and
  emits the complete value as ValueSet. Rendering walks ancestors only, with
  the nearest definition winning. Closing removes the scope from the live
  registry, but does not explicitly erase its values.
- `session.go` appends rendered context in Generate. WithScopeTemplate receives
  decoded values as well as their text, so templates can select JSON fields.
- `loop.go` supplies scope context and the preceding task's local record to the
  planner. That intentional feedback is distinct from leaking child values into
  a parent's scope.
- `command.go` buffers both output streams fully in memory, returns full strings,
  and only afterward writes streams larger than 64 KiB into `commands/`.
- The existing scope-data test deliberately reads an ended child's context.
  Parent visibility and destroying all references to a child's memory are
  different contracts. This proposal guarantees the former.

## Artifact storage

Use one runtime-owned tree, preserving scope nesting and instance ordinals:

```text
runs/<run-id>/artifacts/recon/1/
  research.json
  output.txt
  vet/1/stdout.log
  vet/1/stderr.log
```

Set-once keys already have uniqueness within a scope instance. Command names
need their own ordinal because a call site can run repeatedly. Random suffixes
are unnecessary for identity; temporary files used for atomic publication can
have random names. Encode path components without collisions and enforce run
containment; names and keys must not become arbitrary filesystem paths.

Large strings become readable UTF-8 text files; structured values retain valid,
complete JSON. Keep an internal descriptor with the path, original size, type,
and a bounded head/tail preview. Publish a reference only after the file is
successfully written. A failed write fails the operation/run explicitly: it
must not silently lose data, publish a broken pointer, or fall back to retaining
unlimited bytes. Set has no error return today, so the implementation must
settle how an artifact write error reaches Run before changing storage.

Use run-relative paths in durable records and absolute paths in agent prompts.
Artifacts survive scope closure for review. Value records and the web reader
must understand file-backed values; do not retain another full copy in the
live observation store just because the scope map now holds a descriptor.

## Context budget

Starting policy: T = 15,000 tokens for the automatically supplied scope context,
with a provisional per-entry ceiling of 4,000. Use
[tiktoken-go/tokenizer](https://github.com/tiktoken-go/tokenizer), with its
`o200k_base` encoding and `Count` method, as one common counter across harnesses.
It is pure Go with embedded vocabularies, so counting needs no runtime download.
Treat that count as a practical approximation for other providers. Tyler's
tolerance is roughly 50%; exact model matching is unnecessary. Count the whole
render, including headings, references, and previews. These limits cover added
scope context, not the provider's entire conversation history.

At Set/SetJSON, snapshot once and spill a value that exceeds the per-entry
ceiling. Small entries can remain inline. Aggregate overflow is handled for
the visible scope chain, not all values anywhere in the run.

At prompt assembly:

1. Resolve inheritance and shadowing first. Only visible values count.
2. Keep small values intact. Share the remaining budget among large values by
   lowering a common ceiling until the complete rendered context fits.
3. Spill any newly abbreviated value. Render a clearly labeled head/tail
   excerpt, an omission marker, and the absolute full-file path. Preserve UTF-8
   boundaries. A JSON excerpt is labeled as an excerpt, never as complete JSON.
4. If even the keys and references exceed T, write the full context index to a
   file and return a bounded preview and its path. No entry becomes unreachable.
5. Measure the final render before dispatch. A child can use a shorter view of
   an ancestor's value, but cannot mutate that ancestor's value or its rendering
   allowance. Parent and sibling views are computed independently.

This combines per-object checks at storage time with a total check where the
actual prompt is known. We need not rewrite all stored values on every Set.
Persisted files are immutable; representation can vary per prompt.

WithScopeTemplate needs an explicit treatment: preserve typed `.Value` access,
then budget its final rendered output. If it exceeds T, store the complete
render as an artifact and supply an excerpt plus path. This preserves template
field selection, though full JSON decoding can still require transient memory.
The context budget does not claim to bound arbitrary template computation.

Apply the same budgeting to runtime-generated planner context, including the
previous task record; it must not bypass the policy through localText. Explicit
workflow instructions and provider conversation history are separate budgets.
Once an agent has seen child data, leaving that scope cannot erase its native
conversation history. The regression proves subsequent injected context only.

## Command capture

Stream stdout and stderr to separate artifact files while the process runs.
Keep only bounded previews in memory, retaining partial files on failure or
cancellation and propagating capture errors. Record paths and previews so the
existing run reader can retrieve full output.

Keep the scalar return shape. Small results remain complete strings; large
results return head/tail excerpts with an omission marker and the absolute
full-file path in the text. Agents can follow that reference. No caller
migration project or further return-contract decision is needed for this pass.

Read only a bounded prefix and suffix from each file, using seek/ReadAt or a
similarly simple helper. Small files are read once, without duplicated overlap.
Do not read complete large files back into memory to produce their previews.
Use byte limits for those reads and the token counter for the resulting text;
do not tokenize an entire command log merely to preview it.

## Delivery and evidence

1. Establish scope isolation. The added unit test captures actual Generate
   prompts in the parent, child, parent again, and sibling. It covers Set and
   SetJSON, shadowed and child-only keys, default and custom rendering, and
   successful/error child exits. Parent and sibling prompts must exactly equal
   the original parent prompt. No production changes are required for this.
2. Add runtime artifact storage and file-backed scope values together with
   record/reader support. Test exact file contents, uniqueness, containment,
   write failures, and references readable after the child ends.
3. Add per-entry and total context budgets. Test one large value, many small
   values, deep inheritance, shadowing, sibling independence, template output,
   planner feedback, Unicode, and the too-many-references fallback. Assert the
   measured complete render fits T and omitted content remains recoverable.
4. Stream command output and return previews with file references. Test both streams,
   very large output, partial output on cancellation/nonzero exit, capture
   failures, bounded retained buffers, and full file fidelity. Do not infer a
   memory bound merely from an artifact's existence.
5. Run the real workflow with Codex gpt-5.6-luna: make context exceed T, observe
   the actual prompt, have the agent read omitted middle content from a file,
   and inspect command artifacts while output is still streaming. Report what
   was seen in chat/PR; commit no proof programs, screenshots, or run logs.

No summarizing model, vector index, configurable eviction framework, new
workflow wrappers, or public artifact API is needed to establish this behavior.
