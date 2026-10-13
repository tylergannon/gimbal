# Worklog: promises ledger core (pl-core, plan steps 2-4)

- claude.TextOnly(): isolate(extra, textOnly) sets `--tools ""` and
  `--setting-sources ""`, keeps --strict-mcp-config. Applies even with
  WithUserConfiguration (text only wins).
- internal/promises: errs is a `[]Failure` (oldest first) rather than the plan's
  `map[string]string`, because View.Errors must be oldest-first and a map has no order.
- Worker memory (seen, retryAt, failures) lives on Store, touched only by the
  goroutine running step, so tests can drive step with explicit now.
- A cancelled ctx during a batch records no failure and no backoff: shutdown is
  not an error.
- Test hang on first run: a test stepped at t+8 s without a prior step at t+0,
  so the settle clock started at t+8 s. Fixed the test, not the code.
