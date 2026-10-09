# Project workflows and their owning server

## TL;DR

**Backends execute runs; one server durably owns and controls them.** One active server holds an OS lock on its persistent state directory. The server records every launch before starting it, remembers it through restarts, and can cancel one run or all its runs by identity. A disconnected runner remains on that list until its outcome and cleanup are established.

The server opens an HTTP subscription and each backend streams its logical run history back. A local runner is the simplest backend. Ordinary HTTP requests carry controls. Separate executable builds keep the website and historical UI state out of runners. Start locally with Unix sockets; keep the protocol usable across machines later.

## Choose a topic

Each topic starts with its own short answer, then gives the mechanics and reasons.

- [Ownership and restart](topics/ownership.md) — how the server remembers its runs, reconnects, and kills one or all.
- [Events and controls](topics/interchange.md) — subscription direction, replay, command results, and failure behavior.
- [Lean runners](topics/lean-runners.md) — separate executables, memory, and why the website needs no plugin.
- [Workflow discovery](topics/workflow-discovery.md) — project builds, CLI commands, and the design associated with each run.
- [Backend contract](topics/backend-contract.md) — distributed ownership, termination, event history, and backend validation.
- [Storage and remote execution](topics/storage-and-remote.md) — file ownership, artifacts, containers, and network reachability.
- [Delivery and review](topics/delivery-and-review.md) — the first implementation boundary, evidence to gather, and review status.

This is the editable proposal for [issue 439](https://github.com/tylergannon/gimbal/issues/439). Its [requirements](requirements.md) include the subsequent conversation. The linked Markdown files are the document; there is no separate PDF edition to keep synchronized.
