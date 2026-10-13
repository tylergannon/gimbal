> Recovered from the session log; the original `ephemeral/reviews/202610130700-promises-ledger-plan-round-01.md` was git-excluded and was deleted with the worktree.

# Adversarial review: promises-ledger design plan, round 01

## Review target

`ephemeral/promises-ledger/plan.md` at commit `c5e365cc`, reviewed against its
section 0 authoritative request and the repository instructions.

## Evidence inspected

- The complete plan and its `main...HEAD` diff.
- `AGENTS.md`, `docs/definition-of-done.md`, `docs/web-app.md`,
  `ephemeral/research/svelte-idioms/dos-and-donts.md`, `go doc -all .`, and the
  API design/sprint records named by `AGENTS.md`.
- Current Gimbal code cited by the plan, including `harness.go`,
  `internal/binding/binding.go`, `claude/claude.go`,
  `internal/conversation/manager.go`, `internal/host/host.go`,
  `internal/transcript/{store,durable}.go`, the workspace remotes and Svelte
  components, and the native recorder/controller call path.
- `/Users/tyler/src/promises-research/notes/{clarity-claims-ledger,gimbal-integration-points,clarity-export-and-live}.md`.
- The referenced Clarity snapshot at
  `/Users/tyler/src/promises-research/token-cache/clarity`, especially the
  review provider validation/instructions, deliberation cadence, and hosted Go
  review application.

## Findings

### 1. Critical — untrusted transcript text is handed to a fully tooled, bypass-permission coding agent

The plan concatenates finalized speech into the same prompt sent to a fresh
Claude Code session (`plan.md:192-204,242-245`) and chooses Claude Sonnet as the
default (`plan.md:198-202`). It then acknowledges that this harness runs with
bypass permissions, but claims that an empty working directory plus the prompt
sentence “Do not use tools” bounds it (`plan.md:343-350`). That claim is false.

The current Claude adapter explicitly starts sessions with the CLI's own tools
available (`claude/claude.go:70-95`) and sets `PermissionModeBypassAll` plus
`AllowDangerouslySkipPermissions` (`claude/claude.go:144-150`). A working
directory is not a filesystem sandbox; the proposed directory is inside the
project (`<project>/.gimbal/promises/work`), and built-in file/shell tools can
walk to the repository or elsewhere. JSON-encoding a transcript also does not
create an instruction boundary. A recording can therefore contain speech such
as “ignore the ledger instructions and use Bash to …”, and the background
process is authorized to act before returning schema-valid output.

Impact: marking or creating a requirements recording grants its content an
unattended path to read or mutate local files and run commands. This is a
critical antipattern in the primary user path, not a hypothetical style concern.
The plan needs an inference path that cannot use coding-agent tools, or an
enforced sandbox/tool denial; prompt text is not a security control.

### 2. Critical — the claimed atomic/idempotent batch transaction loses its guarantee on save errors

The plan's commit phase mutates the live ledger with `apply(ops)`, advances
`Cursors[id]`, clears state, and only then calls `save` (`plan.md:177-180`). It
asserts that one atomic rename is the entire crash-idempotency story
(`plan.md:98-103,182-186`), but defines failure handling only for adapter,
decode, and timeout failures (`plan.md:188-190`). The test plan likewise covers
provider failure and reopening, but not a failed durable write
(`plan.md:317-328`).

`transcript.WriteJSON`, which the design reuses, can fail before the rename,
during it, or after the rename while opening/syncing the directory
(`internal/transcript/durable.go:95-125`). If it fails before rename, the
in-memory entries and cursor have already advanced even though disk has not;
the next tick can skip the batch, and the apparent ledger changes disappear on
restart. If it fails after rename, the commit may already be present even
though the caller received an error, so a blind rollback/retry can duplicate
adds. Atomic replacement protects file shape, not the in-memory/durable commit
decision.

Impact: the design does not establish its own done criterion that restart
preserves the ledger without reprocessing (`plan.md:45-47`). The commit must be
defined over a copy and publish in-memory state only after a confirmed write,
with explicit reconciliation for an indeterminate post-rename error, and this
failure boundary needs a test.

### 3. Issue — structural schema validation permits promises that violate the required record and permits fabricated “verbatim” evidence

Section 0 requires every promise to have a details paragraph
(`plan.md:5-11`). The output schema only says `title` and `details` are strings,
with no non-empty constraint (`plan.md:255-266`). Application skips an `add`
only for an empty title and does not reject empty details; `update` is not even
specified to reject an empty replacement title (`plan.md:271-279`). A
schema-valid model response can therefore create a promise without details or
erase an existing promise's title/details, and the planned tests do not cover
either case (`plan.md:317-328`).

The same semantic-validation hole makes the added audit UI misleading. The
model supplies a supposedly verbatim quote, but an out-of-range chunk is
silently changed to the batch's first chunk and “Evidence never invalidates an
op” (`plan.md:242-244,271-273`). Nothing checks that the quote occurs in that
chunk. The UI nevertheless presents these values as evidence links
(`plan.md:302-308`). By contrast, the cited Clarity provider rejects missing or
out-of-range evidence references before application
(`ResponsesClaimReviewProvider.swift:100-138`) and instructs exact source
coordinates (`:298-300`).

Impact: the core ledger can contain structurally valid but requirement-invalid
entries, while the extra “audit trail” can falsely attribute model-generated
text to a recording. The lean request explicitly rejects proof machinery
(`plan.md:16-20`); if coarse provenance is retained, it must at least be derived
from and checked against the actual transcript rather than presented as
unverified model testimony.

### 4. Issue — fresh per-batch sessions discard the repository's required cleanup ownership

Each batch creates a new native session and merely defers `Close`
(`plan.md:192-194`). No behavior is defined for a `Close` error, no session is
retained for retry, and the promises worker is not integrated with an owner
that can finish cleanup later. This contradicts the current adapter contract:
failed remote cleanup remains owned and retryable (`harness.go:55-58`), and the
repository's definition of done repeats that unverified cleanup cannot be
treated as cessation (`docs/definition-of-done.md:42-50`). The conversation
manager cited as precedent owns its active sessions and closes them during
manager shutdown (`internal/conversation/manager.go:464-490`); the proposed
worker has no corresponding ownership path.

Impact: a transient close failure can leak a native Claude session/process on
every processed batch, while later batches keep creating more. The plan must
either retain failed session identities until close succeeds or use an
existing owner that already fulfills the adapter lifecycle contract.

### 5. Issue — the “existing recording” flow stops at the newest 25 recordings

The authoritative request requires a button on an existing recording
(`plan.md:12-15`), and the plan's own demo calls for opening an older ordinary
recording (`plan.md:45-46`). Current `DurableStore.List` returns at most 25
headers and exposes the continuation in `History.Next`
(`internal/transcript/durable.go:239-270`). The current workspace always asks
for `after: ''` and renders only that first page
(`web/src/lib/workspace/ProjectWorkspace.svelte:26-31,149-150`).

The plan adds the button only inside an already-open transcript
(`plan.md:310-313`) and does not add history pagination, a load-more path, or
another way to find older recordings. Its worker instruction says only “Page
`recordings.List(after)`” without defining traversal, and neither the tests nor
the real-UI proof use more than 25 recordings (`plan.md:162-186,317-341`).

Impact: once a project has 26 recordings, the older ones are not reachable
through the proposed UI, so the required declaration button cannot be used for
them. If the worker also implements only the first ambiguous page, even a
directly linked/marked older recording will never be ingested. Both UI
reachability and complete worker traversal need explicit acceptance coverage.

## Outcome

**material findings remain**
