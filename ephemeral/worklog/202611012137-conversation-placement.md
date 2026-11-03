# Conversation placement prerequisite

- decision: Full matching-request replay is required for execution waits because the caller and worker clocks may differ; a diagnostic timestamp cannot define completeness.
- friction: The current River worker has a single configured queue and no exclusive logical registration or retained-owner addressing; the existing conversation path still creates a physical environment binding and performs separate preparation. These seams remain to be migrated together before C1 can be claimed.
- decision: A redelivered retained completion must reuse the event ID persisted by the native owner so event storage deduplicates it. This does not yet provide startup-driven recovery publication.
- correction: The selected namespace bus must reach conversation host launch independently of the logical environment input. Preserve the selected TaskRoute name/configuration; preflight rejects physical placement fields and unsupported physical transports. River namespace interpretation and delivery remain the next task.
- friction: Command-package readiness tests require the generated web manifest; run `just build` in a fresh worktree before those tests. Regeneration must include workflow graphs as well as the application and TypeScript schema.
- decision: `service.go` is an additional production owner outside the parent file map: services can launch local processes, so the explicit no-local-execution rule requires the same selection guard as commands and sessions.
