Read AGENTS.md, ephemeral/browser-evaluator-build.md, the current design in
ephemeral/browser-implementation-design.md, and the complete independent review
ephemeral/reviews/browser-design-round-01.md. All five findings are accepted.
Revise only ephemeral/browser-implementation-design.md to resolve them. No
implementation, commits, other agents, or Gimbal invocation. You are not alone;
preserve others' edits. The prior design agent ran through Gimbal; its complete
design and review are the handoff because run-prompt cannot resume ended runs.

Keep the design small and concrete. The controller still owns ordinary Go;
do not create remote subgraphs or a generic resource framework. For the
unconfirmed remote cancellation path, define the smallest concrete action
that stops dependent work before claiming release, with errors surfaced;
merely declaring claim 6 unsupported is not a resolution. Existing timeout
and backend teardown mechanisms may suffice. Do not invent recovery.

Additional manager finding: a fixed workdir/browser wrapper is overwritten
when two browsers share a workdir. Each supplied handle must keep a distinct
command. Fix that directly. A generated shell wrapper may guard lifecycle
commands, but do not claim this sandboxes full-access agents. The image probe
at /tmp/gimbal-browser-build.AfTh8P/image/README.md is actual new evidence.
Use its working image/config, disable browser idle expiration, and finalize
recording before close. Keep the image build artifacts external as already
assigned; track B owns only runtime binary injection and related source/tests,
not a second speculative image. Remove the old image-baked-worker recipe or
convert it coherently to a worker build artifact plus external consumer image.

Do not add a worker protocol-version mechanism solely for hypothetical future
compatibility: the supported recipe builds the injected binary from the same
checkout as the controller, and startup validates it. Keep operator build and
architecture requirements explicit. Real authentication is still pending the
user's choice; do not copy host login state or assume a new auth API.

The concrete external suite is /tmp/gimbal-browser-build.AfTh8P/evaluation/suite.yaml;
the backend config draft is /tmp/gimbal-browser-build.AfTh8P/execution.json.
Use canonical /private/tmp paths for shared data. Include the correct server
startup, gimbal run validate-product invocation, all three Codex role flags,
and mounts for inputs, workspace, run context files and outputs. Generated
workflow metadata must be regenerated, not held unchanged.

Finish with coherent implementer file ownership. No need to repeat the whole
codebase investigation: inspect only what resolving these findings requires.
