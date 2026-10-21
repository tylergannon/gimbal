correction: Continue the browser migration past the initial research. A rendering mismatch or one unsuccessful provider configuration is not a completion boundary.
decision: Investigate released and nightly Lightpanda with ordinary locator interactions; keep protocol, iframe sizing, test-detail assertions, and application behavior as separate causes.
friction: Lightpanda 1.0.1 predates native iframe input fixes. Current nightly plus explicit HTML iframe dimensions allows ordinary clicks through the Vitest iframe.
friction: A transitive static import blocks the browser WebSocket needed by a lazy manual mock factory. Resolving manual factories during registration unblocks the existing question-answering test; this timing change needs qualification with importOriginal factories.
decision: Disable the inherited Chromium prewarm and advertise sequential browser execution. Lightpanda permits only one browser context per CDP connection, so default Vitest parallel context creation is invalid.
