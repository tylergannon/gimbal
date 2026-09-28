# Browser evaluator build authority

2026-09-27. User authorizes implementation of the following complete scope and
explicitly requires Gimbal and subagents. Work on this existing task worktree;
preserve others' edits. The manager owns integration, commits and live validation.

1. NewBrowser(ctx, ...) returns a browser handle and error, using idiomatic Go.
2. WithBrowser gives an agent working access to that specific browser.
3. Connector details stay outside workflow code.
4. Browser belongs to its creation scope and survives individual Generate calls.
5. Linter rejects handle escapes beyond the owner in supported authoring forms.
6. Normal completion/cancellation stop dependent agent work and release owned
   resources in their environment; partial startup/cleanup failures are errors.
7. Backend supplies worker binary at startup independently of project image.
8. Prepared image includes app, harness, browser and media tools without repeated
   agent-led setup.
9. Go sequencing, validation, retries and supervision control stay controller-side.
   No generated remote subgraphs or Go-state serialization.
10. Principal/supervisor agents can run concurrently in one worker/live workspace.
11. Activity reaches supervision; objections can steer the running principal.
12. Agents can access supplied scoped information and referenced files.
13. Existing validate-product performs a real browser evaluation on Docker/Temporal.
14. Accessible reports/screenshots/finalized video; record stop precedes processing.
15. Simple workflow sequence and repeatable configuration/invocation.

Keep one browser connector and one harness for this first supported example.
Current local Docker/Temporal/shared-path arrangement is allowed. Use existing
playwright-cli as first connector if practical; a browser-specific integration
can hide open/video/close commands and inject session-specific shell instructions.
Do not create a generic resource registry/framework. Browser access is explicitly
granted; do not implicitly let supervisors drive it. Application startup remains
Service. Sessions/browser/service can outlive an individual Generate.

Consumer image must not include the Gimbal worker: local Docker injects a pinned,
compatible Linux binary (read-only mount/explicit entrypoint is the candidate).
Keep auth secrets and login state out of Git; do not copy host login state into
the worker. Use existing supported secret configuration; report live auth honestly.

Historical inspected head: 23c02f6, PR398. Existing code already places principal
and supervisor calls in one worker; timers, transcript buffer and arbitrary typed
validation remain in controller. Preserve supervision lifetime across typed retries.
Existing supervisor context is bounded; don't promise universal full-scope inheritance.

Runtime code belongs here in Gimbal, not gimbal-view. This repository permits
authored notes under ephemeral but not source/test programs or run transcripts.
Put product code/tests in maintained paths, generated files via their generators.
External operational files/logs go under /tmp/gimbal-browser-build.AfTh8P.
No unrelated cleanup, public issue creation, framework/recovery work, or UI redesign.

Definition-of-done is actual behavior, not just passing build. The manager will
run the real evaluator and bounded supervised/cancellation cases, inspect media,
and obtain independent review. Workers run focused tests for their owned changes.
