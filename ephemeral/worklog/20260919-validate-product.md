# Product validation workflow

decision: User requested implementation followed by exactly one adversarial review through gimble run-prompt, then feedback. No merge/install or iterative review loop is requested for this change.
decision: Keep project-specific preparation/startup/readiness, target addresses, and plain-English features in JSON/YAML input. Ordinary Go performs service ownership, per-feature recording, operation, independent validation, and reporting.
friction: Opus pilot established that Playwright CLI daemonizes, video-stop is mandatory, and cancellation cleanup needs a fresh context. GoTTY worked as the macOS terminal fallback. SIGKILL cannot guarantee cleanup or finalized video; preserve that limitation.
