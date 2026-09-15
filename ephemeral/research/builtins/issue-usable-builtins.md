Make Gimble usable for someone arriving with a goal, claim, design, or implementation plan. This is higher priority than viewer work. Adapt the ideas in df-sprint-plan, df-sprint-execute, and df-easy-loop-e2e into ordinary Gimble workflows, not their orchestration machinery.

- A guided CLI asks only useful clarifying questions and recommends an explicit lfg, plan, or sprint route based on task size and uncertainty. Users can also choose a route directly.
- lfg is one supervised worker session for a small clear task, with scoped inputs and optional explicit verification commands. A supplied failed check must fail the run. Without a check, successful worker completion is not an independent acceptance verdict.
- Planning uses independent Codex/Claude/Gemini drafts, cross-critiques, human refinement when needed, and a saved implementation plan.
- Sprint accepts richer goal/plan/acceptance/constraints/context/check inputs, works in general repositories, uses scoped data and supervisors, and independently validates the requested result. Planning output flows into execution.
- The original goal stays authoritative. Supervision steers rather than gates; failed deterministic checks cannot be overridden by prose. Workflow sites and context keys remain constant and statically legible.

Prove a small real lfg task, a real generated plan, and sprint execution using that plan. Preserve the run records and produced artifacts. Test cancellation, invalid input, failed checks, and human question handoffs. Keep graph extraction/recording and viewer implementation in their own tasks.
