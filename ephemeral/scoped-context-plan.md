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

This is storage for the run where it was created. A run is not relocatable and
cannot be resumed after moving it. Nothing in this work adds either behavior.

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
unlimited bytes. `Set` and `SetJSON` have no error return, so a failed artifact
write panics immediately, as a failed value encoding does today. The run records
the panic through its existing panic path when recording is still available,
then lets it escape. There is no sticky-error mechanism and no public API change.

Structured artifact fields in durable records use run-relative paths. Agent
prompts and scalar command returns use absolute paths. A recorded prompt remains
the exact prompt sent to the agent, including its absolute path; this is not a
portability promise. Readers resolve structured references against the run's
existing directory. Artifacts survive scope closure for review. Value records
and the web reader must understand file-backed values; do not retain another
full copy in the live observation store just because the scope map now holds a
descriptor.

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

For an ordinary Generate, T covers the scope context Gimbal appends, not the
caller's prompt. For a planner turn, the same policy covers the dynamic context
Gimbal assembles for that turn, including visible scope values and the previous
task record. Fixed workflow/planner instructions and provider conversation
history are outside this budget. This is not general context-window management.

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

Every started command streams both output channels to its run-owned files; this
is what makes output inspectable while the command is still running. The files
remain with the run. A capture failure is returned through RunCommand's existing
error result and recorded as a run failure; it does not publish a successful
reference to incomplete or missing storage.

Keep the scalar return shape. Small results remain complete strings; large
results return head/tail excerpts with an omission marker and the absolute
full-file path in the text. Agents can follow that reference. No caller
migration project or further return-contract decision is needed for this pass.

Read only a bounded prefix and suffix from each file, using seek/ReadAt or a
similarly simple helper. Small files are read once, without duplicated overlap.
Do not read complete large files back into memory to produce their previews.
Use byte limits for those reads and the token counter for the resulting text;
do not tokenize an entire command log merely to preview it.

## Implementation sequence

1. Add one internal artifact substrate: contained collision-free paths, atomic
   publication, immutable descriptors, and bounded UTF-8 prefix/suffix reads.
   Do not add an exported artifact API.
2. Put large Set/SetJSON snapshots on that substrate and change the lifecycle
   record, live observation store, finished-run tables, and log rebuild together
   so a file-backed value has one meaning everywhere.
3. Add the per-entry and aggregate context budgets after inheritance and
   shadowing have resolved. Then apply the same final-render check to
   WithScopeTemplate and the planner's runtime-generated context.
4. Stream RunCommand output through the same artifact substrate. Preserve exact
   small scalar returns and produce bounded large returns without reading a
   complete large file back into memory.
5. Run the repository gates and the one live acceptance run described below.

## Focused tests

Keep this to roughly seven focused test functions on the branch, including the
scope-isolation regression that already exists. Use table-driven subtests rather
than creating one test for every example below, and add no test framework or
proof program.

1. The existing parent/child/restored-parent/sibling prompt isolation test.
2. Artifact path containment, collision-free encoding, atomic publication, and
   the decided panic on write failure.
3. One file-backed scope value through the live reader, ended scope, finished
   table read, and missing-table log rebuild, with exact file contents.
4. One table-driven default-render budget test covering a large entry, aggregate
   overflow, inheritance and shadowing, sibling independence, Unicode, and the
   too-many-references index fallback.
5. One test covering final WithScopeTemplate output and PromiseLoop's previous
   task record so neither bypasses the budget.
6. One command test covering small and large stdout/stderr, visible file growth
   while running, bounded returned previews, and byte-for-byte file fidelity.
7. One abnormal-command test covering nonzero exit, cancellation, partial
   output, capture failure, and directly asserted bounded preview state. Do not
   infer a memory bound merely from the existence of an artifact.

If the implementation wants materially more tests, stop and explain which
uncovered behavioral seam requires them instead of expanding the matrix.

## Definition of done

- Every ordinary Generate receives only values visible from its current scope,
  with nearest-scope shadowing. Returning successfully or with an error from a
  child restores the parent's original rendered context and budget, and a
  sibling sees no child-only value. This holds for default and custom rendering.
- The complete Gimbal-added context render is at most T according to the common
  o200k_base counter. A value over the per-entry ceiling and any value abbreviated
  to satisfy the aggregate ceiling has a labeled head/tail excerpt, omission
  marker, and usable absolute path. If references alone exceed T, a bounded
  index preview points to the complete index. No omitted entry is unreachable.
- Large strings remain readable UTF-8 text and structured values remain complete
  valid JSON. Their artifacts are immutable, contained by the run, and readable
  after their scope ends. Live observation, finished table loading, and log
  rebuild agree on the descriptor without retaining another full payload.
- WithScopeTemplate preserves typed `.Value` access before its final output is
  budgeted. PromiseLoop's runtime-generated context, including the preceding
  task's local record, cannot bypass the same policy.
- Every started RunCommand writes stdout and stderr to files while it runs. Small
  returns are exact. Large returns are bounded head/tail excerpts with an
  omission marker and absolute full-file path. Nonzero exit and cancellation
  retain all bytes captured before the process ended, and command capture never
  accumulates a complete large stream in memory.
- An artifact write failure never creates a broken reference or an unlimited
  in-memory fallback. Set/SetJSON artifact failure panics immediately;
  RunCommand capture failure returns an error. Both use the run's existing
  failure-recording path without promising that a broken filesystem can record
  its own failure durably.
- The workflow-facing API gains no artifact primitive, wrapper, relocation, or
  resume behavior. Godoc and the generated event/schema code describe the
  changed RunCommand return semantics and file-backed record representation.
- The focused tests above and the existing repository build, test, race, vet,
  lint, and generation gates pass sequentially.

## Evidence

The Go tests are the primary evidence for this backend change because they cross
the real Run, prompt assembly, filesystem, subprocess, observation, and replay
boundaries. No new browser scenario is required.

Run one real workflow with Codex gpt-5.6-luna. Make the context exceed T, put a
sentinel only in omitted middle content, and observe the agent follow the
absolute path and report that sentinel. Separately observe a command artifact
growing while its command is still running. Report the exact behavior seen and
the proved commit in chat or the PR. Commit no proof program, screenshot, run
log, or other proof artifact.

No summarizing model, vector index, configurable eviction framework, new
workflow wrappers, or public artifact API is needed to establish this behavior.
