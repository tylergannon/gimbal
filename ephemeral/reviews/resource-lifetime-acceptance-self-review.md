# Acceptance plan self-review

Target: ephemeral/plans/resource-lifetime-acceptance.md, before Tyler approval.
This is the spec-review mechanical pass; independent review is separate.

- Consistency: task boundary follows Tyler's accepted proof-first scope. The
  earlier coding-task ban on runtime work remains effective until written-plan
  approval. Observation windows are explicitly experimental limits.
- Traceability: each acceptance claim serves release of owned resources,
  preservation of results/uncertainty, sharing or resumption. Each ownership
  boundary is exercised by a claim. Historical cause is conditional on recurrence.
- Seams: the plan uses existing workflow, executor, adapter and borrowed native
  daemon contracts; it proposes no new seam or public API. It is an acceptance
  plan for the existing repair, not a replacement system nlspec. New cell budgets
  or an architectural decomposition would add scope without serving acceptance.
- Classification: end-to-end resource claims require live integration evidence.
  Local failure/routing and final-result properties also have package-local
  checks. Integration-heavy acceptance is expected for this outstanding proof
  task; decomposing proof into additional product cells would not remove the
  external observation requirement.
- Derivability: accepted scope, exclusions and proof thresholds are explicit.
  Instrumentation details remain derivable. Unsafe ownership conflicts or a
  needed new public contract escalate before the dependent action.
- Pump hygiene: the new task boundary comes from Tyler's explicit scope answer.
  No implementation change or new behavioral guarantee is introduced by this
  draft. It must receive independent review and Tyler approval before use.

Independent round 1 subsequently identified missing topology/harness mapping,
concurrent coverage and a concrete failure owner. The amended draft now names
the persistent instance/daemon and per-turn worker topology, maps claims to CLI
workflows and package live tests, requires bounded two-worker/concurrent-session
observations, and orders private-daemon then owned-client failure vehicles.
Debug recording remains disabled; retained detached threads have an explicit
grace-period baseline distinction. Rechecking the six passes above found no new
internal contradiction. Independent round 2 reports only nitpicks remain; its
four clarifications were incorporated: marked observable shell action, scoped
redial failure injection, private-home test opt-in/isolation/auth guard, and
bounded cancellation-race reattempts. No acceptance claim was removed or added
by those clarifications. This note is not
independent review or runtime proof.
