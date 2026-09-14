# #199: Run page prints stated cost with float noise (/bin/zsh.021586099999999997)

https://github.com/tylergannon/gimble/issues/199

Seen in the run-store proof (`ephemeral/attest/run-store/browser-log.json`, run 3, 2026-09-14): the header and a session card showed `$0.021586099999999997` and `$0.027581199999999997` while the underlying `turn_usage.json` rows hold `0.0215861` and `0.0059951`. The sum in Go is exact enough; the page prints the float as-is.

Presentation only, per the run-store Definition of Done (a quirk is filed, not fixed in the sprint). `usageText` in `web/src/lib/observation/index.ts` should format the cost (for example, to at most seven decimals, trailing zeros trimmed) rather than interpolate the raw number.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
