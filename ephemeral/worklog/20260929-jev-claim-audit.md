# Jev index-claim audit research

decision: Tyler requests exhaustive source checking and contradiction marking,
with no request-count constraint. Earlier sampled, advisory index feedback is
not the scope of this request.

finding: Current research-document validates source counts and nonempty indexes;
it relies on prompts/editorial review for factual support. Audit insertion points
are immediately after initial curation and after editorial gap-index repair.

decision: Propose a synchronous compiled audit command using RunCommand, keeping
repair control flow in research-document. Ordinary Go already permits Jev calls;
the missing primitive concerns graph observation, not execution capability.

finding: Separate source entailment from inter-claim compatibility. Numerical
limits on different quantities can be falsely classified as contradictions;
retain measurement scope and unresolved findings instead of selecting a winner.

friction: TYPESAFE_API_KEY was absent from the shell but available through
Gimbal's existing config/Doppler secret resolution. Check that path before
claiming live Jev access is unavailable; never display credential values.

decision: Keep downloaded official sources in the local cache and operational
probe output outside Git. Commit only the research proposal and source catalog.
