# Runtime public API extraction

correction: This repository uses Go 1.27.1 with generic methods; follow Session.Generate[T] and expose Session.GenerateResponse[T], rather than assuming older Go restrictions.
friction: Renaming plan to Plan on macOS caused Polytype cleanup to delete its new case-only schema path. A second root generation recreated Plan.json; explicit Git rename preserves the case on Linux.
decision: Keep contextdata.Store physical storage/cache fields and the existing semantics; retire compiledscope callbacks and any-session/raw-task bridges completely. Public CheckContext returns the snapshot; the canonical command record lives under its authored key.
