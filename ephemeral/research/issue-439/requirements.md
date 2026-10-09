# Authoritative request

Read issue 439, think about design, and write a two-page proposal. Use the consensus and request-adversarial-review skills to obtain independent Claude Fable review and resolve material findings. This is design work, not implementation.

The main question is the most elegant and idiomatic data interchange between ongoing workflow runs and the central server: whether the server pulls from each control socket or runs push. Expected concurrency is tens of runs per computer, not hundreds. Anticipate important IPC needs without overdesigning.

Additional user requirements: separate processes and sockets should leave open running workflows on other machines or in containers; individual workflow processes should load minimal material and should not load the entire web application.

Issue snapshot: issue.json beside this file. Repository instructions and current public Godoc remain authoritative. Initial workflow authoring/discovery must not be gated on seamless application upgrades.
