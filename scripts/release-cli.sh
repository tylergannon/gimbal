#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/release-cli.sh <darwin_arm64|linux_amd64|linux_arm64>" >&2
  exit 2
fi

target="$1"
case "$target" in
  darwin_arm64|linux_amd64|linux_arm64) ;;
  *) echo "unsupported release target: $target" >&2; exit 2 ;;
esac

tag="${RELEASE_TAG:-${GITHUB_REF_NAME:-$(git describe --tags --exact-match HEAD)}}"
version="${tag#v}"
head_tag="$(git describe --tags --exact-match HEAD)"
if [[ "$head_tag" != "$tag" ]]; then
  echo "release tag $tag does not point at checked-out HEAD tag $head_tag" >&2
  exit 2
fi
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release must run from a semantic-version tag; got $tag" >&2
  exit 2
fi

os="${target%_*}"
arch="${target#*_}"
archive="dist/gimbal_${version}_${os}_${arch}.tar.gz"
if [[ "$target" == darwin_arm64 ]]; then
  : "${MACOS_SIGNING_IDENTITY:?CI signing setup must export MACOS_SIGNING_IDENTITY}"
  : "${MACOS_NOTARY_PROFILE:?CI signing setup must export MACOS_NOTARY_PROFILE}"
fi

config="${RUNNER_TEMP:-/tmp}/gimbal-goreleaser-${target}.yaml"
sed "s/      - darwin_arm64/      - ${target}/" .goreleaser.yaml > "$config"

# CI restores the generated source/frontend from this exact revision. A manual
# tagged build prepares them locally, without compiling an unused generic CLI.
if [[ "${RELEASE_PREPARED_INPUTS:-}" != 1 ]]; then
  just build-assets
fi
test -s web/build/skgo.manifest.json

RELEASE_TARGET="$target" goreleaser release --clean --skip=publish,validate --config "$config"

if [[ ! -f "$archive" ]]; then
  echo "GoReleaser did not create expected archive: $archive" >&2
  exit 1
fi

if [[ "$target" == darwin_arm64 ]]; then
  scripts/sign-notarize-macos-cli.sh "$archive"
  # The app bundles the same versioned CLI instead of linking it again.
  mkdir -p native/Resources
  tar -xzf "$archive" -O ./gimbal > native/Resources/gimbal
  chmod 755 native/Resources/gimbal
fi

GIMBAL_RELEASE_ARCHIVE="$PWD/$archive" go test ./cmd/gimbal -run '^TestReleaseCLIPlugin$' -count=1

if command -v shasum >/dev/null 2>&1; then
  (cd dist && shasum -a 256 "$(basename "$archive")" > checksums.txt)
else
  (cd dist && sha256sum "$(basename "$archive")" > checksums.txt)
fi
echo "Built $archive for $tag"
