Build a dependency-aware merge-queue CLI in ordinary Go, using the standard library.
Read one JSON object from stdin: {"prs":[{"id":1,"deps":[],"checks":"pass"}]}.
IDs must be positive and unique. checks is pass, fail, or pending. A dependency must name a PR in the input. Reject malformed JSON, extra JSON values, invalid status, invalid IDs, duplicate IDs, missing dependencies and any cycle (including cycles among failed PRs), with exit 2, a useful stderr diagnostic and no stdout.
On valid input return {"waves":[[1,2],[3]],"blocked":[4,5]}. Each wave contains every passing PR whose dependencies were in earlier waves; ascending IDs per wave. Nonpassing PRs and everything transitively depending on them go into blocked, sorted. Empty arrays must be [], never null. Input order must not change the result. Empty queue works.
Supply tests and a short README with runnable input/output examples. The workflow will run an external black-box acceptance program and independent agent validation. Preserve every requirement; do not change the external acceptance program.

New requirement disclosed after the first successful base acceptance: add --parallel N to limit each wave to at most N ready PRs, choosing lowest IDs first. The default is unlimited. Reject missing, zero, negative, or non-integer N with exit 2 and no stdout. Document the flag.
