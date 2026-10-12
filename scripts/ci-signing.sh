#!/usr/bin/env bash
set -euo pipefail

mode="${1:-}"
case "$mode" in
  setup|cleanup) ;;
  *) echo "usage: scripts/ci-signing.sh setup|cleanup" >&2; exit 2 ;;
esac

if [[ "$mode" == cleanup ]]; then
  keychain="${GIMBAL_CI_KEYCHAIN_PATH:-}"
  if [[ -n "$keychain" ]]; then
    previous_keychains=()
    while IFS= read -r item; do
      [[ -n "$item" ]] && previous_keychains+=("$item")
    done <<< "${GIMBAL_CI_PREVIOUS_KEYCHAINS:-}"
    if ((${#previous_keychains[@]})); then
      security list-keychains -d user -s "${previous_keychains[@]}" || true
    fi
    if [[ -f "$keychain" ]]; then
      security delete-keychain "$keychain" || true
    fi
  fi
  temp_dir="${GIMBAL_CI_SIGNING_DIR:-}"
  if [[ -n "$temp_dir" && -d "$temp_dir" ]]; then
    rm -rf "$temp_dir"
  fi
  exit 0
fi

[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo "signing setup is restricted to ephemeral GitHub-hosted runners" >&2; exit 2; }

: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_ENV:?GITHUB_ENV is required}"
: "${DEVELOPER_ID_CERTIFICATE_P12_BASE64:?Developer ID certificate secret is required}"
: "${DEVELOPER_ID_CERTIFICATE_PASSWORD:?Developer ID certificate password secret is required}"
: "${APPLE_TEAM_ID:?Apple team identifier secret is required}"
: "${APP_STORE_CONNECT_KEY_ID:?App Store Connect key ID secret is required}"
: "${APP_STORE_CONNECT_ISSUER_ID:?App Store Connect issuer ID secret is required}"
: "${APP_STORE_CONNECT_API_KEY_P8_BASE64:?App Store Connect API key secret is required}"
: "${MACOS_NOTARY_PROFILE:?MACOS_NOTARY_PROFILE is required}"

signing_dir="$(mktemp -d "$RUNNER_TEMP/gimbal-signing.XXXXXX")"
keychain="$signing_dir/release.keychain-db"
keychain_password="$(openssl rand -hex 32)"
previous_keychains_raw="$(security list-keychains -d user | sed -E 's/^[[:space:]]*//; s/^\"//; s/\"$//')"
previous_keychains=()
while IFS= read -r item; do
  [[ -n "$item" ]] && previous_keychains+=("$item")
done <<< "$previous_keychains_raw"
p12_path="$signing_dir/developer-id.p12"
p8_path="$signing_dir/AuthKey_${APP_STORE_CONNECT_KEY_ID}.p8"

cleanup_on_error() {
  security list-keychains -d user -s "${previous_keychains[@]}" >/dev/null 2>&1 || true
  security delete-keychain "$keychain" >/dev/null 2>&1 || true
  rm -rf "$signing_dir"
}
{
  printf 'GIMBAL_CI_KEYCHAIN_PATH=%s\n' "$keychain"
  printf 'GIMBAL_CI_SIGNING_DIR=%s\n' "$signing_dir"
  delimiter="GIMBAL_CI_KEYCHAINS_$$"
  printf 'GIMBAL_CI_PREVIOUS_KEYCHAINS<<%s\n%s\n%s\n' "$delimiter" "$previous_keychains_raw" "$delimiter"
} >> "$GITHUB_ENV"
trap cleanup_on_error EXIT

printf '%s' "$DEVELOPER_ID_CERTIFICATE_P12_BASE64" | base64 -D > "$p12_path"
printf '%s' "$APP_STORE_CONNECT_API_KEY_P8_BASE64" | base64 -D > "$p8_path"
chmod 600 "$p12_path" "$p8_path"

security create-keychain -p "$keychain_password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$keychain_password" "$keychain"
security import "$p12_path" -k "$keychain" -P "$DEVELOPER_ID_CERTIFICATE_PASSWORD" \
  -T /usr/bin/codesign -T /usr/bin/security >/dev/null
security set-key-partition-list -S apple-tool:,apple: -s \
  -k "$keychain_password" "$keychain" >/dev/null
security list-keychains -d user -s "$keychain" "${previous_keychains[@]}"

identity_list="$(security find-identity -v -p codesigning "$keychain")"
identity_count="$(printf '%s\n' "$identity_list" | awk -F '"' '/Developer ID Application:/ {n++} END {print n+0}')"
if [[ "$identity_count" -ne 1 ]]; then
  echo "expected exactly one valid Developer ID Application identity in imported certificate" >&2
  exit 1
fi
identity="$(printf '%s\n' "$identity_list" | awk -F '"' '/Developer ID Application:/ {print $2}')"
if [[ "$identity" != *"($APPLE_TEAM_ID)"* ]]; then
  echo "Developer ID identity team does not match APPLE_TEAM_ID" >&2
  exit 1
fi

xcrun notarytool store-credentials "$MACOS_NOTARY_PROFILE" \
  --key "$p8_path" --key-id "$APP_STORE_CONNECT_KEY_ID" \
  --issuer "$APP_STORE_CONNECT_ISSUER_ID" --keychain "$keychain" >/dev/null
rm -f "$p12_path" "$p8_path"

{
  printf 'MACOS_SIGNING_IDENTITY=%s\n' "$identity"
  printf 'MACOS_NOTARY_PROFILE=%s\n' "$MACOS_NOTARY_PROFILE"
} >> "$GITHUB_ENV"
trap - EXIT
echo "Prepared ephemeral macOS signing keychain and notary profile"
