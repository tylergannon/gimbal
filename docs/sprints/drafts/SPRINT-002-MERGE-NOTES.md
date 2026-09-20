# Sprint 002 merge notes

**Authority:** [SPRINT-002.md](../SPRINT-002.md) is the planned sprint. The
intent, three drafts, and critiques remain input, not competing contracts.

The merged direction is deliberately small: retain a Claude process and reader
for one adapter attempt while it receives explicit `waiting` and then
`completed` declarations; close after completion; make the next `Generate` a
fresh `--resume` process with its own schema. Native result success is not task
completion. This is not a Session daemon, public API, provider-wide change, or
promise that validation retries use one process.

Material corrections from review are retained. The private envelope has an
intermediate message and a required final payload; it cannot accept missing
payload as empty `Text`. Schema composition must preserve root `$defs`/`$ref`
semantics rather than nesting an arbitrary caller schema. A resumed orphan,
including a structured one, cannot satisfy the new prompt. Origin, index, and
a launch-schema token are diagnostic only; the precise private attribution rule
is intentionally left for source-backed implementation validation. A command
exit failure is not automatically an assignment failure.

The live acceptance remains the existing temporary two-call Haiku fixture: it
must witness waiting, native notification, automatic wakeup, command-minted
receipt, and return ordering; then prove same conversation but different PIDs
and schemas with context recall and no host reprompt. The documented
`EARLY_RETURN` run is the red unchanged-harness baseline, not a green claim.

Plan writing is complete when these two documents pass independent inspection.
The product remains planned until a changed harness has a green live Haiku run
and focused regressions; neither implementation nor that run is authorized by
this documentation assignment.
