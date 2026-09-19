# OpenCode harness research

correction: GitHub release `target_commitish` is not authoritative for the release tag's actual commit. For OpenCode v1.18.27 it names b04697366f05419e9bd7a92f841813dd976161c9, while `git ls-remote` resolves the tag to 4b7e19e315cca414121ba1d61523fef74bb3ae8b. Resolve the tag before comparing package versions or claiming installed/source drift.

decision: Inspect the installed OpenCode server's `/doc` alongside the website. Version 1.18.27 declares both legacy endpoints and an experimental `/api` family with distinct prompt-delivery and durable-event contracts; a legacy-only comparison would miss decision-relevant capabilities.

correction: Gimble steering requires delivery into the active RunTurn. Consumption at the next model/tool boundary within that same turn can qualify; mid-token provider interruption is not required.

friction: Two research lanes wrote the shared final-report path before their assigned indexes were handed off, delaying synthesis. Steer researchers back to their owned topic directories and leave the complete report to the author role.

correction: The upstream Go SDK exists, but its inspected generated surface lacks fork, async prompt, schema format, and the newer API routes. Verify concrete SDK coverage rather than inferring availability from a JS/TS-only research corpus.

friction: Links from ephemeral/research/opencode to the local .gimble corpus require three parent levels. Check report reference targets after synthesis.
