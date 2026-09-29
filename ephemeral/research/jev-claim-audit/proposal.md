# Jev source checks and exhaustive pairwise contradiction marking

Proposal, 2026-09-29. No workflow changes are implemented by this document.
Repository baseline: `f276674347930bffc77022896f00a3bc4a762281`.

## Recommendation

Write a compiled Go command, `gimbal audit-index`, that reads the collected
sources and semantic index, checks every indexed claim with Jev, packs explicit
pair questions into small shared-state requests, and annotates every occurrence
of a disputed claim. Packed pairs are the only specified first implementation;
the whole-index screen below remains an optional research direction. Invoke it
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

The acceptance promise is **every claim source-checked, pairwise contradiction
search covering the whole index, all unresolved findings visible at their
occurrences**. This is a pairwise coverage promise, not universal logical
consistency: three or more claims can be jointly inconsistent while each pair
is compatible. Group-level findings are outside the first implementation; if
the optional group path is later built, observed group inconsistencies must
remain visible even when no pair witness explains them.
A separate Jev request or output edge for every pair is not a requirement. A model judgment is
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
  small questions. Arithmetic in Go only helps when trustworthy structured
  quantities already exist; this index does not supply them. Do not ask Jev to write claim records
  or explanations.
- [Confidence](https://docs.typesafe.ai/confidence) describes the distribution,
  not independently verified correctness. The initial policy below uses returned
  labels without a confidence cutoff. Any future probability threshold needs
  labeled research examples; do not copy the supervisor's 0.65 threshold.

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

Run `gimbal audit-index --prepare --research-dir <absolute-directory>` after
curation. Code produces the file/block inventory, using the researchers' file
roles and cross-checking every file under the research directory. A file with
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

The prepare operation records marker-free block digests, enclosing headings,
table headers, stable occurrence anchors, source paths, and context digests.
It reuses unchanged blocks' extraction records **verbatim**, including IDs,
claim text, splits, and digests; it does not ask the curator to recreate them.
It gives the curator only new, changed, or still-rejected blocks to
extract into `claims.jsonl`. Code assigns new claim IDs and merges those records
with the unchanged records. A heading/table-context change counts as a changed
block; loss or ambiguity of an anchor requires occurrence repair, not a guessed
match. Source changes invalidate dependent judgments even if extraction text
is unchanged. This producer runs before the first extraction and each repair
pass, never between retries of the same frozen audit.

For assigned blocks, split compound assertions and retain negation, “only,”
“always,” units, attribution, and applicability. Questions and pure routing
instructions are not factual claims, but claims inside recommendations and
descriptions still count. For example, “use X because it supports images”
contains an independently checked factual premise.

**Extraction cannot be trusted merely because it produced JSON.** For every
block, Jev checks whether the proposed records faithfully cover all assertions
and qualifiers; a block with no records still receives this question. For each
record, check that the cited index span actually asserts it. Code checks that
all blocks and records have answers. Omission or distortion returns the exact
block to the curator for repair and re-audit. An unresolved block prevents a
claim of complete coverage. A suspected false extraction alarm can use the
independent `reviewed-extraction-complete` resolution below; a valid resolution
also prevents unchanged records being re-extracted merely because Jev repeated
that rejected alarm. This establishes exhaustive scheduling; semantic
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

For `inference` and `recommendation`, extract their stated factual premises
as separate assertions and apply the same literal source checks to those
premises. A source need not literally state the resulting synthesis; a
`not-established` judgment about that synthesis is not by itself a bad-citation
finding. Keep its `kind` visible and let the existing editor assess whether the
stated premises ground it, requiring qualification or exclusion when they do
not. This happens in the existing post-authoring editorial review; the author
may draft labeled inferences before that review, without a new pre-authoring
pass. The author may present an accepted synthesis explicitly as an inference
or recommendation, never relabel it a source-established fact. Unsupported or
disputed premises retain their findings and restrictions. This is editorial
grounding, not an inference engine or an exemption for factual claims hidden
inside advice.

Store judgment, probabilities, confidence, exact evidence identifiers, model,
and question revision. Successful evaluation means the answer was recorded,
not that the claim passed. Do not stop after the first false claim.

### 3. Start with packed explicit pair questions

Partition the complete inventory into base batches of at most **five claims**.
For each nontrivial batch, send its claims and one independent Choice question
for each internal pair (at most ten questions). For every pair of batches, send
both batches in shared state (at most ten claims) and one question for each
cross-boundary pair (at most 25 questions). Every unordered pair is scheduled
exactly once. A clean internal batch says nothing about another batch.

This is the concrete first algorithm. There is no group-screen recursion that
must descend to every pair anyway. It saves HTTP requests and repeated claim
text without relying on a group negative to discharge unseen pair questions.
It does not reduce the N(N-1)/2 semantic questions; their repeated instructions
still cost tokens. Shared instructions can live in state, but each question
must identify its pair explicitly. The initial five-claim size is an operating choice, not measured semantic
fidelity. Questions explicitly name both claim IDs and retain original text,
qualifiers, and necessary enclosing context; question IDs alone are invisible
to Jev. Keep unsupported and source-contradicted claims in this search too.

Use estimated packet budgets of 48k total tokens and 24k for state plus its
longest question, leaving margin below the published 64k/32k limits. There is
no established local Jev tokenizer, so these estimates are not proof a request
fits. Split the question list and include only the original claims/context its
questions require when either estimate is exceeded. Provider size rejection is
authoritative and also causes a split, never truncation. A single pair that still cannot fit needs context/evidence windows
or an explicit unresolved judgment, not a falsely completed comparison.
Different questions in one request are independent; no question may consume
another answer from that request.

Use Choice labels `contradiction`, `uncertain`, `different_scope`, and
`compatible`. The first operating policy is deliberately explicit and
**uncalibrated**, using the returned top label without a probability/confidence
cutoff:

| Returned label | Recorded relation and action |
| --- | --- |
| `contradiction` | `conflict`: mark both claims and require a disposition. |
| `uncertain` | `possible-conflict`: visibly mark both claims unresolved. |
| `different_scope` | `different-scope`: no dispute finding from this answer. |
| `compatible` | `compatible`: no dispute finding from this answer. |

An exact top-probability tie involving `contradiction` or `uncertain` is
`possible-conflict`; a tie between the two nonconflict labels creates no dispute.
An invalid/missing answer is an incomplete task, never a clean result. Store
the full distribution and confidence for inspection, but minority contradiction
mass alone does not create another finding in this initial policy. Source
verdicts likewise retain their returned Choice label, with the deterministic
missing/invalid-reference prohibitions above; no unspecified threshold blocks
execution. These decisions establish scheduling and inspectable judgments,
not measured semantic recall.

For conflict/possible-conflict, follow up with original passages for both
claims, retaining evidence that opposes either assertion. Apply the same label
mapping to that focused answer. Disagreement with an earlier judgment, explicit
unresolved applicability, or insufficient context stays `possible-conflict`
until source-backed review resolves it. Do not choose a winner from confidence,
source count, or date alone. A `conflict` remains a Jev finding, not an
independently confirmed contradiction. Store pairs in canonical order and mark
both endpoints; contradiction is symmetric, not transitive.

Under this top-label rule the packed pilot's 28 pair answers yield five initial
conflict findings and no possible-conflict findings: three planted conflicts
and two false positives involving differently measured limits. This is a small
synthetic case, not an estimate of production prevalence. At 1,000 claims even
a hypothetical 0.1% flag rate would mean about 500 findings, so authoring cannot
depend on resolving each one in a separate agent turn.

**Known limitation:** the exploratory Jev 1.13 pilot confused differently
measured limits (total-request capacity versus state-plus-question capacity).
The focused scope follow-up and group prompts did not establish a reliable
resolution. Free-text scope is not a trusted quantity parser: where evidence
judgments disagree and no trusted structured quantities exist, retain an
unresolved finding. A later shared-state packed-pair probe preserved the same measurement-scope
false positives: packing worked as a request mechanism, not as a semantic fix.
This proposal does not add a parser, ontology, or numerical reasoning framework. Different measurement names alone are not proof of
compatibility; nor are different numbers proof of conflict.

#### Researched alternative: one whole-index coherence question

The user's latest idea is feasible as an optional fast path: before partitioning,
a complete original claim inventory plus necessary context and one question
might fit Jev's 32k state-plus-longest-question limit and 64k whole-request
limit. Use the conservative estimates/headroom above; claim text alone fitting
“32k” is insufficient, and provider size rejection is authoritative. An exact
whole-index negative would clear the pairwise relation in one query. Context
fit does not establish semantic fidelity: accepting Jev's negative would need
independent source-backed evaluation at the actual whole-packet claim count and
token length, not extrapolation from tests at 5/10/20/40 claims. Source checks
for every claim remain separate. Without that evidence, use packed pairs.

The named scope-owned-services corpus does fit as raw prose: its 21 non-source
Markdown documents (indexes and clips) contain 7,759 whitespace-separated words
and 62,670 UTF-8 text bytes. Including original texts, relative paths, and one
coherence question produced a 65,423-byte serialized packet. `gimbal count-tokens`
reported 14,709 tokens with its `o200k_base` proxy; Jev 1.13.0 accepted the real
request and reported 15,736 input tokens. Its top answer was `no_conflict`, but
that is not a validated clearance: the packet has not been independently labeled,
the atomic inventory has not been extracted, and source truth was not checked.
The eventual claim count and record/context overhead remain unknown. Thus the
user's fit idea works on this raw corpus; using its negative to skip the pair
pass still needs fidelity evidence at that actual packet size.

A positive could enter the retained-witness search proved in
[mathematical-analysis.md](mathematical-analysis.md); uncertainty or inconsistent
answers return to packed pairs, retaining any unresolved group finding. That
proof shows O(log K) localization of one witness and O(m² + N log K) exact-oracle
queries for marking vertices globally. For its partition schedule,
`b = floor(K/2)` and `m = ceil(N/b)`; marked endpoints remain available against
unmarked claims. It does not promise subquadratic whole-index work for fixed K.
The user needs affected claims marked, not every edge stored; the reason to
start with explicit pairs is simpler coverage and fallible-judgment handling.
This alternative is research, not a second implementation mandate or a large
calibration program attached to the baseline.

The exploratory group prompts produced expected top labels on repetitive
synthetic examples, but differently measured limits remained ambiguous.
Neither those examples nor the packed-pair feasibility probe establish domain
recall or a safe group size. Real source-backed held-out cases would be needed
before using a whole-index negative to skip comparisons.

#### Where O(n log n) is possible

When original input already provides trustworthy exact values for the same
known single-valued property under identical scope, hashing/sorting can mark
incompatible value groups in O(n)/O(n log n), without emitting all pair edges.
The index currently has free-text scope, so this is a special case, not the
proposed first algorithm. Topic routes, embeddings, and aliases can order work
but cannot exclude comparisons. No LLM-created bucket label is a proof that
cross-bucket comparisons are unnecessary.

### 4. Mark every occurrence, preserving evidence

Code writes `.semantic-index/audit.jsonl` as completed work accumulates and
renders an `AUDIT.md` that resolves claim IDs, locations, source verdicts, and
pair findings and reviewed resolutions. The JSONL is the input to
marking and resume, not an optional second report. The audit renderer adds a compact visible marker next to every
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
claim-reference judgment, every aggregate claim judgment, and an explicit
answer for every claim pair by identity, not counts alone. An unanswered task
covers nothing. Verify every current dispute has its occurrence marker and
linked finding, and every cleared marker has a valid reviewed resolution.
Re-check input hashes, then publish a completion summary. Successful execution
can contain findings; unresolved judgments are answered, never clean or supported.

Cache by claim text/scope, evidence content, question definition, and pinned
model. Unchanged extraction records, valid cached judgments, and dispositions
carry forward exactly. A changed claim invalidates its source checks and all
incident pairs. Changed source passages, index context, metric definitions, or
occurrence anchors invalidate dependent judgments and resolutions. An invalid
anchor blocks marking until repaired. New claims compare against every retained
claim; removed claims retire their findings and markers. The active revision
must never inherit a completion flag from a different snapshot.

Bound concurrent HTTP work, not coverage. Respect retry headers and backoff;
persist completed responses before advancing. After bounded HTTP retries, a
transient transport/rate-limit failure records an incomplete checkpoint. The
workflow makes up to three command attempts total on that **same frozen
revision**, resuming completed work without preparing, extracting, researching,
or editing between attempts. Cancellation, authentication failure, permanent
invalid input, or an inconsistent completion record stops immediately. Exhausting
those attempts fails the stage with its checkpoint intact; a direct invocation
on that unchanged research directory can still resume it. No request-count cap
silently converts unchecked work to “passed.”

## Fit in the actual workflow

Collect sources, join research, and curate the combined index as today. Before
initial authoring, prepare the block inventory, extract only assigned blocks,
and run the blocking audit. Reuse the existing `maxRounds` value (default three) as the maximum number of curator
repair turns for this pre-authoring stage; allow one initial audit and one
re-audit after each turn. This is a bound on repairs, never on covered claims or
pairs. Apply the same bounded stage after gap research changes the index, before
document revision; the existing outer editorial loop remains bounded too.

The command writes `.semantic-index/completion.json` with these fields:

| Field | Meaning |
| --- | --- |
| `revision` | Hash of the current marker-free inventory, claims, context, and sources. |
| `coverage_mode` | `pairwise`; never a claim of universal logical consistency. |
| `complete` | Every scheduled extraction, source, and pairwise task finished with an answer; hashes match. Findings, including omissions, are answers; request failures are not. |
| `coverage_complete` | The inventory covers all index blocks, extraction has no known omitted/distorted assertion, pairwise coverage is accounted for, and occurrence markers are valid. |
| `repair_required` | Claim/block IDs with extraction omissions, invalid occurrences, source findings, or `conflict` findings lacking an acceptable disposition. Validly marked `possible-conflict` findings may remain unresolved without a separate curator turn. |
| `unresolved` | IDs of retained unresolved claims and pairs, including those already dispositioned. |
| `authoring_allowed` | `complete && coverage_complete && len(repair_required) == 0`, recomputed by code. |

Pair findings have a curator-selected disposition, validated against the current
claim and source digest: `retain-unresolved` or `exclude-from-factual-use`.
`retain-unresolved` keeps the original assertion, evidence, and visible audit
qualification; it authorizes describing the dispute, not asserting the claim
as settled. `exclude-from-factual-use` retains the finding/marker but prohibits
using the assertion as a factual premise. When both sourced claims carry a
current disposition, the pair leaves `repair_required`; it remains unresolved.
Source findings require repair, or an independent editor's
`reviewed-source-supported` decision against every valid cited original.
Uncited and invalid-reference claims cannot be reviewed or dispositioned away.
Genuine or unresolved extraction omissions/distortions and invalid occurrence
mapping cannot be dispositioned away.

For a suspected false extraction alarm, the curator may propose
`reviewed-extraction-complete`, citing the original block, its enclosing context,
and **all** its extracted records (including an intentionally empty list).
The existing editor independently checks every assertion and qualifier against
those records. Only if it agrees that nothing is omitted or distorted does code
remove that coverage finding from `repair_required`. Keep the raw Jev judgment,
review reasoning, and decision in `AUDIT.md` and the structured audit, bound to
the block/anchor/context digest, the complete ordered record set and its digests,
and extraction/review-policy digests. Any change invalidates the resolution.
An actual omission must be repaired; an unresolved dispute about completeness
still blocks `coverage_complete`, however often Jev has repeated it. A valid
review rejects a false alarm, not the requirement to enumerate every assertion.

Finding volume does not change those meanings. A `possible-conflict` receives
an auditor-owned unresolved marker and may remain in `unresolved` without
entering `repair_required`; this is retention, not clearance. The author may
report what each source says with attribution and describe the unresolved
relationship, but may not use the affected assertions as settled, mutually
compatible premises. The same restriction applies to explicitly retained
conflicts. A factual assertion needs a `supports` aggregate source verdict,
valid references, and no active dispute or source finding to be stated normally
within its recorded qualifications; that remains a model judgment against
collected evidence. Editorial inferences follow the explicitly labeled grounding
rule above and do not gain factual status from it.
A reviewer may prioritize consequential claims while leaving the rest visible;
neither a repair limit nor a long list authorizes silent dismissal.

One curator/editor turn may address multiple findings. Where cause and evidence
are identical, a shared rationale may resolve an explicitly enumerated set of
pairs; each pair must still carry its own claim/context/evidence bindings and
review decision. Inspect the original passages for every listed pair. Do not
clear all pairs of a claim from one representative, infer compatibility
transitively, or apply a dismissal to unlisted pairs. Findings without an actual
review decision stay marked; batching review is not a blanket approval.

For a suspected false conflict, the curator may propose `reviewed-compatible`
with the original passages for **both** claims and why the assertions coexist.
Before authoring, instantiate the existing editor role for this independent
assessment from original evidence; the curator cannot dismiss its own finding. If the
editor agrees, code records the decision and reasoning, both source spans,
both claim/context digests, evidence digests, and judgment/review-policy digests.
Keep the raw Jev verdict beside the resolution in `AUDIT.md` and the structured
audit; change neither claims nor original evidence. Remove only this pair's
dispute markers and repair requirement, preserving other active disputes and
source-check findings. If the editor disagrees or cannot resolve scope, retain
the unresolved finding. Changed claims, context, evidence, anchors, or governing
policy invalidate the resolution and restore the finding pending fresh assessment.
Correct facts can then be used without presenting a rejected model false positive
as a real dispute.

Only deterministic, auditor-owned qualification/disposition annotations are
excluded from extraction, so marking an unchanged unresolved claim does not
create another claim and re-trigger repair. Arbitrary curator prose is not
exempt just because it appears next to a disposition. If the curator rewrites
the underlying assertion or scope, it is a changed claim and must be re-audited; any disposition must be reconsidered
for that digest. New substantive prose in the index always gets extracted.

The loop is ordinary Go in research-document, with each repeated operation in
its own scope; the following sketches control flow rather than adding API names:

```go
for repair := 0; ; repair++ {
    // RunCommand "prepare-audit": audit-index --prepare; fail on execution error.
    // Curator extracts only prepare's new/changed/rejected blocks; code merges records.
    for attempt := 1; attempt <= 3; attempt++ {
        exit, _, _, err := gimbal.RunCommand(ctx, "audit-index", env.WorkDir,
            executable, "audit-index", "--research-dir", researchDir)
        if err != nil { return err } // Includes cancellation/start/capture failure.
        // Read completion.json; require the frozen revision and valid JSON.
        if exit == 0 && summary.Complete { break }
        if exit != 75 || summary.Complete {
            return fmt.Errorf("claim audit permanent or inconsistent failure")
        }
        if attempt == 3 { return fmt.Errorf("claim audit transient retries exhausted") }
        // Back off; repeat on the same revision/checkpoint, without extraction.
    }
    if summary.AuthoringAllowed { break }
    if repair == maxRounds { return fmt.Errorf("claim audit repair limit reached") }
    // Curator repairs/dispositions findings; existing editor assesses proposed dismissals.
}
// Existing author writes from the current marked index and original sources.
```

A missing inventory, incomplete transport run, uncovered claim, or invalid
annotation blocks authoring. A fully audited and visibly qualified source
disagreement does not. If the repair bound is exhausted, return an error and
leave the annotated index available; do not author from a still-unacceptable
revision. If this happens after gap research, leave the previous draft at
`documentPath` in place; the failed run does not accept it for the new revision.
Use `gimbal.Set` with constant keys for absolute claims/audit paths and
small summaries, scoped afresh for each pass. The complete data stays in files.

The author/editor prompts must name the current audit path and distinguish
source support from inter-claim agreement. The final editor still checks any
new synthesis introduced by the document; auditing the index alone does not
prove the author made no new unsupported claims. Before final acceptance,
verify that the audited index/source revision is unchanged; if it changed,
audit it and make the editor inspect that revision.

### Concrete command boundary

The prepare invocation produces the local inventory and extraction assignments;
the normal invocation is `gimbal audit-index --research-dir <absolute-directory>`.
It consumes that inventory and claims file, performs Jev calls, marks the index,
and writes `.semantic-index/completion.json` for the current frozen revision.
Exit 0 plus `complete: true` means execution finished, including findings and
known extraction omissions. Exit 75 plus `complete: false` means a resumable
transient failure. Other nonzero exits are terminal; authentication/invalid-input
errors must not use 75. An exit/summary disagreement, missing summary, or wrong
revision is a protocol failure, not a reason to author or retry indefinitely.
`coverage_complete` and `authoring_allowed` decide whether a finished audit needs
curator repair: a detected omission is a finding, not a transport failure.

Resolve `os.Executable()` as research-document already does for token counting.
Use a literal command node name such as `audit-index`, and check both the
`RunCommand` error and exit code. Read the complete structured result from its
file: command output larger than 64 KiB is returned as a head/tail excerpt.

The child inherits `TYPESAFE_API_KEY` from the hosted process, whose existing
secret loader supplies configuration/Doppler credentials. Exempt `audit-index`
from the unconditional pre-command `needsSecrets` path, then resolve secrets
conditionally inside this command: `--prepare` never needs a key; a normal
invocation with an inherited key skips secret loading entirely. For direct
resume without an environment key, use the existing config/Doppler loader.
Initialize the current-revision incomplete summary/checkpoint before that
fallback, so a transient loader failure has the same resumable exit-75 record
as a Jev transport failure. Preserve transport/HTTP status when classifying
loader errors: timeouts and retryable service errors may be transient; invalid
config, missing keys, and rejected authentication are terminal. Thus the hosted
child never re-fetches Doppler, and a configured standalone caller can resume
without manually exporting a key.

The proposed command also needs actual exit routing: after writing its
incomplete checkpoint, return a package-local typed transient-audit error;
`executeCLI` recognizes that type with `errors.As` and returns 75 before its
existing generic error-to-1 mapping. Cancellation, invalid input, authentication,
and other permanent errors never use that type. The conditional secret-loader
fallback uses the same typed mapping after recording its incomplete checkpoint. Register the Cobra command and
include its name in `isOrdinaryCLI`; no new public workflow primitive is needed.
These are required changes to the current startup path, not existing behavior.
Do not pass secrets in argv or audit files. Run directly as the foreground binary with HTTP worker
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

## Work size: requests, tokens, and worst-case complexity

Source checking is O(C + N + B) judgments for C claim-reference associations,
N claims, and B index blocks, assuming bounded individual evidence. Extraction
and evidence expansion can add work. This is separate from contradiction search.

For the selected first algorithm, b = 5 and m = ceil(N/5). Each internal
batch request has at most ten explicit pair questions; each cross request has
at most 25. Before token splits, retries, and evidence follow-ups, the request
count is at most **m(m+1)/2**, omitting singleton internal batches. The total
number of explicit pair judgments remains N(N-1)/2.

| Claims | Individual-pair requests | Initial packed requests | Ideal request-rate floor, packed / individual |
| ---: | ---: | ---: | ---: |
| 100 | 4,950 | 210 | 0.175 / 4.125 min |
| 300 | 44,850 | 1,830 | 1.525 / 37.375 min |
| 500 | 124,750 | 5,050 | 4.208 / 103.958 min |
| 1,000 | 499,500 | 20,100 | 16.75 / 416.25 min |
| 5,000 | 12,497,500 | 500,500 | 417.083 / 10,414.583 min |

The floors use the published 1,200 requests/minute and are **not latency
forecasts**. Real duration also depends on token throughput, concurrency,
request latency, shared quota, splits, retries, source checks, and follow-ups.
No screen requests are added to this initial algorithm. Shared state reduces
repeated claim text; each pair still has its own question and evaluation.

For an illustrative 300-claim inventory (not a measured count of the corpus),
1,830 packed requests give a 91.5-second ideal request-rate floor. The 28-pair
packed pilot used 7,724 input tokens, about 276 per pair. Extrapolating only that
same short-text/prompt mix to 44,850 pairs gives about $0.52 at $0.042/M input
tokens, before source checks, splits, retries, and follow-ups. This is not a cost
estimate for the actual corpus. The accepted raw whole-corpus question used
about $0.000661 of input tokens at that price, but its small cost does not justify
unvalidated pruning. Packed pairs remain the initial choice.

**Bounded packets do not change the arbitrary-case quadratic exponent.** In the
exact-oracle model where a pair conflict can be learned only by co-presenting
its claims in a query of at most K claims, the all-clean transcript must cover
every pair: otherwise an unseen pair could hide the sole conflict. Each query
covers at most K(K-1)/2 pairs, requiring at least
ceil(N(N-1)/(K(K-1))) queries. This remains quadratic for fixed K, even when the
output marks vertices rather than listing all edges. It is not a lower bound
for structured property records or an unlimited whole-index oracle.

This applies to overlapping batches and adaptive recombinations, not merely
the fixed partition used in the proposed schedule. Two internally clean groups
give no information about their cross pairs. Without additional structure,
there is no way to know which groups hide a conflicting pair without covering
those comparisons. Random regrouping trades work for a probability of missing
an unseen pair; it does not establish exhaustive subquadratic coverage.

When N <= K and the complete packet fits, one exact whole-index negative
clears the pairwise relation in one query, while still reading O(N) text.
This is the optional shortcut above; it does not contradict the bounded-oracle
lower bound when the inventory exceeds the validated packet size. Jev's
whole-index fidelity remains unproven until evaluated at that actual size.
With bounded-length claims, the initial schedule sends O(N²/b + N) claim-text
volume, plus O(N²) explicit-question volume and separately counted evidence.
O(log K) witness localization does not make whole-index coverage O(N log N).
Use returned usage and elapsed time for empirical cost comparisons, never
confidence or a request-count floor as a performance claim.

## What must be demonstrated before calling the implementation done

Package-local tests should establish enumeration, citation resolution, both-end
annotation, snapshot invalidation, retry/resume, and refusal to report complete
when answers are missing. Semantic cases need independently labeled original
passages: true paraphrases, fabricated/moved quotations, absent evidence,
opposite claims across topic branches, qualifier loss, conflicting originals,
different versions, different measurements, and facts hidden in tables/root
summaries. Include an omitted-claim block and correctly extracted routing,
recommendation, and table blocks, including a correct empty extraction. Observe
a false extraction alarm independently resolved for the exact unchanged block
and record set, then invalidated by a changed block/context/record. A genuinely
omitted or still-disputed assertion must continue to block authoring.

Also exercise unchanged-block reuse across repair passes: IDs, byte text,
judgments, and valid dispositions must survive without a new extraction call.
Change a source/context/anchor and observe its dependent state invalidate.
Demonstrate an evidence-backed `reviewed-compatible` decision clears a false
pair dispute while retaining Jev's original judgment and unrelated findings.
Observe a transiently failing audit's bounded command retry resume the same
revision without repeating research or extraction.

Run the real command against a real collected research index, inspect every
seeded defect and a hand-labeled clean slice, and report misses and false
positives separately. Use held-out examples to validate the frozen policy;
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
or an implementation. It supplies a concrete packed-pair starting algorithm,
bounded repair/resume and reviewed-resolution semantics, a path through the
current workflow API, and a separately proved optional whole-index direction.
