# Sprint 001 draft critique

doc_bug: Claude's Phase 2 expects cacheWrite.unknown=2 for the recorded Antigravity fixture, but its specified rules count all three completed assistant rows (two accounting records and one tool-only row without accounting), yielding 3. Resolve accounting eligibility before selecting the expected value.
doc_bug: Gemini's proposed TaskInfo summary/goal fields do not preserve the current Task name, description, definition_of_done, and validation payload. Its promise to persist Go rollups also has no destination in the proposed snapshot shape.
decision: Review only the two requested drafts against the intent and current code. Write the critique; do not implement the sprint, edit competing drafts, or commit without Tyler's request.
