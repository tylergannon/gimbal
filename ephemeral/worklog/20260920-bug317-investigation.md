# Bug 317 investigation

decision: Investigate structured-output and native wakeup semantics before selecting a repair; application behavior remains unchanged pending that choice.

friction: Normalized events hide the orphan-task result that can precede a resumed prompt's answer -> use raw SDK events and native session history when investigating result attribution.

decision: A valid structured result and native terminal_reason completed can coexist with unfinished background work; neither establishes assignment completion. Origin distinguishes task-notification results from ordinary prompt results in the observed cases.

correction: User emphasized that disabling background work would substantially constrain Claude -> prefer supported persistent streaming with a session-owned process; per-turn teardown is an adapter choice. Distinguish process lifetime from assignment completion, and verify per-turn schema changes before implementation.

decision: User requested independent Sol and Terra investigations plus a concurrent research-document run; wrote local common/role briefs and cached issue 317 before dispatch. Investigative Claude probes use Haiku; documentary workflow uses Gemini Flash in all roles.

friction: Literature collection overstated public reports as current facts, labeled closed issues open, and promoted suggested causes to established ones -> independently fetched issue bodies/comments, used Sol for a bounded source audit, and steered curator to repair sources, clips, and indexes before authoring.

decision: Sol observed waiting and completed structured successes with active background work; a persistent service survived legitimate assignment completion. Task counts and native terminal reasons cannot replace explicit task semantics.

decision: Terra observed acknowledged schema-B reinitialization and literal jsonSchema null both leave schema A enforced on CLI 2.1.270, including a late notification. Persistent-session design must explicitly settle per-call schema enforcement; SDK acknowledgement alone is not behavioral proof.

clarification: Schema mutation is relevant only to successive differently typed Generate calls in one persistent session; it is not required for a waiting answer and automatic continuation of the same assignment. Keep the completion question separate from that compatibility decision.

friction: The documentary editor incorrectly asserted TaskOutput was absent from the official tools reference. Direct primary-page inspection showed an explicit deprecation entry -> corrected the final report from the current source rather than treating editorial acceptance as factual proof.

correction: Server survey missed Remote Control server mode because top-level help omits it. Official remote-control docs plus subcommand help confirm multi-session capacity; embedded 2.1.270 bridge code confirms child_process.spawn per session. Distinguish a shared server entrypoint from shared in-process session execution.

correction: User proposed retaining Claude for one logical Generate rather than the whole Session. Live Haiku probe passed waiting -> automatic completed -> EOF -> resume schema B -> SIGTERM -> resume schema C -> EOF -> resume plain text, with one conversation ID and context preserved. Schema stickiness was limited to tested in-process reinitialization; process-per-logical-call is the smaller direction when no live service must span calls.

correction: User wanted Gimble with lower models to do the heavy lifting. Direct df-sprint-plan draft/critique CLI launches used Fable/Astra/default Gemini and missed that routing intent. No harness changes were made. Remaining plan synthesis and independent validation were assigned through Gimble to Terra and Sol, with an explicit documentation-only boundary. User routing overrides skill defaults.

decision: User expressly requested a temporary acceptance workflow. It lives outside the checkout at /private/tmp/gimble-317-sprint/ and exercises the real public Run/Session/Generate path with Haiku. Its current-harness baseline returned a typed waiting response before background completion and failed EARLY_RETURN. Sprint completion must require the same two-call scenario to pass after implementation; planning does not itself authorize the repair.

decision: Gimble run 01M2ZXA75D0Z3TY09HAMJR98DS.implement finished successfully using Terra for planning/drafting/critique and Sol for independent validation. The assignment owned only Sprint 002 and its merge notes. Sol validated the documents against the existing fixture and raw red baseline, with no gaps; this does not satisfy the future implementation gate. Sprint status remains planned.

correction: User authorized implementing Sprint 002 through Gimble, then explicitly selected Sol medium for coding and Sol high for review. No Terra implementation run was launched. Planning and architectural critique also use Sol high; Claude Haiku is reserved for the required adapter acceptance.
