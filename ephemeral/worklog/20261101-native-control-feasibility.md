# Native-control feasibility

decision: Tyler accepts Desktop-dependent native control and prefers sharing one application instance. Source: this chat's voice interview, November 1.
decision: Prove make-or-break interface availability before substantial integration work; retain a reviewed configuration/interface coverage checklist, including whether Desktop itself can expose a control socket.
correction: Connection selectors in Desktop must be distinguished from an externally attachable listener and from native Locked Use eligibility. Installed source reveals WebSocket/local-daemon selectors that earlier bridge proposals must account for.
friction: Root Gimbal checkout has unrelated uncommitted test edits and run state -> created an isolated task worktree at existing HEAD; did not fast-forward or disturb root.
decision: Existing written-spec approval gate remains; voice acceptance of the Desktop boundary and request to continue are not approval of an as-yet-unwritten spec or authorization to replay the rejected installation.
correction: The existing control-socket path is a symlink. Inspect its resolved target before deciding whether it has a live holder; its current holder is the standalone daemon, not Desktop's stdio child.
decision: Keep the historical default worker ledger untouched and require an explicit qualification-only worker directory. Absence of that qualification directory never implies absence of historical state elsewhere.
friction: Static inspection of only the main Desktop bundle misses renderer-defined task tools. Follow the actual dynamic registry/dispatch before inferring that task-read/archive are absent.
correction: The installed main module's sole explicit MCP reload call is scoped to the internal-testing marketplace, not a generic user-marketplace refresh. Earlier source-report inference must not be treated as worker activation evidence.
decision: Sonnet reviews retained under ephemeral/reviews/native-control-round-01.md onward, session b6363a3f-c836-4e8b-a4ce-d22b19968c95. Harness metadata confirms Sonnet selection. Runtime experiments and Terra implementation have not started.
decision: Round 07 concluded only nitpicks remain. The reviewed audit certifies its explicit inspection coverage, not runtime feasibility or universal absence of other interfaces. Spec/experiment approval remains pending.
