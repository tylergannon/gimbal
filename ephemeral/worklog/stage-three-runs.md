# Stage three Runs integration

decision: Retained run panels own explicit project/run identity, distinct form instances and selected-panel keyboard handling. Hidden panels remain observing; teardown closes observation only.

friction: Go does not permit bracketed route directory import paths. Share the existing run registry reads in web/runview and keep both route loads and typed remotes as thin consumers.

friction: Fresh task worktree requires frontend dependencies before SKGO generation and built assets before web Go tests. Install/build before those checks; missing manifests are build prerequisites, not product failures.

correction: Latest delegated Designer key from the planning chat works today. All three current stage-three references match the named earlier versions.

friction: Native hydration set page.status to 500 while SSR and data requests remained 200. RunPanel's deep $state proxied the snapshot; RunObservation's native structuredClone rejects proxies. Keep the immutable preload in $state.raw and exercise the panel itself in browser regressions.

friction: SvelteKit 3.0.1 query client does not return the fetched response value. Client-only ordinary queries resolve undefined while SSR-seeded data can mask it. Carry a reproducible package patch returning result._; native restoration must exercise a previously hidden, unseeded query.

correction: Green RunView-only tests did not cover the retained panel's state carrier or the real remote-function client. Native observation and an independent validator identified both missing boundaries.

friction: New runs.remote.ts was not in the central's explicit-project shared remote allowlist. Same payload returned plain 404 without Referer and 200 with project Referer. Add this validated explicit-project module alongside workspace/transcript reads and exercise actual HTTP requests with no Referer and with a different displayed project.

friction: Native Computer Use became locked during final acceptance. Reset the REPL and restarted the exact socket-owning SkyComputerUseService; automatic unlock still failed. Requested manual unlock. This is a desktop access blocker, distinct from an application permission grant or missing Gimbal build identity.


friction: A literal pending snippet in SvelteBoundary replaces all server-rendered children, even seeded synchronous panels. Remove the new workspace pending snippet; keep panel-owned loading states and the error boundary. Full web/host SSR tests now pass.

validation: Manual unlock restored native control. Final native candidate restored mixed run/chat/file identities, rendered the synthetic README, retained an unsent draft and slash input across run/file visits, retained Watchboard and map selection/80-percent zoom, preserved Runs Cancelled filter and exact-ID search, and admitted workflow-launch links into the same run tabs. Show Runs and sidebar Runs converge. Real Haiku runs demonstrated isolated cancellation and close/reopen without stopping execution. Promises empty entry and Recording menu remained accessible. Native needs-human, connection-loss and detailed reading-scroll cases remain unverified rather than inferred from tests.
