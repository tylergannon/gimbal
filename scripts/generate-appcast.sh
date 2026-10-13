#!/usr/bin/env bash
set -euo pipefail

# Run after notarization and stapling: the signature covers the final ZIP bytes.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:?usage: scripts/generate-appcast.sh VERSION ARCHIVE}"
ARCHIVE="${2:?usage: scripts/generate-appcast.sh VERSION ARCHIVE}"
TOOLS="$ROOT/dist/sparkle"
SPARKLE_VERSION=2.10.0
SPARKLE_SHA256=c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c

if [[ -z "${SPARKLE_ED_KEY_FILE:-}" && -z "${SPARKLE_ED_PRIVATE_KEY:-}" ]]; then
  printf 'Set SPARKLE_ED_KEY_FILE or SPARKLE_ED_PRIVATE_KEY to sign updates.\n' >&2
  exit 1
fi
if [[ ! -x "$TOOLS/bin/generate_appcast" ]]; then
  mkdir -p "$TOOLS"
  curl -fL "https://github.com/sparkle-project/Sparkle/releases/download/$SPARKLE_VERSION/Sparkle-$SPARKLE_VERSION.tar.xz" -o "$TOOLS/Sparkle.tar.xz"
  printf '%s  %s\n' "$SPARKLE_SHA256" "$TOOLS/Sparkle.tar.xz" | shasum -a 256 -c -
  tar -xf "$TOOLS/Sparkle.tar.xz" -C "$TOOLS"
fi

STAGE="$(mktemp -d "${TMPDIR:-/tmp}/gimbal-appcast.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
ditto "$ARCHIVE" "$STAGE/$(basename "$ARCHIVE")"
args=(--download-url-prefix "https://github.com/tylergannon/gimbal/releases/download/v$VERSION/" --maximum-deltas 0)
if [[ -n "${SPARKLE_ED_KEY_FILE:-}" ]]; then
  "$TOOLS/bin/generate_appcast" "${args[@]}" --ed-key-file "$SPARKLE_ED_KEY_FILE" "$STAGE"
else
  printf '%s' "$SPARKLE_ED_PRIVATE_KEY" | "$TOOLS/bin/generate_appcast" "${args[@]}" --ed-key-file - "$STAGE"
fi
python3 - "$STAGE/appcast.xml" <<'PY'
import sys
import xml.etree.ElementTree as ET

enclosure = ET.parse(sys.argv[1]).find("channel/item/enclosure")
signature = "{http://www.andymatuschak.org/xml-namespaces/sparkle}edSignature"
if enclosure is None or not enclosure.get(signature):
    sys.exit("Sparkle did not sign the update; check the private key matches SUPublicEDKey.")
PY
mkdir -p "$ROOT/dist/macos"
mv "$STAGE/appcast.xml" "$ROOT/dist/macos/appcast.xml"
printf 'Signed update feed: %s\n' "$ROOT/dist/macos/appcast.xml"
