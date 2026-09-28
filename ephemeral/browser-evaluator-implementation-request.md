Implement the validate-product migration in section 6 of the accepted current
ephemeral/browser-implementation-design.md, governed by all claims in
ephemeral/browser-evaluator-build.md. Read repository instructions. You are not
alone; preserve and accommodate others' changes.

Own internal/workflows/validateproduct source and tests only. Browser core,
generator, linter, Codex and execution backend belong to other workers. Do not
edit generated workflow_gen.go by hand or run global generation; the manager
will generate and integrate once all source edits are ready.

Keep the explicit tester slots and straightforward Go orchestration. Acquire
NewBrowser within each tester scope, pass WithBrowser for browser operation,
finalize recording on scope exit, then encode through environment RunCommand.
Preserve task errors independently of startup/cleanup errors, and retain the
bounded post-cancellation encoding of successfully finalized recordings. Remove
manual browser management, host-only executable checks/encoding and old
PlaywrightCLI configuration. Check input readability in the execution environment.
Make issue_repo optional and use the report-only prompt when absent; that mode
must not publish issues or upload artifacts. Update package documentation and
meaningful existing tests accordingly. Follow the design for all details.

The public API is NewBrowser(ctx context.Context, name, workdir, video string)
(*Browser, error) and WithBrowser(*Browser) AgentOption. Report any real contract
problem promptly rather than broadening the design. Run focused package tests.
Do not commit, launch agents, invoke Gimbal or edit outside your ownership.
Report changed files, actual tests, and unresolved problems. Manager owns
regeneration, integration, independent review, and authenticated live validation.
