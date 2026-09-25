# Run dashboard

decision: User explicitly prioritized useful, correct dashboard delivery over polish, hypothetical races, and unrelated edge-case fixes. Manager owns scope and delivery; Gimbal implement owns implementation and independent validation.
doc_bug: skills/gimbal-runs still says review is the only built-in; installed help also exposes implement, pyramid-summary and research-document. Used actual command help; deferred instruction changes outside dashboard scope.
decision: User clarified the current feature revamps the existing listing page. Separate subsequent conversation-interface direction is captured in ephemeral/conversation-interface-direction.md and excluded from this implementation run.
friction: Browser refresh advanced elapsed time but a run completed by another Gimbal process stayed cached as running. A refreshing UI does not prove fresh run facts; exercise a cross-process completion before claiming live listing behavior.
