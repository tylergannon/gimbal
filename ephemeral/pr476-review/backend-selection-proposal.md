# Proposal for #479: independent backend selection

Keep messaging, environment orchestration, and secrets as independently selected dependencies. Register orchestrator and secrets constructors by name with validated configuration, using the registry's existing registration pattern. A single plugin may register any combination; it does not have to supply messaging to supply secrets.

Keep `Route.Backends` as the configuration location, but replace its implicit local configuration with named `Orchestrator` and `Secrets` selections, each containing a name and configuration. A route validates and constructs those selections before work starts. Messaging receives the constructed dependencies; it neither chooses nor constructs them. Remove `MessagingDefinition.ConstructBackends` and the implicit local fallback rather than supporting two configuration grammars.

Move the subprocess orchestrator into a backend package. Give it the narrow routing capability from #478 and explicit daemon bootstrap dependencies; it must not import the root registry or rely on a process-global registry. Keep private file secrets and in-memory secrets as separate registered implementations. The daemon receives the selected secrets configuration at startup rather than an unconditional `--secrets-directory` flag.

No new cloud or secrets provider is part of this change. Proof is that a second small secrets implementation can be selected without changing the messaging plugin or orchestrator, and the existing three messaging transports still complete `service-check` with cleanup.

This is a proposal for approval, not authorization to implement. #478 and #479 remain follow-up work after #476, per the owner's review.
