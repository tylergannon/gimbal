# Issue 130 durable snapshot evidence

The repeatable production-browser scenario is
`ephemeral/research/issue-130/proof/run-browser.sh`. It uses the production
handler and Chrome, opens the SSR page with JavaScript disabled, joins live,
observes concurrent partial text/reasoning/tool updates, reloads mid-run,
finishes, stops the Go process, starts a new process against the same project,
and verifies the final transcript after restart.

The 2026-09-14 run passed. Its measured run had 69,047 bytes of raw JSONL
history, an 8,542-byte public reduced snapshot, and a 12,046-byte SSR response.
The 39 incremental delta payloads ranged from 282 to 2,765 bytes. The final
snapshot covered the journal exactly, so restart read a zero-byte suffix.
Initial SSR navigation was 21.36 ms, the partial concurrent update batch was
visible in 131.86 ms, and restart plus process startup and browser rejoin was
136.57 ms. These are single local samples, not latency benchmarks.

Two real harness attestations also passed through the production browser:

- Codex `gpt-5.6-luna`: run
  `01M2GSX9CJRWA39FQBZ8XC9XP7.observation-proof`, completed, 28 delta frames,
  shell marker and final-answer marker visible.
- Claude `haiku` (`claude-haiku-4-5-20251001`): run
  `01M2GSWZHEQSX7RS4VRB3N5TZW.observation-proof`, completed, 39 delta frames,
  shell marker and final-answer marker visible.

Focused recovery tests force a snapshot cut before later events, restore by
seeking to its saved byte offset, compare suffix-reduced state with uninterrupted
state, reject stale stream identities with a replacement snapshot, and verify
an interrupted final journal line does not advance the recovered cursor.
