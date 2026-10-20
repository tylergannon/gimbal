# Independent spec review: durable Interactive execution (round 02)

Reviewer: Claude Opus 5.5, independent adversarial pass. Read-only apart from
this file.

## Target

`ephemeral/research/durable-interactive/SPEC.md` at `13ee0140`
(`docs: complete durable Interactive spec with ending provenance`), together
with its `SOURCE-MAP.md` and the author's `REVIEW.md`.

The launch prompt asked for an independent review against the repository
instructions and the relevant current source. It did not narrow scope,
predict findings or ask for a verdict, so no caller instruction was ignored.

## Evidence inspected

- Standards: `spec-writing/SKILL.md` and `notation.md`, `spec-review/SKILL.md`,
  `adversarial-review/SKILL.md`.
- Repository instructions: `AGENTS.md`, in particular "The rule", "No
  wrappers", "Build what was asked", "Proof is what you saw" and "Working
  here".
- Spec set: `SPEC.md` (548 lines), `SOURCE-MAP.md`, `REVIEW.md`, and the git
  history of the spec directory (`a93fe1f8`..`13ee0140`).
- Prior art: `/Users/tyler/src/gimbal-token-cache/index/README.md` and the
  DBOS facts the source map cites (version gating and fencing limits).
- Current source: `interview.go`, `run.go:363-411`, `runstore/store.go:40-90`,
  `harness.go:20-60`, `session.go:410-540,759-800`,
  `internal/runner/launch.go`, `internal/runner/runner.go:30-120`,
  `internal/invocation/invocation.go`, `internal/host/routes.go:95-140`,
  `registry.go:128-169`, `cmd/gimbal/main.go:304`,
  `internal/conversation/manager.go:210-240`, `events.go:159-176`,
  `internal/live/live.go:19`, `web/runview/list.go`,
  `internal/devgenerate/registry.go:34-36`, `internal/workflows/**` and
  `cmd/examples/**`, plus `go doc -short .`.
- Mechanical recount of every cell baseline using the spec's own rule
  (handwritten production source; exclude `_test`, `testdata`, generated
  suffix or a `// Code generated … DO NOT EDIT` header).

## Pass summary

| Pass | Result |
| --- | --- |
| Consistency | Defects. The interaction address is defined per call occurrence, but the ingress is required to span occurrences. The agent-end boundary says undelivered input is terminal and also that it carries forward. The disposition enum does not match the prose (F1). The freedom to pick either initial-entry mode contradicts C6 (F2). |
| Claim traceability | Mostly sound: every cell routes at least one claim and every external seam has a live claim. Two gaps: no cell or claim covers the existing workflows that C12's admission rule would reject (F3), and no claim covers what happens to a dormant run when the code changes (F4). |
| Seam audit | All six cells state a responsibility, boundary, contract and budget, and the Parnas reasons differ. The A baseline does not reproduce under the spec's own counting rule, and several paths the change must touch belong to no cell (F5). No merge or split candidate found. |
| Claim classification | 12 cell-local and 4 integration claims. The labels and routing are plausible. The decomposition is not integration-dominated. |
| Derivability | Spanning failures at the J/I/P ingress seam (F1), the I contract on entry (F2), the A admission contract versus the existing library and workflows (F3), and the E/H code-identity seam (F4). Elision candidate: the illustrative file tree (lines 114-134) repeats the ownership sets in lines 141-152 and could drift from them. |
| Pump hygiene | Satisfied. R1 (EndedBy) entered the public contract after explicit human approval. The other amendments in the directory history trace to recorded human decisions. No reviewer opinion became public API. |

## Findings

### F1 — critical — The inbox address is per call occurrence, but the ingress must span occurrences, and the agent-end boundary contradicts itself

Passes: consistency, derivability. Location: `SPEC.md:233-243`,
`256-258`, `209-217`, `315-322`, `329-330`, C6/L4 (`479`, `489`).

The spec defines `InteractionAddress = (run, interaction: OperationIdentity)`.
It also says identity "uses the existing run/node naming discipline,
including parentage and authored occurrence" (`256-258`). In the current code,
each call to a named node takes the next slot ordinal:
`scope.nextNode(runstore.Question, name)` (`run.go:368`) fills
`Slot{Parent, Kind, Name, Ordinal}` (`runstore/store.go:53-59`). The
conversation workflow calls one `Interactive` call site in a loop
(`SPEC.md:196-207`), so every iteration has a different `OperationIdentity`
and therefore a different inbox address.

The spec nevertheless requires:

- "The conversation's durable ingress remains usable while its workflow moves
  between these call occurrences" (`214-215`);
- pending input "follows the durable ingress into the next call occurrence
  without changing its identity or order" (`320-321`);
- "A pending human finish still reaches the next call" (`329-330`).

A browser message sent after an agent end, while the workflow is between
calls or dormant after a restart, must be addressed to an occurrence that has
not yet been allocated. Under the only address the spec defines, P cannot
name a destination, and J cannot accept the input before E reaches the next
call.

