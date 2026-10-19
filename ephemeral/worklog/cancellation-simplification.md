# Cancellation simplification worklog

correction: The user explicitly withdrew core cancellation ledgers, retained cleanup, release receipts, and retry-cleanup supervision. Earlier design and DoD text cannot reinstate those requirements.
decision: Preserve fe043bdc through a task-branch checkpoint before implementation. One GPT 6.1 Sol worker owns the simplification; normal completion collects results, cancellation eventually releases resources, and UI shutdown timing is outside acceptance.
friction: Repeated whole-branch Astra reviews turned a bounded fixup into an hour of development. This assignment has focused operational verification and no repeated independent review loop.

decision: Remove the three core pending-resource maps, retained live controllers, runner wait/retry controls, and periodic orphan cleanup supervision. Local resource owners retain bounded stop attempts.
doc_bug: docs/definition-of-done.md still requires retryable retained ownership; direct scope supersedes this, and docs edits are not authorized.

decision: Close and remote-scope stop requests use fresh transport identities without run-store intent/outcome receipts. Ordinary session/turn/command recording and writer exclusivity remain unchanged.
decision: Event callbacks and observer closure share lifecycleMu; terminal callbacks cannot reopen logs. Remote service result readers can abandon an unavailable final result after the existing bounded wait, then release local capture handles. Scope exit clears contextual values and owned references.
decision: Removed orphan-only runstate inspection/action helpers and observation release projection helpers once their callers disappeared. Agent-executor local stop retries and conversation/environment management remain: these are executing-node or separate interactive resource owners, not a core workflow cancellation ledger.
friction: This fresh worktree had no frontend dependencies/assets. Root checkout dependencies had an older skgo adapter, so installed the exact lockfile locally and generated/built once. Global vp test used incompatible Vite internals; the installed web/node_modules/.bin/vp ran the affected browser tests successfully.
correction: TestScopeData formerly read child values after scope exit. The approved contextual-lifetime requirement makes that expectation invalid; the test now expects only the still-live parent's values.
decision: Verification covered affected Go packages, focused race checks including session admission and missing remote final output, svelte-check, 24 affected browser checks, and the existing HTTP cancellation control tests. Live Codex luna normal and timed-out turns exited and saved their distinct outcomes; native command execution/readback and cancellation also behaved as requested. Private operating output stayed outside Git.
