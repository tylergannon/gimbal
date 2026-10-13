# Optional authentication design

correction: Tyler wants application-owned OAuth endpoints and plugin protocol operations; a plugin HTTP handler obscured the intended seam.
decision: Tyler chose durable local users and accepts an additional interface implementation; design provider identity separately from durable customer-defined users.
decision: This task requests design and Claude Opus consensus only, not implementation or live authentication proof.
friction: The installed agent launcher parses flags before its positional workdir and still launched Fable with an explicit Opus override; both attempts failed with rate_limit before producing a review. Native Claude Code with --model claude-opus-5-5 confirmed Opus in init and assistant events. Preserve the native session for consensus rather than counting a different model.
decision: Opus round 01 identified material gaps in selection, origin, gate ordering, duplicate admission registries, and attribution. Revised the design to use explicit --auth-file/WithAuthFile startup selection, one application origin, auth before project routing, provisioned local users as admission, and unchanged operator/supervisor event semantics.