The boundary rule then contradicts itself in adjacent sentences. "Agent end
also records a terminal boundary. Inputs not delivered before a terminal
boundary remain durably recorded and visibly unprocessed" (`317-319`). The
next sentence says such inputs carry forward into the next call (`320-321`).
Only the finish boundary says what happens to later messages: they are
rejected as closed (`315`). For agent end, the spec does not say whether
later messages are accepted, rejected or held. That leaves two options, and
each breaks something:

- If input is closed after agent end, the conversation loop of C6 and L4
  breaks.
- If input stays open after agent end, a plain workflow that calls
  `Interactive` once and then continues accepts messages that nothing will
  ever consume. They sit pending forever and the UI shows them as awaiting
  delivery.

The `disposition` enum (`243`) also has `superseded`, which no prose
defines. It has no value for "terminal, visibly unprocessed" or for
"carried to the next occurrence", which the prose requires.

Impact: two capable implementers will build incompatible J records and P
routes. One addresses a run-level or call-site-level ingress; the other
addresses the occurrence and drops or strands input across loop iterations.
C2, C6, C8, L3 and L4 cannot be judged against a single contract.

Proposed amendment (HITL): name the address that outlives occurrences. One
option is run plus authored call-site path without the ordinal. Another is an
explicit per-run interaction ingress that each occurrence consumes from. State
which occurrence consumes an input, by acceptance order relative to each
occurrence's start and end. State the disposition of input accepted when no
further occurrence will run, for example "closed when the run reaches a
terminal state". Then reconcile the enum with the prose: define or remove
`superseded`, and add the terminal-unprocessed value the prose relies on.

### F2 — critical — Leaving initial entry open contradicts C6: agent-first entry makes the conversation loop spin

Passes: consistency, derivability. Location: `SPEC.md:209-222`, `534-536`,
C6 (`479`), L4 (`489`); `interview.go:51-53`.

The spec delegates initial entry: "choose the simplest correct behavior.
Existing immediate agent turns are available as a starting point, but this
spec does not require either entry mode" (`219-222`). It also says there is
no mode machinery, and that `Interactive` keeps the existing arguments
`(ctx, name, session, purpose)` (`165-169`).

