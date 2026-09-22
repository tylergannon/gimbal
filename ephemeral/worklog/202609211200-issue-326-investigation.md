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
