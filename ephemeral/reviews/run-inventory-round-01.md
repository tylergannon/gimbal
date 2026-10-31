# Run inventory repair review

Target: working tree on `codex/run-inventory-tests`, based on release candidate `4b828028d01a2c10d6ab8a0913922e708952746c`. Reviewed the replacement `web/run_inventory_test.go`, publication test changes, workspace component tests, browser journey and fixture changes, and the scope in `ephemeral/worklog/run-inventory-scope.md`.

Evidence inspected: repository instructions and definition of done; current `watchRuns`, `runview.List`, registry discovery and workspace query ownership; the full relevant diff. Independently ran the four replacement/publication tests (`go test ./web -run 'TestHostedRunPublicationBeforeDirectoryIsReadable|TestRunInventory' -count=1`, passed in 14.647s) and the real production-binary browser journey (passed in 8.1s). Inspected screenshots of the populated project home, live updated focused Runs row at phone width, and retained selected workflow graph. Independent browser artifacts are local under `/tmp/gimbal-inventory-review-proof`.

The structured tests retain identities, ordering, pending interaction, cost, duration, empty-list shape, foreign running-to-completed refresh, and real publication/hosted-run/reopening coverage. The browser journey supplies the rendered application behavior formerly inferred from server HTML, including home/runs empty state, live discovery, direct reloads, in-place updates, transcript opening and graph navigation. The per-transition stream watchdog bounds waits without giving the full publication lifecycle one cumulative deadline.

No application code was changed or bent around the test harness. The fixture-driven browser observation establishes this bounded inventory regression repair; it does not establish native installation, model-provider operation, packaged release qualification, or publication. Those are not claimed by the repair scope.

Findings: none.

Outcome: no findings.

## Addendum: consolidated pending/completed browser coverage

Reviewed the net additions after `08e37ec6`: the pending/completed fixture, contiguous foreign completion journal, Needs you/All/Live/Ended assertions, and additive failed-run fixture used by the existing home/reload/update/transcript/graph journey. Inspected `/tmp/gimbal-inventory-review-addition.txt`, the earlier completed-run provenance and independent observations in the Gimbal View worklog, and the owning durable reader in `internal/observation/follow.go`. Resetting fixtures only before the single journey preserves the admitted registry's observed identities; adding the third record avoids deleting history under that server.

Independently ran the consolidated production-browser journey: passed in 11.8s (scenario 10.3s), with artifacts under `/tmp/gimbal-inventory-review-final`. Also ran the e2e TypeScript check, which passed, and inspected the outcome screenshots showing the pending question beside a completed release, then both rows Completed in Ended with the needs-you badge and action control gone. These checks establish the added assertions against the actual production application rather than only the mocked query. The unchanged Go/component work was not repeated for this addition.

The synthetic completion follows the real durable checkpoint/contiguous-journal seam, retains its pending interaction to verify terminal filtering, and checks stable row identity and focus without introducing application behavior or sleeps. No material or nitpick findings were found in the addition. The prior bounded proof limitations remain accurate.

Reaffirmed outcome: no findings.
