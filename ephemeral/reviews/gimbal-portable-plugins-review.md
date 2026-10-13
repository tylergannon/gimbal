# Gimbal portable plugin release review

## Result

No release-blocking defect found in the reviewed patch.

The GoReleaser build adds `-trimpath`, matching the cross-checkout host/plugin
probe. The release gate extracts the native archive, checks out the exact
release tag in a separate source directory, builds the external consumer
plugin in a Go workspace with that checkout, and verifies the archived CLI
starts the filesystem messaging worker and stops on interrupt. The documented
recipe explains both required identities: matching release build settings and
the tagged Gimbal source as a workspace main module. The shell gate runs before
checksums and artifact upload for each native target, after macOS signing and
notarization where applicable.

Review checks: `git diff --check` passed. The archive integration test was not
rerun in this review because the current `dist/` tree has no v0.13.2 candidate.
The new archive regression test passed locally with the signed/notarized
macOS trimpath candidate and failed as expected against the public non-trimpath
v0.13.1 CLI. Its Linux execution remains unverified; CI for v0.13.2 will be
the first Linux run of this new gate. Earlier Linux CLI runtime checks do not
cover this archive integration test. No source files were changed.
