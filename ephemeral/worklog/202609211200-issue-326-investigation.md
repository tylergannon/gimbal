# Issue 326 investigation

correction: Tyler states Gimble's prescribed process model is single-process only; do not explain the 43840/43841 observation by assuming a supported standalone-viewer plus delegated-runtime topology.

decision: Before reproduction or repair, recover the actual evaluator and runtime topology, enumerate the exact skgo/Gimble 500 paths, and distinguish observed evidence from hypotheses through two research-document runs.

outcome: The first browser state was a healthy empty project. The 500 appeared only after the evaluator launched a second Gimble process against the same project directory. All eight observation tables existed from initialization, contradicting the missing-table explanation.

outcome: The raw server error was not retained. A partial read of the actively appended session JSONL and a foreign viewer writing an incompatible observation snapshot are both source-supported causes. The pinned skgo data-error path sanitized either cause and did not log it.

outcome: The historical foreign-run test deliberately renders an active store owned by another registry, but serializes every mutation and render. It proves intended semantics without exercising concurrent file I/O.

outcome: The supported one-process path serves a registered live run from memory. A narrow directory-publication-before-registration window remains source-visible and should be tested deterministically before claiming the prescribed path is race-free.

correction: The one-process invariant is Tyler's explicit architecture requirement; neither the issue-era nor current AGENTS.md literally contains the sentence attributed to it by the generated research draft. The final research artifact was corrected against primary source.

correction: Tyler challenged the later plan's assumption that one process must mean one process per project. The desired direction permits arbitrarily many projects per Gimble instance, with a possible one-instance-per-machine end state. Treat the current project-scoped `web.Runtime` and API design record as implementation/history to migrate, not as the final ownership boundary.

decision: A per-project owner is only a candidate containment stage for issue #326, not the recommended destination. Ask Fable 5.1 to challenge a host-plus-project-state plan and the risk of hardening a temporary per-project process model before selecting the implementation game plan.

correction: Tyler explicitly stipulated that more than one Gimble instance must always be permissible; the goal is that one instance is sufficient for many projects, not a machine-enforced singleton. Instance communication endpoints must be configurable for independent testing. Same-project concurrent attachment/write semantics are a distinct unresolved question; do not smuggle a machine-wide lock into that decision.

decision: Six Fable 5.1 review rounds ended with no findings on the multi-project plan. Material corrections included failure isolation for workflow panics, per-run environment authority, physical project identity, client/instance version negotiation, explicit instance selection, UDS and browser trust boundaries, and unknown launch outcome across a process restart. These are design requirements, not implemented behavior.

friction: A host-only Unix-socket protection was insufficient because the loopback web listener already exposes mutating controls to other local users. Ordinary cookies also cross loopback ports and would couple independently permitted instances. The revised plan uses a port-scoped HTTP authentication challenge and requires same-browser multi-instance and adversarial-third-port proof; still verify browser behavior before implementation claims.

correction: Tyler rejected that security-focused destination, not merely its position in the migration order. Gimble is a local single-user tool; coherence, simplicity, and useful functionality are the priorities. The authentication, credential-isolation, and elaborate protocol/lost-response requirements above are superseded by the simplified single-instance plan, not implementation requirements.

decision: Fable 5.1's fresh review of the simplified plan identified four functional gaps: a workflow panic must not kill an unrelated project in the shared host; hosted runs need a stated environment source; a second instance must exercise the chosen same-project ownership behavior; and the publication window must not be represented as the unproven cause of #326. The plan now addresses these without restoring the security framework, and Fable's next full review found no material findings. This remains design consensus, not runtime proof or diagnosis of the historical 500.

friction: The installed `gimble run implement` was still the superseded single-promise workflow, while the ordered-outcome replacement existed only as unfinished source edits. Tyler pointed out that `go run ./cmd/gimble` can execute the checkout directly; finish generation and wiring, then launch that command rather than treating installation as a blocker.

correction: Do not finish a turn while an hours-long workflow is still running and imply later phase monitoring without an actual wake mechanism. The current app had no automation update tool available; keeping the turn active and checking the run was the only available monitoring path here.

correction: Internal polling is for the agent's awareness. Tyler wants occasional substantive updates, not repetitive unchanged-state commentary every minute; report meaningful phase results, blockers, and final proof instead.

friction: The outcome-loop run finished with all seven validator passes, but browser e2e was still 5/6 because its saved implement fixture used the superseded root-level `implementation.1` graph shape. Update the fixture's scopes to `outcome.1/implementation.1`; the subsequent real browser run passed 6/6. Keep post-run checks separate from the workflow's aggregate ok verdict.
