#!/usr/bin/env bash
set -euo pipefail

# Use a stable Developer ID requirement for local microphone validation, matching
# release builds. Linker ad-hoc signatures identify one code hash, so rebuilding
# otherwise discards the identity macOS granted privacy permissions to.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:?usage: scripts/sign-native-dev.sh PATH_TO_APP}"
IDENTITY="${MACOS_SIGNING_IDENTITY:-}"

if [[ -z "$IDENTITY" ]]; then
  identities=()
  while IFS= read -r identity; do
    [[ -n "$identity" ]] && identities+=("$identity")
  done < <(security find-identity -v -p codesigning | awk -F '"' '/Developer ID Application:/{print $2}')
  if [[ ${#identities[@]} -ne 1 ]]; then
    printf 'Set MACOS_SIGNING_IDENTITY to an existing Developer ID Application identity (found %s).\n' "${#identities[@]}" >&2
    exit 1
  fi
  IDENTITY="${identities[0]}"
fi
[[ "$IDENTITY" == "Developer ID Application:"* ]] || {
  printf 'MACOS_SIGNING_IDENTITY must be a Developer ID Application identity.\n' >&2
  exit 1
}

MAIN_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Contents/Info.plist")"
MAIN="$APP/Contents/MacOS/$MAIN_NAME"
sign_code() {
  local target="$1"
  local args=(--force --options runtime --timestamp=none --sign "$IDENTITY")
  if [[ "$target" == "$APP/Contents/Resources/gimbal" ]]; then
    args+=(--entitlements "$ROOT/scripts/macos-cli.entitlements.plist")
  fi
  codesign "${args[@]}" "$target"
}

while IFS= read -r -d '' file; do
  [[ "$file" == "$MAIN" ]] && continue
  if /usr/bin/file -b "$file" | grep -q 'Mach-O'; then
    sign_code "$file"
  fi
done < <(find "$APP/Contents" -type f -print0)

while IFS= read -r -d '' bundle; do
  [[ "$bundle" == "$APP" ]] && continue
  sign_code "$bundle"
done < <(find "$APP/Contents" -depth -type d \( -name '*.framework' -o -name '*.bundle' -o -name '*.xpc' -o -name '*.appex' -o -name '*.app' \) -print0)

codesign --force --options runtime --timestamp=none --entitlements "$ROOT/native/release.entitlements.plist" --sign "$IDENTITY" "$MAIN"
codesign --force --options runtime --timestamp=none --entitlements "$ROOT/native/release.entitlements.plist" --sign "$IDENTITY" "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
printf 'Signed local app with %s; bundle name and identifier retained.\n' "$IDENTITY"
