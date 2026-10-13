# Promises ledger worklog

- 2026-10-13: plan consensus after 9 review rounds (round 09: no findings).
- Shared base fb6a531b: Requirements flag, MarkRequirements, markRequirements remote, codegen.
- Parallel tracks (Opus agents): pl-core (internal/promises + claude.TextOnly + live eval),
  pl-web (host wiring, remotes, UI; stub promises API commit to drop on merge),
  pl-native (Swift entry point), design pages + reviewers (Designer "Gimbal workspace"),
  fixtures recon (~/src/promises-research/fixtures), demo driver (~/src/promises-research/demo).
- Merged pl-native, pl-core, pl-web (stub dropped) into promises-ledger; updates now keep tags when none are given.
- Web review fixes: no live-query reconnect after commands; unused RecordingHistory.svelte deleted.
- Proof, 2026-10-13, GIMBAL_PROMISES_MODEL=haiku (claude-haiku-4-5), scratch project /tmp/promises-demo-project,
  driven through the same HTTP remotes the native app uses (~/src/promises-research/demo/speak.go):
  - 01-notes-app (requirements at start): 3 promises added with tags; "withdraw the second one" withdrew #2.
  - 02-sync-service (marked after stopping): 3 more added; the restated p95 promise stayed one entry; the
    in-batch "withdraw the second one" (99.9% availability) was never added.
  - Restart: ledger.json checksum unchanged after a minute of running; nothing reprocessed.
  - Screens: ~/src/promises-research/screens/demo-promises-tab.png, demo-recording-marked.png.
- Not proven: clicking Recording ▸ Record Requirements Discussion in the native app with the real microphone.
