# Kit focused-form HMR

decision: Implement only Gimbal #601. The separately owned #603 validation-lifetime work is excluded. Preserve the existing Kit queue/query patch and add the focused-form listener correction there, using public svelte/events.on() and its returned disposer.

provenance: Svelte PR #13737 introduced the existing event-context protection for synchronous blur during DOM removal. Pinned Svelte 5.57.2 exposes it through public on(); no private helper copy or Vite HMR coordination is necessary. Three prior independent reviews support this integration boundary.

friction: Running vp install from the Go repository root creates unrelated package-manager scaffolding because declarations live in web/. Removed only that newly created scaffolding; subsequent frontend commands run from web/.

friction: The nested vp in the package test script loaded the global runtime and failed Kit's RunnableDevEnvironment check. Running the same suite through vp exec vitest run resolved the project's declared Vitest/Vite dependencies and passed; no ambient package-manager selection was used.

decision: Keep the implementation to the Kit listener registration and returned remover. Local browser qualification inspects the real form's touched state and tracks wrapped-listener removal across HMR and navigation; an empty-message submission reaches the Go handler's rejection before manager.Start, avoiding a model run. Validation-lifetime repair remains owned by the separate #603 task.

friction: Unified diff context in dependency patch files triggers staged whitespace rules (blank context markers and space before a tab). Keep the new hunk in the existing patch's zero-context format; reinstallation produced byte-identical runtime content after only the patch identity changed.

handoff: Independent implementation review found no material findings after repeating the real composer checks. Production generation restored the development-selected Go output. The build formatter also rewrote three unrelated generated TypeScript files; restored those task-created formatting differences to preserve the focused delivery scope. No proof scripts or run logs are committed.
