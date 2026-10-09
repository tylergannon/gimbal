# Authoritative request

Read issue 439 and think about design. The initial two-page request is superseded by the later request for an editable pyramid: TL;DR with links to drill down on each topic, not a PDF. Use the consensus and request-adversarial-review skills to obtain independent Claude Fable review and resolve material findings. This is design work, not implementation.

The main question is the most elegant and idiomatic data interchange between ongoing workflow runs and the central server: whether the server pulls from each control socket or runs push. Expected concurrency is tens of runs per computer, not hundreds. Anticipate important IPC needs without overdesigning.

Additional user requirements: separate processes and sockets should leave open running workflows on other machines or in containers; individual workflow processes should load minimal material and should not load the entire web application.

Issue snapshot: issue.json beside this file. Repository instructions and current public Godoc remain authoritative. Initial workflow authoring/discovery must not be gated on seamless application upgrades.

Follow-up question: should the website itself be a plugin loaded only in server mode, or is there a simpler way to keep satellite invocations lean? Evaluate that choice in the proposal.

Latest user clarification: the server must durably know every workflow run it started, including through restart, so it can kill one run or all its runs without filtering process listings. Independent processes must not become unmanaged.

Document preference: for this human-agent collaboration, provide a navigable pyramid with a TL;DR and topic-level drill-down; the PDF is rejected as the collaboration format.

The user also proposed having runners stream events into a directory for the central server to read, then recalled the separate-compute requirement. Compare that local simplification with a transport boundary without discarding the durable journal.

Latest annotations require one active server per state/run directory, with another server refusing concurrent access. Account for compiled distributed backends: each participating execution site must have run-owned subprocess lifetime management, and event-history design must not assume a common filesystem or one local process. Provide backend-author guidance and use targeted linters where appropriate; behavioral claims still require tests.

The same workflow must support tasks on Linux and tasks on macOS. Remotely invoked Gimbal agent tasks must be able to use computer use through Claude or ChatGPT sessions. The design must represent task-level execution requirements and remote session ownership, even when scheduling and integration are implemented by consumer-owned backends rather than the central server. This is a required target capability, not only whole-workflow remote placement.

The user suggested a macOS backend using a launchd-managed resident parent to launch run tasks with the desktop access unavailable to an earlier SSH-origin task. Treat it as a concrete candidate backend and consult the earlier investigation; do not assume launchd or elevated privileges alone fix provider/session authorization.
