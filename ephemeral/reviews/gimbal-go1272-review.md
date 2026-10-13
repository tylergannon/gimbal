# Independent review: Go 1.27.2 release pin

Review date: 2026-10-12. Reviewed commits `a429f60` and `ef2f238` on
`codex/gimbal-release-go1272`, relative to merged release `5c95a3c`.

## Result

No blocking findings. The change addresses the release regression at the
toolchain boundary without changing Gimbal runtime implementation: `go.mod`
now requires Go 1.27.2, and both Linux and macOS release jobs select Go from
that file. The Go 1.27.1 failure is consistent with upstream
[golang/go#81890](https://github.com/golang/go/issues/81890), which records
the macOS 27 `plugin.Open` linker defect, its 1.27.2 milestone, and the
backport that disables chained fixups when building plugins.

The exact patch pin is effective in `actions/setup-go`'s `go-version-file`
mode: its documentation says a patch component in `go.mod` selects that
specific Go patch release. The Mac-only `cache: false` is scoped to the
persistent runner, preserving the existing host-local Go caches while avoiding
GitHub cache activity. The Homebrew Cask now expresses the actual macOS 27
minimum for both native CLI and app artifacts. Changelog comparison links
advance from immutable `v0.13.0` to `v0.13.1`, leaving Unreleased above it.

One non-blocking wording nit: the changelog says “Skip redundant GitHub cache
uploads,” while `cache: false` disables both restore and save. “Disable GitHub
Go cache on the persistent Mac release runner” would describe the setting more
precisely. This does not affect release behavior.

No source implementation, test, or proof-output changes were made as part of
this review.
