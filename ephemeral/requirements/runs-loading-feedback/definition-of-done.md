# Runs loading feedback

Complete issue 272, saved in /Users/tyler/.codex/worktrees/803e/gimble/ephemeral/requirements/runs-loading-feedback/272.md, against the current runs-card design. Read the reassessment in /Users/tyler/.codex/worktrees/803e/gimble/ephemeral/requirements/ui-navigation-inspection/272-reassessment.md.

While a client navigation to Runs waits on its data, the user sees accessible loading feedback and a skeleton appropriate to the current cards. Successful navigation replaces it with real cards or the existing empty state. Ordinary background polling keeps existing cards visible. A very long failed-run summary remains contained and readable at desktop and narrow phone widths, without horizontal page overflow or inaccessible card actions.

Demonstrate these outcomes through the freshly built application on an isolated port, including a deliberately delayed Runs navigation and background refresh. Add meaningful browser regressions beside the existing tests. An independent validator must assess observed behavior and required repository checks. Report evidence in the agent result; do not commit screenshots, logs, or proof programs. Consult official SvelteKit documentation for navigation-state APIs used.

Keep this one coherent assignment unless validation finds a substantial gap. The current card design already clamps summaries; avoid redesigning it. Initial server document loading cannot display a page-local skeleton before the document arrives; changing that architecture is outside scope. Issue 290 is separate. Do not change workflow defaults, docs, this definition of done, or unrelated code.

The parent agent owns commits, push, PR, merge, and installation. Implementation and validation agents must not perform those actions. You are not alone in the worktree; preserve others' changes. Use compact browser observations on controlled fixtures; never dump the active workflow's transcript-bearing page or scan entire .gimble session logs.
