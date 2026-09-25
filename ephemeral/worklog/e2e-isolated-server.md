decision: Issue #274 uses Playwright's managed webServer with Gimbal's existing port-zero listener and startup address; explicit BASE_URL remains the way to target an existing server.
friction: The browser suite requires recorded runs, but a clean worktree has none. The existing internal/observation/testdata/cancelled-run.jsonl fixture supplies its current read-only scenarios when copied into disposable project data.
decision: Clear inherited GIMBAL_WEB_PROXY and GIMBAL_WEB_ORIGIN for the managed server so the built binary serves its embedded frontend at its actual listener origin.
