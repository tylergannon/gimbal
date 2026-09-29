# Exhaustive Jev checks for research-document's semantic index

Proposal, 2026-09-29. No workflow changes are implemented by this document.
Repository baseline: `f276674347930bffc77022896f00a3bc4a762281`.

## Recommendation

Write a compiled Go command, `gimbal audit-index`, that reads the collected
sources and semantic index, checks every indexed claim with Jev, compares every
pair of claims, and annotates every occurrence of a disputed claim. Invoke it
with the existing blocking `RunCommand` after index creation and after every
index repair, before the author uses that index revision.

Use the existing curator to enumerate and repair claims. Let ordinary Go own
file resolution, scheduling, completeness, request persistence, and marking.
Let Jev make small typed judgments against actual source text. Keep the
existing independent editor responsible for the final document.

This needs no new exported Gimbal API, workflow language construct, agent
harness, or named high-level workflow wrapper. Compile the command into the
same Gimbal executable, as `count-tokens` already is. A separate executable is
also possible, but adds installation/version alignment without improving the
audit. This is a synchronous command, not `gimbal run audit-index`: hosted
workflow submission would return before the audit finishes.

The acceptance promise is **every claim examined, every claim pair examined,
all unresolved findings visible at their occurrences**. A model judgment is
not a guarantee of factual truth. The useful source verdict is “supported by
these collected passages at these versions,” not “universally true.”

## What exists today

The current [research-document source](../../../internal/workflows/researchdocument/researchdocument.go)
has five parallel researchers, topic indexes, a combined index, an author,
and a bounded editorial repair loop. Research prompts ask for source locations,
qualifications, and contradictions. `verifyResearchFloor` only enforces source
file counts and a nonempty topic index. After the curator finishes, the code
only requires a nonempty combined index before authoring. Gap research updates
the index and goes directly to document revision. Neither location performs a
claim audit.

