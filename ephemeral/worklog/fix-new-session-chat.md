correction: The user requires ordinary chat without Git and explicitly waived release requirements; fix, verify chat, and merge.
decision: Git worktrees are optional for chat. An unborn repository, including one inherited from a parent folder, must use the selected directory rather than reject the first message. Setup and creation must agree on whether HEAD exists.
decision: A selected folder under a parent repository is an ordinary chat workspace; only the selected repository root with a valid HEAD receives an isolated worktree.
friction: Reusing another checkout's frontend build failed the remote-function manifest check; build the frontend in the task worktree before testing the real server.
