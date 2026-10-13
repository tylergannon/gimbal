# Adversarial review: promises-ledger plan, round 04

> Recovered from the session log. The original `ephemeral/reviews/202610130830-promises-ledger-plan-round-04.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Issue — invalid evidence is replaced with unrelated speech and then presented as the operation's audit trail

The plan says per-entry evidence is the audit trail (`plan.md:56`), asks the
model for the exact new chunk and a verbatim quote (`plan.md:316-318`), and
renders those quotes as evidence links in the Promises tab
(`plan.md:394-400`). But `apply` deliberately makes evidence incapable of
invalidating an operation: an out-of-range chunk is changed to the batch's
first new chunk, and a quote absent from that chunk is changed to that chunk's
entire text (`plan.md:345-350`).

A direct reproduction needs two chunks: chunk 10 says “Exports finish in two
seconds” and chunk 11 says “Users can tag notes.” If the model correctly adds
the tagging promise but returns chunk 999, the plan stores chunk 10 and its
export sentence as the tagging promise's evidence. A fabricated quote follows
the same path. The operation succeeds and the cursor advances, so the false
association is durable. This is weaker than the cited Clarity boundary, which
rejects missing or out-of-range evidence references before application
(`ResponsesClaimReviewProvider.swift:100-138`).

Impact: the ledger can display genuine transcript text as support for a promise
that the text does not support, which is more misleading than omitting evidence
altogether. The lean request does not require proof machinery; if evidence is
kept, an invalid chunk/quote must either be repaired by locating the supplied
quote in the actual new chunks, omitted, or reject the reply rather than being
silently reassigned to unrelated speech. The tests should cover this two-chunk
case, not only that a fabricated quote is replaced by some real text.

### 2. Issue — `MarkRequirements` can durably split the flag between the recording and the history the worker scans

The existing-recording path calls a new `MarkRequirements` that reads and then
uses the transcript store's existing `save` (`plan.md:149-159`). That `save` is
not one atomic header update: it first writes `<id>.json`, then separately
writes `<id>.meta.json`, and updates the in-memory `headers` map only after both
succeed (`internal/transcript/durable.go:127-135`). `Get` reads the full JSON,
while `List` reads the cached metadata headers (`durable.go:232-270`). The new
worker filters the paged `List` result for `Requirements` before calling `Get`
(`plan.md:186-194`).

If the first write succeeds and the metadata write fails, the command returns
an error but the full recording already says `Requirements=true`; the live
recording view then reads that file and shows “Requirements discussion,” while
the worker's header map remains false and never ingests it. Restart does not
reconcile the files: `NewDurable` repopulates the map from the stale meta file,
so the split persists. The proposed tests cover only successful mark and reload
(`plan.md:409`), unlike the ledger tests that explicitly exercise failed
durable writes.

Impact: the required “Treat as requirements discussion” action can visibly
take effect yet never put that recording's promises into the ledger. The flag
needs one authoritative durable commit or explicit reconciliation of the two
files, plus a failure test at the boundary between their writes.

### 3. Issue — an indeterminate ledger save can leave a recording in a permanent error state even after its cursor committed

The ledger correctly reloads after every save error because `WriteJSON` may
fail after rename (`plan.md:227-234`; `internal/transcript/durable.go:95-125`).
When reload finds the new file, however, the plan adopts the advanced cursor,
sets `errs[R]`, and does not resend the batch (`plan.md:217-232`). The only
transition that deletes `errs[R]` is a later successfully saved batch for that
same recording (`plan.md:211-223`). If the recording was stopped and this was
its final batch, `n <= done` forever, so that transition can never occur.

Impact: the Promises tab permanently reports that a recording failed even
though its entries and cursor are the durable state the worker has chosen to
adopt; the only accidental way to clear the warning is restarting Gimbal,
which clears all in-memory errors. Conversely, a post-rename sync failure does
represent unresolved durability and should not simply be called success. The
plan needs an explicit resolvable state for this case—such as retrying the
durability acknowledgment or distinguishing an adopted commit from a batch
that still needs replay—and a fault-injection test after rename. Making the
directory unwritable tests only the pre-rename branch (`plan.md:426-429`).

### 4. Issue — the final required live demonstration is planned on a model the repository forbids for proof runs

The repository requires live runs started to see behavior work to use Claude
Haiku and to name the model (`AGENTS.md:70-75`). The plan recognizes that rule
and says Haiku is the proof model (`plan.md:255-258,431-435`), but its final
real-microphone demonstration explicitly runs “on the default model”
(`plan.md:447`), while the default is `sonnet:low` (`plan.md:255-258`).

Impact: following the build order literally violates an authoritative working
instruction at the final definition-of-done proof. Run both live evaluations,
including the microphone demo, with `GIMBAL_PROMISES_MODEL=haiku` and report
that model; the product default can remain a separate design decision.

## Outcome

**material findings remain**
