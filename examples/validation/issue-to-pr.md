# Implement a project issue through Gimbal

You are evaluating Gimbal for use by your engineering team. The `gimbal` binary is
on PATH. Your workspace is a disposable worktree of a different project, B. Read
`issue.md` in this workspace; the caller has saved the issue, repository details,
and its acceptance criteria there.

Use Gimbal to do the implementation and review work, all the way to opening a PR
in B's repository. This assignment permits creating a branch, committing, pushing,
and opening that PR; do not merge it. Rely on Gimbal's agents to inspect and edit
B rather than doing the implementation yourself. Do not inspect Gimbal's source.

Start the implementation workflow with its own web listener (`--port`, not
`--no-web`) and navigate the recorded browser to that address for live monitoring.
The initially supplied standalone server is a history/start page. Preserve the
active project's `.git` and `.gimbal` when delegating scaffolding. Explore the controls and
information you naturally need to understand what is happening. Report whether
you reached a usable PR, link it, and describe interruptions, confusion, bugs,
and the most annoying parts of the experience. Capture ordered screenshots with
captions at meaningful moments; this is a real task, not a click-by-click script.
