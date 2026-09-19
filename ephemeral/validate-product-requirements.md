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
including incomplete runs. A passing feature needs actual recorded interaction
and supporting evidence assessed by a separate agent. A failed expectation must
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
