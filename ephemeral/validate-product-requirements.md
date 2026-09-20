# Product validation workflow: user intent

The user wants a daily process that obtains the latest Gimble build, actually uses
its features end to end, and records video. They proposed a built-in workflow whose
JSON or YAML inputs describe the product's startup or existing endpoint and the
features to verify. Support browser and CLI frontends. Project-specific details
should be parameters; use Gimble as the first concrete example. Keep the design
simple and capable, using ordinary Go workflow code and existing service support.

This implementation task is to write that workflow and ask for one adversarial
review through `gimble run-prompt`, then bring the feedback to the user. No merge or
installation is requested in this step. Scheduling and obtaining a new build can
be supplied externally or through the suite's prepare command; there is no request
for a scheduler framework, product auto-repair, or a model evaluation framework.

Expected reporting preserves every declared feature as pass, fail, or blocked,
including incomplete runs. A passing feature needs actual interaction with the product and a useful
explanation supported by screenshots or command output. A failed expectation must
not disappear through retries. Owned target/browser/terminal resources and video
recording need cleanup on success, failure, and ordinary cancellation. Hard kill
and machine failure are acknowledged limitations.

The earlier Opus feasibility pilot and its evidence are local at
/private/tmp/gimble-opus-validation-artifacts/REPORT.md. That pilot is evidence for
the tools and lifecycle approach, not proof of this new workflow's correctness.

Follow-up user request: fix both findings in
`ephemeral/reviews/20260919-validate-product-round-01.md`, then obtain an
adversarial review from Fable 5.1. The two findings concern evidence/report
ownership and report completion claims that precede the outer run's agent-session
cleanup outcome. No merge or installation was requested in this follow-up.


Latest user clarification: screenshots should be taken by the exercising agent at
appropriate moments. Video is intended for optional human review. Prioritize the
80/20 case and remove redundant validation machinery. The simplified shape is one
agent per feature that exercises and assesses the product, saves screenshots and
CLI output, and returns its result; recording remains workflow-owned. Earlier
independent-agent and video-analysis requirements in this design are superseded.


Current implementation request: write the practical workload workflow above,
keeping it simple, flat, and as non-dynamic as possible. This replaces the earlier
feature-by-feature loop. Use caller-defined practical assignments, parallel user
sessions, a Gemini Flash screenshot/caption review, and final senior-agent issue
triage. Never inspect the primary product A's implementation source; project B's
source is permitted but the tester should normally rely on A to do that work.
