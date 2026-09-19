decision: Replace the frontend-specific builder with a generic implementation Promise Loop driven by an explicit promise, a local definition-of-done file, and a bounded task count.
decision: The generic workflow modifies the working tree but does not commit, push, merge, or deploy; those are delivery policy rather than implementation-loop semantics.
correction: One-off workflows are acceptable when they stay cheap and reliable; promote only recurring proven shapes into a small set of polished generic workflows.
doc_bug: docs/design/README.md still names the removed build-frontend command -> update only with explicit permission to edit docs.
correction: Unit tests and deterministic commands are checks that gather evidence, never validation; only the independent validator assesses whether the requirements are demonstrated.
correction: The overall promise and supplied definition of done are the loop's authority; every planner task has its own definition of done, and validation occurs inside each lap before the planner receives feedback or the loop may finish.
correction: Passing independent validation at 90–95% with only small gaps ends the loop immediately; the loop is forbidden from taking another lap for the last 5%.