Today's `Interview` is agent-first: its first action is
`dispatch[interviewDecision](…)` (`interview.go:52`). Suppose an implementer
takes that starting point. The conversation loop would then run: agent turn,
`end`, return with EndedBy `agent`, loop, `Interactive`, a new agent turn,
`end`, and so on. Those are autonomous repeated turns, which L4 forbids ("An
agent end leaves the conversation available … without autonomous repeated
turns") and which C6 forbids ("re-entry waits or uses pending input instead of
spinning"). Every occurrence has identical arguments, so it can enter
agent-first initially and human-first on re-entry only by inspecting durable
state about earlier occurrences. That inspection is the mode machinery the
spec excludes (`519`, `534-536`).

So the choice is not free. Only human-first entry for every occurrence
satisfies the claims as written. That changes behavior for the maintained
`internal/workflows/interview` example, which A must keep as Interview's
replacement (`422-424`): the person, not the interviewer, opens the
conversation. Neither the spec nor REVIEW.md records this consequence.

Proposed amendment (HITL): either state that `Interactive` waits for human
input first on every occurrence, and accept the change to the interview
example, or state how an occurrence decides its entry, along with the
argument or durable fact it uses. Remove "does not require either entry
mode" in both places.

### F3 — critical — The authoring contract outlaws what every maintained workflow does, and the library offers no replacement

Passes: claim traceability, derivability, seam audit. Location:
`SPEC.md:79-90`, `422-427`, C12 (`485`), exclusion `509-510`, ownership
`141-152`; AGENTS.md "No wrappers".

The spec forbids "direct file reads or writes, clocks, randomness, and
equivalent operations outside the library … in workflow functions" and in
"their source-visible helpers. A helper is not an escape hatch." It also
forbids any Activity mechanism. C12 requires these to be rejected.

The current maintained workflows do exactly these things in bodies and
helpers:

- `internal/workflows/sprintplan/files.go:19,34,45,53`: `os.ReadFile`,
  `os.MkdirAll`, `os.WriteFile`.
- `internal/workflows/validateproduct/validateproduct.go:94,122,130,197,275,314`
  and `config.go:40,89,114`: `os.MkdirAll`, `exec.CommandContext`,
  `os.WriteFile`, `time.Now`, `os.Stat`.
- `internal/workflows/implementation/implementation.go:72`: `os.ReadFile`.
- `internal/workflows/researchdocument/researchdocument.go:172,175,221,224,367`
  and `internal/workflows/pyramidsummary/pyramidsummary.go:121,541`.

The public library (`go doc -short .`) has no file read, file write, stat or
clock operation. Its only effect operations are `RunCommand`, `Check` and
`Service`.

The spec does not say how these programs become admissible. Readers will
diverge three ways:

1. Add exported library operations such as file I/O and a clock. AGENTS.md
   forbids this unless Tyler asks by name, and the spec does not approve
   any.
2. Rewrite every workflow so each file read becomes `RunCommand` with `cat`,
   and so on.
3. Apply the admission rule only to workflows that reach `Interactive`.

`internal/workflows/**` and `cmd/examples/**` belong to no cell, carry no
budget, and are covered by no claim. Also undecided: whether pure but
nondeterministic Go constructs are admitted. Examples are `range` over a
map, raw `go` statements and channel `select` in a workflow body. They
are "ordinary Go control flow" (`84-85`), but they break replay determinism.

Impact: C12 either fails at merge, because the stock workflows are
rejected, or passes only through an unapproved public API or an unbudgeted
rewrite of about ten programs. The replay contract in cell E (`354-358`)
holds only if this admission boundary is decided.

Proposed amendment (HITL): decide the scope of admission, either all
workflows or replayable ones. Name the library operations authors use for
file reads and writes, stat and time, or state that these go through
`RunCommand`. Assign `internal/workflows/**` and `cmd/examples/**` to a
cell with a budget, and add a claim that the maintained workflows are
admitted after migration. State whether map iteration, `go` statements,
channels and `select` in workflow code are rejected or made deterministic.

### F4 — issue — "Same code identity" is undefined, so dormant conversations may not survive a Gimbal rebuild

Passes: derivability, claim traceability. Location: `SPEC.md:346-352`
(`workflowAndCode : SameCodeIdentity`), `379-383`, `395-399`, exclusion
`511-512`, L1 (`486`).

Today every hosted run is a fresh re-exec of the current binary
(`internal/runner/launch.go:21-36`). Each run loads `.so` plugins at start
(`internal/runner/runner.go:30-31`, `registry.go:148-150`), and nothing
records code identity. The conversation runtime survives restarts and
rebuilds because it replays nothing. It marks in-flight requests unresolved
and lazily resumes the native session (`internal/conversation/manager.go:216-233`).

The spec turns each conversation into a replayed workflow. It requires
enough code identity to refuse an incompatible resume, and it excludes
changed-code replay. It never says what that identity is:

- If identity is an executable or plugin digest, as in the DBOS prior art
  the source map cites, every `just build` or release turns every dormant
  conversation tab into an "actionable recoverable state". That would be a
  regression in tab continuity for a project that rebuilds constantly.
- If identity is the workflow graph or source digest, a conversation's
  identity survives rebuilds that do not touch it, but library changes can
  change replay without detection.

The shared Go host (`395-399`) makes this sharper. Go plugins cannot be
unloaded or reloaded, and a process cannot hold two builds of one plugin
path. A long-lived shared host therefore cannot run a dormant run's old
code beside new admissions of the rebuilt code. That decision falls between
H (placement) and E (reconstruction), and neither contract settles it.
L1 restarts the host but never changes the code, so no claim exercises the
case.

Proposed amendment (HITL): define `SameCodeIdentity` and its granularity.
State the product behavior for a dormant conversation after a Gimbal or
workflow rebuild. State whether activations with different code identities
may share a host process. Add a claim that exercises a rebuild between
suspend and resume, even if the required outcome is "visibly unresolved".

### F5 — issue — The A baseline does not reproduce, and paths the change must touch belong to no cell

Passes: seam audit. Location: `SPEC.md:98-105`, `136-158`.

Recounting with the spec's rule reproduces J 913, E 3,469, I 102, H 4,288
and P 4,532 exactly. A measures **6,248**, not 5,881. The difference, 367
lines, is exactly `internal/generate/mermaid.go`. That file is handwritten
generator source with no generated header. It contains "Code generated" only
inside an emitted string at line 15. Other generator files (`definition.go`,
`source.go`, `stock.go`, `workflow_page.go`) contain the same phrase in
strings and were counted. So the stated rule ("a Code generated header") was
not the rule that was applied, and the mechanical check is ambiguous at
baseline.

With the correct baseline, A has 252 lines of headroom under 6,500. In that
space it must add Interactive extraction and diagrams, plus the new
admission analysis for effects and nondeterminism in bodies and helpers.
`internal/gimballint/analyzer.go` alone is 904 lines today, and F3 may add
the stock-workflow migration. A will breach almost immediately.

The spec also says "necessary changes outside these sets must be assigned
and budgeted before implementation" (`157-158`). These Interview-dependent
paths are already known and belong to no cell:

- `events.go:159-176`: the public `InterviewQuestionAsked` and
  `InterviewQuestionAnswered` lifecycle events, which the spec requires
  removing (`187-188`).
- `internal/live/live.go:19`: the live `AnswerInterview` controller path.
- `web/runview/list.go:33-163`: interview attention rows.
- `internal/devgenerate/registry.go:34-36`.
- `internal/workflows/interview/` and `internal/workflows/implementinterview/`.
- `cmd/examples/interview/` and `cmd/examples/implement-interview/`.

Proposed amendment (HITL): define "generated" as the Go convention, a
`^// Code generated .* DO NOT EDIT\.$` line before the package clause. Restate
A as 6,248 and re-decide its maximum given F3. Assign each path listed above
to a cell now, for example `events.go` to E or I, `internal/live` and
`web/runview` to P or H, and the workflows and examples per F3.

## Outcome

material findings remain
