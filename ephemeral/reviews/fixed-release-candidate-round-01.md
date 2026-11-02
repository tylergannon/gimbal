# Fixed release candidate assembly review

Target: uncommitted working tree on `codex/fixed-release-candidate`, based on `a3ee508b8e7a1582971ce03170e5695fadedf118`. Reviewed the workflow, release-assets command dispatch and new assembly/verification implementation and tests, distribution reference, and authoritative scope in `ephemeral/worklog/fixed-release-candidate.md`.

Evidence inspected: the complete relevant diff; existing shared-input packing/restoration and CLI/macOS packaging; appcast production; release publication and Homebrew checksum consumption; tagged automatic dispatch and its tag-only macOS job; local published-file demonstration under `/tmp/gimbal-fixed-candidate-demo`. The latter contains the three published CLI archives, unchanged ZIP/DMG/feed, assembled metadata and rejection output. Independently verified the existing fixed directory using its compiled CLI and exact repository/source/tag/run/attempt identity, and compared the resulting stdout with the original assembled manifest.

Independently completed focused checks: `go test ./internal/automation/release-assets -count=1` (passed, 1.893s), `go vet ./internal/automation/release-assets` (passed), `golangci-lint run ./internal/automation/release-assets` (0 issues), and `actionlint .github/workflows/release.yml` (passed).

The implementation assembles publication bytes once, keeps signed native packages/feed unchanged, generates the existing five-archive checksum format, and binds the final seven publication files to source and producing-run identity in the manifest. Verification rejects missing/extra/nonregular files, unsafe or duplicate manifest file names, identity and content mismatches, misleading rehashed checksums, and the specified feed inconsistencies. Upload-assigned immutable artifact identity is exposed outside its own bundle. The publisher consumes the explicit artifact ID, verifies rather than reconstructing publication content, and uses the saved assembly attempt for retries. Current publication default, source/platform gates, native packaging and two-day retention policy remain intact; candidate metadata contains no acceptance verdict.

Findings: none.

Limits: this is tooling and workflow inspection with local fixed-file demonstrations. No Actions artifact upload/download, workflow retry, tagged/native packaging, GUI/model-provider session, publication, service lifecycle or credentials action was performed. Those operational paths are not established by the focused checks, and the current draft does not claim product acceptance or durable storage beyond the existing artifact retention period.

Outcome: no findings.
