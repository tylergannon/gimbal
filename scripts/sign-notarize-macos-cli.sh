#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/sign-notarize-macos-cli.sh <goreleaser-tar.gz>" >&2
  exit 2
fi
: "${MACOS_SIGNING_IDENTITY:?CI signing setup must export MACOS_SIGNING_IDENTITY}"
: "${MACOS_NOTARY_PROFILE:?CI signing setup must export MACOS_NOTARY_PROFILE}"

archive="$1"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir "$work/unpacked"
tar -xzf "$archive" -C "$work/unpacked"
binary="$work/unpacked/gimbal"
[[ -f "$binary" ]] || { echo "archive does not contain gimbal executable" >&2; exit 1; }

# Go plugins are separately linked, ad-hoc-signed binaries. This entitlement is
# required on the macOS CLI host for supported Gimbal plugin loading; it is not
# used by Gimbal.app or applied to plugin binaries.
codesign --force --timestamp --options runtime \
  --entitlements scripts/macos-cli.entitlements.plist \
  --sign "$MACOS_SIGNING_IDENTITY" "$binary"
codesign --verify --strict --verbose=2 "$binary"

# Apple notarizes a ZIP containing the signed CLI. The downloadable tarball
# itself cannot be stapled, so users need network access for first Gatekeeper
# assessment; the code signature carries the notarization ticket lookup.
ditto -c -k --sequesterRsrc --keepParent "$work/unpacked" "$work/gimbal-cli.zip"
notary_keychain_args=()
if [[ -n "${GIMBAL_CI_KEYCHAIN_PATH:-}" ]]; then
  notary_keychain_args+=(--keychain "$GIMBAL_CI_KEYCHAIN_PATH")
fi
response="$(xcrun notarytool submit "$work/gimbal-cli.zip" \
  --keychain-profile "$MACOS_NOTARY_PROFILE" "${notary_keychain_args[@]}" --wait --output-format json)"
status="$(printf '%s' "$response" | jq -r '.status // empty')"
if [[ "$status" != Accepted ]]; then
  echo "macOS CLI notarization was not accepted (status: ${status:-missing})" >&2
  id="$(printf '%s' "$response" | jq -r '.id // empty')"
  if [[ -n "$id" ]]; then
    xcrun notarytool log "$id" --keychain-profile "$MACOS_NOTARY_PROFILE" "${notary_keychain_args[@]}" || true
  fi
  exit 1
fi

tar -czf "$work/signed.tar.gz" -C "$work/unpacked" .
mv "$work/signed.tar.gz" "$archive"
echo "Signed and notarized macOS CLI archive: $archive"