[Gimbal's current Jev integration](../../../supervise_jev.go) screens exposed
thinking for supervision; it does not verify collected research. Its truncated
packets, review threshold, and cooldowns are unsuitable for exhaustive source
checking. Reuse the already selected `github.com/kazz187/jev-sdk-go v0.2.0`
dependency, not the supervision policy.

The workflow language is ordinary Go. [The graph extractor](../../../internal/generate/expr.go)
recognizes Gimbal operations and inlines local helpers; a normal external SDK
call can execute but is not a Jev graph node. `RunCommand` already yields a
named command node. Thus the gap is first-class observation, not an inability
to write this algorithm in Go.

## Jev facts that constrain the design

- TypeSafe publishes a [citation-checking cookbook](https://docs.typesafe.ai/cookbooks/citation_check):
  locate the quotation deterministically, then ask a Choice about the claim
  and its surrounding source section. Its small published example uses an
  older model and is not evidence of accuracy on our corpus.
- The [current model page](https://docs.typesafe.ai/models) lists
  `jev-1.13.0`, text-only input, 64k tokens for an entire request, and 32k for
  state plus its longest question. Published input pricing is $0.042/M tokens;
  outputs are free. Listed limits are 1,200 requests/minute and 250,000
  tokens/second, explicitly subject to change. Pin the version for evaluation.
- A [Choice](https://docs.typesafe.ai/primitives/choice) returns one label,
  a distribution, and confidence. Question IDs are not shown to the model:
  explicitly identify the claim and evidence in the instruction or state.
- Questions in a request see the same [state](https://docs.typesafe.ai/concepts/state)
  but are evaluated independently. A question cannot consume another answer
  from that same request. Dependent judgments need another request or code.
- TypeSafe documents [weaknesses](https://docs.typesafe.ai/model-jaggedness/jev-1.13)
  in numerical/date reasoning, indirection, irrelevant context, adversarial
  source text, and prose generation. Use actual excerpts, explicit scope, and
  small questions; do arithmetic in Go. Do not ask Jev to write claim records
  or explanations.
- [Confidence](https://docs.typesafe.ai/confidence) describes the distribution,
  not independently verified correctness. Choose any automation thresholds
  from labeled research examples; do not copy the supervisor's 0.65 threshold.

Official source snapshots are saved locally; [sources.md](sources.md) names
them and the exact repository integration points.

## Define the complete input before sending requests

For this workflow, audit all generated index prose: root and intermediate
routes, topic indexes, leaves, tables, annotations, and explanatory clips.
Include gap-research indexes and unreachable/orphan index files. Starting only
at reachable root links would let an orphan escape. Exclude original source
documents from *index-claim extraction*; they remain the evidence being checked.
Downloaded originals and agent-authored clips must be distinguishable by path
and file role. Include mixed clips as commentary plus separately mapped source
excerpts, not as an authoritative original.

Use an explicit index-file inventory produced by the researchers and curator;
cross-check it against files created under the research directory. A file with
an unknown role is unresolved input, not silently excluded. This is a contract
for research-document's own artifacts, not a universal importer for every
semantic-index format.

Freeze that revision while auditing. Code records hashes of the index files
and the original files used as evidence. No writer changes the index during
the audit. Before applying annotations or accepting its result, check those
hashes again. A changed input invalidates the affected verdicts.

The smallest extra input is `.semantic-index/claims.jsonl`. Each record holds:

| Field | Meaning |
| --- | --- |
| `id` | Stable local claim identity; edits also change a content digest. |
| `text` | One independently assessable assertion, retaining its qualifiers. |
| `scope` | Stated subject, version/time, conditions, modality, and measurement; unknowns stay unknown. |
| `occurrences` | Every index file and exact block/span asserting this claim. |
| `references` | Original local source paths and line/section/page spans, with optional verbatim quotation. |
| `kind` | Factual assertion, attributed assertion, inference, recommendation, or unresolved question. |

Only byte-identical assertions with identical scope may share one record;
preserve all their occurrences and citation associations. Do not use semantic
deduplication to erase a slightly different qualifier. Keep a disputed claim
in the inventory even if its source check fails.

The existing curator can produce this file in an additional extraction turn
after building the index. Code supplies every prose block with its heading,
table headers, source path, and stable span. Split compound assertions and
retain negation, “only,” “always,” units, attribution, and applicability.
Questions and pure routing instructions are not factual claims, but claims
inside recommendations and descriptions still count. For example, “use X
because it supports images” contains an independently checked factual premise.

**Extraction cannot be trusted merely because it produced JSON.** For every
block, Jev checks whether the proposed records faithfully cover all assertions
and qualifiers; a block with no records still receives this question. For each
record, check that the cited index span actually asserts it. Code checks that
all blocks and records have answers. Omission or distortion returns the exact
block to the curator for repair and re-audit. An unresolved block prevents a
claim of complete coverage. This establishes exhaustive scheduling; semantic
extraction and its model-based review remain fallible and need evaluation.

## Algorithm

### 1. Resolve references and build evidence packets

Resolve each reference against the saved source cache, never against an index
summary. Load the indicated passage plus its enclosing heading, paragraph,
table headers, footnotes, and applicable exceptions. Preserve exact text and
source/version identity. Following a clip requires its mapping back to the
original. A URL without collected local content is a missing source.

Check file existence, span bounds, source hashes, and verbatim quote presence
in code. A mismatch is `invalid-reference`, not proof that the claim is false
or that a researcher fabricated it. A paraphrase must not claim to be a quote.
Permitted whitespace normalization must preserve the mapping to original text.

Do not truncate evidence to fit a model window. Start with a bounded enclosing
section; expand into neighboring sections when definitions or exceptions are
missing. When needed, partition the referenced material into separately named
source windows and check all windows. Keep relevant counterevidence alongside
support. Do not merge unrelated material into a huge prompt to save requests.
If the necessary evidence cannot be jointly assessed, mark the claim unresolved
rather than treating the best isolated window as a full proof.

For images/PDFs, retain page-linked text extraction and the original. If a
claim depends on a chart or image not represented faithfully in text, return
`unassessable-evidence`; Jev cannot verify those pixels itself.

### 2. Check every claim against every citation

For each claim-reference association, send the claim as written and actual
source passages to Jev. The question asks what **this evidence establishes**,
without world knowledge or instructions embedded in the evidence.

| Choice | Exact interpretation |
| --- | --- |
| `supports` | Establishes the entire assertion, including its qualifications. |
| `contradicts` | Establishes something incompatible in overlapping scope. |
| `not-established` | Establishes neither the assertion nor its opposite. |
| `mixed` | The supplied passages contain both support and opposing evidence. |
| `uncertain` | Meaning, scope, or context prevents a reliable determination. |

Keep each citation verdict, including a bad citation attached to an otherwise
supported claim. A majority vote among sources is not a truth rule. An
attribution such as “vendor A reports X” may be supported even when X itself
is disputed; represent those as different assertions.

Then assess the claim against its cited evidence set together when that set
fits. This catches conflicting sources and claims requiring multiple premises.
If it does not fit, use the original supporting/opposing spans selected by the
individual checks, never just Jev's labels as replacement evidence. If combining
them requires unresolved reasoning, return `uncertain`. A second pass cannot
erase a contradictory passage by omission.

Claims without usable references also receive a Jev judgment with the missing
evidence explicit, but code forbids assigning a source-supported status to
them. All claims get accounted for; malformed references remain actionable.

Store judgment, probabilities, confidence, exact evidence identifiers, model,
and question revision. Successful evaluation means the answer was recorded,
not that the claim passed. Do not stop after the first false claim.

### 3. Compare every unordered pair of claims

For N claim records, enumerate exactly N(N-1)/2 pairs in ordinary Go. Include
cross-topic, unsupported, and source-contradicted claims. Neither embeddings,
topic assignment, nor matching subject names may discard pairs. Such signals
can prioritize likely conflicts, but the run is incomplete until the entire
pair set has results.

The initial pair packet contains the two original assertions, their enclosing
index context, explicit scopes, and measurement definitions. Ask separately:

1. Do their subjects and applicability overlap, differ explicitly, or remain
   unclear? Unknown version/time does not establish different scope.
2. Are the assertions mutually incompatible as written, compatible (including
   unrelated assertions), or impossible to decide from the packet?

Distinguish competing values of one limit from limits on different quantities.
“64k across the entire request” and “32k across state plus one question” may
both apply. Compare dates and parsed quantities in code when their meanings
are established. Different measurement names alone do not prove compatibility.
Universal statements can conflict with a specific counterexample; missing
qualifiers cannot be invented to make a disagreement disappear.

For a suspected or uncertain conflict, fetch the original passages supporting
both claims and make a focused follow-up judgment. Resolve an ambiguous entity
or metric explicitly before re-asking compatibility. Answers from the first
request become inputs only to this later request. Preserve disagreements
between passes as uncertainty; repeated model votes are not independent proof.

The output relation is one of `conflict`, `possible-conflict`,
`different-scope`, or `compatible`. Until domain calibration exists, a Jev
conflict is visibly marked as a Jev finding for editorial confirmation, not
presented as an adjudicated fact. Never pick a winner from confidence, source
count, or publication date alone.

Store each pair once using canonical ID order and link it from both claims.
Contradiction is symmetric, not transitive: A conflicting with B and B with C
does not justify an A–C conflict. This covers pairwise contradiction; it does
not prove arbitrary multi-claim logical consistency or establish that the
collected sources are themselves correct.

### 4. Mark every occurrence, preserving evidence

Code writes `.semantic-index/audit.jsonl` as completed work accumulates and
renders an `AUDIT.md` that resolves claim IDs, locations, source verdicts, and
conflict edges. The JSONL is the input to marking and resume, not an optional
second report. The audit renderer adds a compact visible marker next to every
claim occurrence, including root summaries and table cells, for example:

> **C17 — source-supported; disputed by C42** — see AUDIT.md#c17.
>
> **C42 — source-contradicted; disputes C17** — see AUDIT.md#c42.

One claim can be source-supported and conflict with another source-supported
claim. Retain those two dimensions. Mark possible conflicts explicitly too.
Never rewrite the original corpus, silently remove a disputed assertion, or
replace an author's words with prose invented by Jev. A short explanation may
be written later by the curator/editor from the retained passages.

Use stable markers or structured occurrence anchors rather than line-number
insertion order. Exclude generated audit markers and audit output from the next
claim extraction so the auditor does not recursively audit its own prose.
Compute the index-content digest from that same marker-free representation;
the auditor's own annotations must not invalidate its input snapshot. Preserve
the mapping back to the displayed marked file.

### 5. Finish only the current revision, and resume honestly

After all scheduled tasks settle, code checks the block inventory, every
claim-reference judgment, every aggregate claim judgment, and the entire pair
set by identity, not counts alone. Verify that every flagged occurrence has its
marker and the linked counterpart exists. Re-check input hashes, then publish
a completion summary. Successful audit execution can contain findings.

Cache by claim text/scope, evidence content, question definition, and pinned
model. A changed claim invalidates its source checks and all incident pairs;
a changed source invalidates dependent checks and source-backed follow-ups.
Changes to index context or metric definitions also invalidate pair judgments.
New claims compare against every retained claim. Removed claims retire their
edges and markers. The active revision must never inherit a “complete” flag
from a different snapshot.

Bound concurrent HTTP work, not coverage. Respect retry headers and backoff;
permanent request errors and cancellation produce an incomplete audit. Persist
completed responses before advancing so a crash does not require the whole
quadratic pass again. There is no request-count cap that silently converts
unchecked work to “passed.” Operational retry exhaustion may stop the command,
but it must return incomplete and remain resumable.

## Fit in the actual workflow

The intended sequence remains visible in research-document:

```text
collect sources in the existing five research branches
join research
curator builds the combined index
curator enumerates claims over all index blocks
RunCommand: audit the index; wait for complete results
curator repairs omissions, inaccurate assertions, and missing qualification
repeat the audit for changed inputs; retain explicitly unresolved disputes
author writes from the annotated index and original sources
editor checks the document
if evidence is missing:
    research the gaps and update the index
    enumerate changed claims and run the audit again
revise the document using the current audit and original evidence
```

Use `gimbal.Set` with constant keys for the absolute claims/audit paths and
small summaries, scoped afresh for each repair pass. Keep the actual loop in
the workflow. A missing claim inventory or incomplete transport run blocks
authoring; a documented source disagreement does not inherently block writing
a document that accurately explains that disagreement.

Do not require a conflict-free index. Require that unsupported assertions are
repaired, qualified as unresolved, or removed from factual use, and that the
document does not assert disputed claims without their qualifications. Existing
editorial limits still apply to research/author repair; they are not a cap on
the audit's claim or pair coverage. Exhaustion returns an honest error with
the annotated index left available.

The author/editor prompts must name the current audit path and distinguish
source support from inter-claim agreement. The final editor still checks any
new synthesis introduced by the document; auditing the index alone does not
prove the author made no new unsupported claims. Before final acceptance,
verify that the audited index/source revision is unchanged; if it changed,
audit it and make the editor inspect that revision.

### Concrete command boundary

Proposed invocation: `gimbal audit-index --research-dir <absolute-directory>`.
The command consumes the local file inventory and claims file, performs all
Jev calls, marks the index, and exits only after finishing or recording an
incomplete audit. Exit 0 means complete and annotations written, including
findings; a nonzero exit means incomplete/invalid execution. The calling
workflow inspects the small completion summary to choose repair or authoring.

Resolve `os.Executable()` as research-document already does for token counting.
Use a literal command node name such as `audit-index`, and check both the
`RunCommand` error and exit code. Read the complete structured result from its
file: command output larger than 64 KiB is returned as a head/tail excerpt.

The child inherits `TYPESAFE_API_KEY` from the hosted process, whose existing
secret loader supplies configuration/Doppler credentials. Do not pass secrets
in argv or audit files. Run directly as the foreground binary with HTTP worker
goroutines. Current `RunCommand` cancellation kills the immediate process,
not an arbitrary descendant process tree: avoid a shell wrapper, daemon,
or agent subprocess in the auditor. Durable partial results must tolerate
abrupt termination.

No nested `gimbal.Run` is needed. It would create separate run state without
joining the hosted parent's live controls. The parent graph displays one
command, with streamed progress and result paths, rather than a separate node
per Jev request. Provider usage can be reported in that command's summary; it
will not automatically become coding-agent token accounting.

### Alternatives

| Approach | What it buys | Cost / limitation |
| --- | --- | --- |
| **Compiled audit subcommand + RunCommand** | Visible blocking stage; existing cancellation and output; no public API addition | Individual requests are command progress, not separate graph nodes. Recommended. |
| Ordinary Go SDK calls inside the workflow | No subprocess; direct parent context | Still no native Jev nodes; substantial audit machinery clutters the workflow if left inline. Technically possible today. |
| New typed Jev operation in the workflow API | Per-call graph/usage/inspection support | Public API, event schema, generator, UI, and accounting changes; justified only if those controls are required. |

A separately built `cmd/audit-index` can use exactly the same file contract if
binary separation is desired. Merely compiling another Gimbal workflow does
not grant it hosted child-run semantics. Start with the command boundary; ask
for a new language primitive only if request-level UI control becomes an actual
requirement.

## Work size and batching

Let B be the number of index blocks, C the number of claim-reference
associations, and N the number of claims. Baseline judgments are approximately
B coverage checks + N extraction checks + C citation checks + N aggregate
checks + N(N-1)/2 pair checks. Pair scope questions and focused follow-ups add
judgments. Shared-state batching reduces HTTP calls without reducing coverage.

| Claims | Unordered pairs |
| ---: | ---: |
| 100 | 4,950 |
| 500 | 124,750 |
| 1,000 | 499,500 |
| 5,000 | 12,497,500 |

Start with small pair packets, not a whole-index prompt. Batch questions sharing
one source section, or one anchor claim and a bounded group of comparisons.
Each instruction names the relevant state entries. Pack below both documented
token limits; do not assume a byte count is an exact Jev tokenizer. A provider
size rejection means split the batch, not discard questions or source text.
Use measured `usage.input_tokens` for cost projections and actual throughput
for elapsed-time projections. At an illustrative 1,000 input tokens per pair,
499,500 pairs would cost about $21 at the documented price, before extra checks
and retries. This is arithmetic, not a measured forecast. At one pair per
request, request-rate limits can dominate elapsed time; batching changes that.

## What must be demonstrated before calling the implementation done

Package-local tests should establish enumeration, citation resolution, both-end
annotation, snapshot invalidation, retry/resume, and refusal to report complete
when answers are missing. Semantic cases need independently labeled original
passages: true paraphrases, fabricated/moved quotations, absent evidence,
opposite claims across topic branches, qualifier loss, conflicting originals,
different versions, different measurements, and facts hidden in tables/root
summaries. Include at least one block whose extractor omitted a claim.

Run the real command against a real collected research index, inspect every
seeded defect and a hand-labeled clean slice, and report misses and false
positives separately. Use held-out examples to choose confidence policy;
rephrasing failed pilot questions is prompt development, not held-out proof.
Observe interruption and resumption without losing completed work or creating
a false complete state.

Then run research-document with cheap supported research/author models, observe
that authoring waits for the initial audit, introduce gap research, and observe
the new audit before revision. Confirm the final document preserves unresolved
source disagreement. Regenerate the workflow graph and rebuild the hosted
binary for the changed call sites. Report those observations in chat/PR text;
do not commit run logs, proof programs, or output captures.

This proposal does not yet supply calibrated thresholds, corpus-level recall,
or an implementation. It supplies the exhaustive scheduling algorithm, artifact
and failure semantics, and a path through the current workflow API.
