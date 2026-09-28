Implement the backend portion of the accepted current
ephemeral/browser-implementation-design.md against
ephemeral/browser-evaluator-build.md. Read repository instructions. You are
not alone; preserve others' edits and accommodate their work.

Own internal/execution source/tests, codex/codex.go and codex/codex_test.go for
the accepted unconfirmed-interruption handling, required cmd/gimbal execution-config
validation/tests, Dockerfile.gimbal-worker, docker-compose.temporal.yaml,
justfile for the separate Linux worker build, and
ephemeral/temporal-backend-api.md for exact startup configuration. Do not edit
browser core, linter, generator or validateproduct. Do not create a second
consumer image: the tested image and recipe are external at
/tmp/gimbal-browser-build.AfTh8P/image. The manager owns that image and final
live suite. No Gimbal binary belongs in the consumer image.

Supply the configured Linux worker through a read-only startup mount and
explicit entrypoint. Resolve the accepted cancellation/shutdown race with
the smallest existing-mechanism change. Surface cleanup errors; no recovery,
generic environment framework or speculative worker protocol-version scheme.
Keep existing credential behavior until the user chooses authentication; do
not read or copy host login state or provision credentials.

Run meaningful focused tests and, if useful, the existing no-model readiness
integration test against our task Temporal/Postgres and tested consumer image.
Do not interfere with other containers or change shared server state. Tests
must use their existing unique environment IDs and clean their own resources.
Task runtime config is /tmp/gimbal-browser-build.AfTh8P/execution.json;
build your changed worker to a distinct external path if needed.

Do not commit, launch agents, invoke Gimbal, or hand-edit generated code.
Report files, actual tests/results and any unresolved requirement. The manager
will integrate and run the actual authenticated evaluator.
