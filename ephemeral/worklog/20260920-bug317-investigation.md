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
