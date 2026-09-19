decision: Replace the frontend-specific builder with a generic implementation Promise Loop driven by a local requirements file, fixed caller-supplied validation command, and bounded task count.
decision: The generic workflow modifies the working tree but does not commit, push, merge, or deploy; those are delivery policy rather than implementation-loop semantics.
correction: One-off workflows are acceptable when they stay cheap and reliable; promote only recurring proven shapes into a small set of polished generic workflows.
doc_bug: docs/design/README.md still names the removed build-frontend command -> update only with explicit permission to edit docs.
