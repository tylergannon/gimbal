# Native plugin authoring seam

- decision: runtime workflow definitions are the sole graph/catalog/admission authority; generated graphs do not register from init, and runtime CLI commands are built from central descriptions rather than generated stock commands.
- friction: Polytype requires schema declarations before a new ordinary local Generate output type compiles -> installed generation discovers concrete local output types and generates jsonschema-only declaration stubs before running the caller's pinned tool; stale normal generated definitions/graphs are overlaid during extraction.
- decision: optional primitive inputs use polytype.Optional and disallow null; schema validation preserves field-addressable errors before concrete invocation decoding. JSON names honor explicit field tags while CLI flags remain derived from Go names.
- doc_bug: service-check lacked a devops-tasks default, conflicting with required plugin role defaults -> built-in migration now supplies gpt-5.6-luna and updates its source help.
- friction: Cobra searches for commands before initializing its help flags; deferred dynamic command parsing needs explicit Args contracts to avoid treating a flag value as a command during catalog listing help.
