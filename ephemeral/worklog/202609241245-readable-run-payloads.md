decision: The run detail pane's shared Payload component is the main human-facing path for structured results, scope values, watcher results, and tool output. Render JSON objects and arrays there as labeled fields and cards by default, with raw JSON available on demand.
decision: JSON-encoded string results should display as ordinary text with real line breaks; the recorded bytes remain available through Copy.
friction: The existing watcher test asserted serialized JSON punctuation in the visible Result pane, so it had to assert the new readable fields instead.
