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
- Native app proof, 2026-10-13: the Debug Gimbal.app (default preset, proxied to the demo server) recorded
  03-billing-portal through RecorderController.record(requirements: true) with a scripted source
  (VN_TRANSCRIPT_FIXTURE). A throwaway, uncommitted launch hook opened /projects/<id>, which made the real
  web bridge post selectProject; the hook then called record and stopped after 80 s. Result:
  - Session ECC98E55 stored with requirements:true, 16 sequences, stopped.
  - Promises #7–#10 added: invoice list p95, never charged twice, invoice PDF, webhook delivery. The
    invoice-list restatement merged into #7 as a second piece of evidence, and the double-charge restatement
    did not duplicate.
  - "withdraw the second one, the double charge promise" withdrew #8, not ledger #2.
  - Without a selected project, record fails with "Select a project before recording." (expected).
- Still not proven: the menu click and real microphone/speech recognition. This Mac (Amritanandamayi) is reached
  only over SSH, so there's no GUI control or TCC prompts, and the computer-use tool drives a different Mac
  that runs an old installed Gimbal 0.13.x (the "gim" window). Native controller tests
  (scripts/check-recorder.sh) pass, including record(requirements: true) → open(requirements: true).
