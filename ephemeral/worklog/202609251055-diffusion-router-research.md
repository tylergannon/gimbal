# Diffusion Router research worklog

decision: Store the portal's full Connect guide inventory under ephemeral/projects/gimble/diffusion-router-research/portal, grouped by coding, agents, and API surfaces; keep exact JSON alongside readable Markdown and source provenance.

doc_bug: Router OpenCode guide reviewed 2026-09-20 mixes singular `provider` and plural `providers`; installed OpenCode 1.2.27 rejects the plural field -> qualify a version-specific configuration before recommending it.

correction: The preserved Gimble Codex adapter starts an app-server per turn and resumes its thread; the user's global-server concern describes another setup or an intended architecture, not that preserved adapter.

decision: Keep live Router inference out of this documentation pass until a profile-specific credential path and proof matrix are defined; a catalog listing or short CLI reply does not establish coding-agent compatibility.

correction: User added Pi as a potentially preferable new harness adapter; assess its native RPC controls against Gimble's full session contract before ranking implementation paths.

decision: Pi 0.87.1 RPC is the strongest new live-control candidate for Go because a local mock proved configuration, events, and cross-process resume; qualify real Router tool calls and exact structured output before choosing it over Codex or OpenCode.

friction: Portal's authenticated catalog API returned 401 to unauthenticated curl, while Safari's existing login exposed the 22-ID JSON without touching keys -> use the browser session for read-only catalog capture and record its fetch timestamp.
