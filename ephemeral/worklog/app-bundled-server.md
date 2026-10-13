# App bundled server

correction: User requires the macOS app to start its own bundled server when GIMBAL_SERVER_URL is absent; installed app must work without a manually started CLI. Explicit URL continues to select an external server.
decision: Durable application state belongs centrally in ~/.gimbal; app launch must not derive its store or project identity from Finder's working directory.
skill_issue: gimbal-release source=skills/gimbal-release/SKILL.md severity=critical -> separate-server packaging instruction superseded by explicit user requirement; update maintained instructions with behavior.
friction: Release proof used a temporary external server and cleanup stopped it, leaving the installed app unusable alone -> prove cold launch from /Applications without any external server.
implementation: Native host resolves only Contents/Resources/gimbal, starts it with --instance-dir ~/.gimbal/desktop --no-project --port 0 --ready, waits for bound-loopback READY, and stops only that recorded child on app termination. Explicit GIMBAL_SERVER_URL bypasses child management. Last-window close keeps app/server alive so active work is not interrupted; Command-Q stops the child. Release packaging builds and signs the exact-version CLI into app Resources with the tested library-validation entitlement.
friction: A double-quoted rg pattern contained Markdown backticks; zsh expanded them and launched the local Gimbal CLI. Terminated that process and removed its generated worktree .gimbal state. Use single-quoted regex patterns for shell searches.
