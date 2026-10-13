#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'USAGE'
Build or package Gimbal.app for macOS.

Usage:
  scripts/release-macos.sh --build-only [--version VERSION] [--build-number N]
  scripts/release-macos.sh --release [--version VERSION] [--build-number N]
      [--skip-generate] [--skip-build --built-app PATH]

Build-only mode creates an unsigned Release-configuration app at
dist/macos/Gimbal.app. Release mode requires a Developer ID Application
identity and an existing notarytool keychain profile, then emits a signed,
notarized ZIP and DMG. The DMG contains Gimbal.app and an Applications alias.

Release environment:
  MACOS_SIGNING_IDENTITY  Exact Developer ID Application identity name.
                          If omitted, the first valid identity is selected.
  MACOS_NOTARY_PROFILE    Existing notarytool keychain profile name.
  SPARKLE_ED_KEY_FILE     Private EdDSA key file for signing app updates.
                          CI uses SPARKLE_ED_PRIVATE_KEY instead.

Use --skip-build --built-app PATH only when the generated Xcode project and
the exact candidate app have already been built. This packages that app without
running SkGo or Xcode. Use --skip-generate to reuse an existing Xcode project
while still building its Release configuration.
USAGE
}

die() {
	printf 'release-macos: %s\n' "$*" >&2
	exit 1
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE=""
VERSION="${GIMBAL_VERSION:-}"
BUILD_NUMBER="${GIMBAL_BUILD_NUMBER:-${GITHUB_RUN_NUMBER:-1}}"
BUILT_APP=""
SKIP_BUILD=0
SKIP_GENERATE=0
SIGNING_IDENTITY="${MACOS_SIGNING_IDENTITY:-}"
NOTARY_PROFILE="${MACOS_NOTARY_PROFILE:-}"
KEYCHAIN_PATH="${MACOS_KEYCHAIN_PATH:-}"

notarytool() {
	if [[ -n "$KEYCHAIN_PATH" ]]; then
		xcrun notarytool "$@" --keychain "$KEYCHAIN_PATH"
	else
		xcrun notarytool "$@"
	fi
}

print_notary_keychain_arg() {
	if [[ -n "$KEYCHAIN_PATH" ]]; then
		printf ' --keychain %q' "$KEYCHAIN_PATH" >&2
	fi
}

while (($#)); do
	case "$1" in
		--build-only)
			[[ -z "$MODE" ]] || die 'choose only one of --build-only or --release'
			MODE="build-only"
			;;
		--release)
			[[ -z "$MODE" ]] || die 'choose only one of --build-only or --release'
			MODE="release"
			;;
		--version)
			(($# >= 2)) || die '--version needs a value'
			VERSION="$2"
			shift
			;;
		--build-number)
			(($# >= 2)) || die '--build-number needs a value'
			BUILD_NUMBER="$2"
			shift
			;;
		--skip-build)
			SKIP_BUILD=1
			;;
		--skip-generate)
			SKIP_GENERATE=1
			;;
		--built-app)
			(($# >= 2)) || die '--built-app needs a path'
			BUILT_APP="$2"
			shift
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			die "unknown option: $1 (use --help)"
			;;
	esac
	shift
done

[[ -n "$MODE" ]] || { usage >&2; die 'choose --build-only or --release'; }
if [[ -z "$VERSION" ]]; then
	VERSION="$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null || true)"
	if [[ -z "$VERSION" && "$MODE" == "release" && "${GITHUB_REF_TYPE:-}" == tag ]]; then
		VERSION="${GITHUB_REF_NAME:-}"
	fi
	if [[ -z "$VERSION" ]]; then
		[[ "$MODE" == "build-only" ]] || die 'release mode needs --version, GIMBAL_VERSION, or an exact version tag'
		VERSION="0.1.0"
	fi
fi
VERSION="${VERSION#v}"
[[ "$VERSION" =~ ^[0-9]+(\.[0-9]+){1,2}$ ]] || die "version must contain two or three numeric components: $VERSION"
[[ "$BUILD_NUMBER" =~ ^[0-9]+$ ]] || die "build number must be an integer: $BUILD_NUMBER"

if ((SKIP_BUILD)) && [[ -z "$BUILT_APP" ]]; then
	die '--skip-build requires --built-app PATH'
fi
if [[ -n "$BUILT_APP" ]] && ((!SKIP_BUILD)); then
	die '--built-app requires --skip-build'
fi
if [[ -n "$BUILT_APP" ]] && [[ "$MODE" == "build-only" ]]; then
	die '--built-app is for release packaging; use --release'
fi

OUT_DIR="$ROOT/dist/macos"
APP_OUT="$OUT_DIR/Gimbal.app"
PROJECT="$ROOT/native/build/macos/Gimbal.xcodeproj"
DERIVED="$ROOT/native/build/macos-release-derived"
ENTITLEMENTS="$ROOT/native/release.entitlements.plist"
CLI_ENTITLEMENTS="$ROOT/scripts/macos-cli.entitlements.plist"
BUNDLED_CLI="$ROOT/native/Resources/gimbal"
mkdir -p "$OUT_DIR"

require_command() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

[[ "$(uname -s)" == Darwin ]] || die 'macOS release packaging requires macOS and Xcode'
for tool in ditto xcodebuild codesign plutil; do
	require_command "$tool"
done
for tool in go python3; do
	require_command "$tool"
done
require_command file

if [[ "$MODE" == "release" ]]; then
	[[ -n "${SPARKLE_ED_KEY_FILE:-}" || -n "${SPARKLE_ED_PRIVATE_KEY:-}" ]] || die 'set SPARKLE_ED_KEY_FILE or SPARKLE_ED_PRIVATE_KEY to sign app updates'
	require_command security
	require_command xcrun
	require_command spctl
	require_command hdiutil
	[[ -n "$NOTARY_PROFILE" ]] || die 'set MACOS_NOTARY_PROFILE to an existing notarytool keychain profile'
	[[ -f "$ENTITLEMENTS" ]] || die "entitlements file missing: $ENTITLEMENTS"

	if [[ -n "$KEYCHAIN_PATH" ]]; then
		IDENTITIES="$(security find-identity -v -p codesigning "$KEYCHAIN_PATH" 2>&1 || true)"
	else
		IDENTITIES="$(security find-identity -v -p codesigning 2>&1 || true)"
	fi
	if [[ -z "$SIGNING_IDENTITY" ]]; then
		SIGNING_IDENTITY="$(printf '%s\n' "$IDENTITIES" | awk -F '"' '/Developer ID Application:/{print $2; exit}')"
	fi
	[[ -n "$SIGNING_IDENTITY" ]] || die 'no valid Developer ID Application identity found in the current keychain'
	if ! printf '%s\n' "$IDENTITIES" | grep -Fq "\"$SIGNING_IDENTITY\"" || [[ "$SIGNING_IDENTITY" != Developer\ ID\ Application:* ]]; then
		die "MACOS_SIGNING_IDENTITY is not a valid Developer ID Application identity: $SIGNING_IDENTITY"
	fi
	if ! notarytool history --keychain-profile "$NOTARY_PROFILE" >/dev/null 2>&1; then
		die "notarytool could not load keychain profile: $NOTARY_PROFILE"
	fi
fi

if ((!SKIP_BUILD)); then
	printf 'Building the exact-version bundled Gimbal CLI for macOS arm64...\n'
	GIT_COMMIT="$(git -C "$ROOT" rev-parse --short HEAD)"
	BUILD_DATE="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
	(
		cd "$ROOT"
		CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath \
			-ldflags "-s -w -X main.version=v$VERSION -X main.commit=$GIT_COMMIT -X main.date=$BUILD_DATE" \
			-o "$BUNDLED_CLI" ./cmd/gimbal
	)
	chmod 755 "$BUNDLED_CLI"
	if ! "$BUNDLED_CLI" --version | grep -Fq "v$VERSION"; then
		die "bundled CLI version does not match $VERSION"
	fi
fi

if ((SKIP_BUILD)); then
	[[ -d "$BUILT_APP" ]] || die "built app not found: $BUILT_APP"
	APP_SOURCE="$(cd "$(dirname "$BUILT_APP")" && pwd)/$(basename "$BUILT_APP")"
	if [[ -e "$APP_OUT" && "$APP_SOURCE" -ef "$APP_OUT" ]]; then
		die '--built-app must not be the output app at dist/macos/Gimbal.app'
	fi
	printf 'Using existing app: %s\n' "$APP_SOURCE"
else
	if ((SKIP_GENERATE)); then
		[[ -d "$PROJECT" ]] || die "--skip-generate requires an existing Xcode project: $PROJECT"
	else
		printf 'Generating native app project and unsigned Debug candidate...\n'
		(cd "$ROOT" && PATH="$ROOT/scripts/public-xcodebuild:$PATH" go tool skgo native build --platform macos)
	fi
	[[ -d "$PROJECT" ]] || die "generated Xcode project not found: $PROJECT"
	printf 'Building unsigned Release app with Xcode...\n'
	xcodebuild \
		-packageAuthorizationProvider netrc \
		-scmProvider system \
		-project "$PROJECT" \
		-scheme Gimbal \
		-configuration Release \
		-sdk macosx \
		-derivedDataPath "$DERIVED" \
		CODE_SIGNING_ALLOWED=NO \
		MARKETING_VERSION="$VERSION" \
		CURRENT_PROJECT_VERSION="$BUILD_NUMBER" \
		build
	APP_SOURCE="$DERIVED/Build/Products/Release/Gimbal.app"
	[[ -d "$APP_SOURCE" ]] || die "Xcode did not produce the app: $APP_SOURCE"
fi

rm -rf "$APP_OUT"
ditto "$APP_SOURCE" "$APP_OUT"
INFO="$APP_OUT/Contents/Info.plist"
[[ -f "$INFO" ]] || die "app Info.plist not found: $INFO"
[[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$INFO")" == com.tylergannon.gimbal ]] || \
	die 'distribution app must use bundle identifier com.tylergannon.gimbal; build without a native preset'
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$INFO" 2>/dev/null || \
	/usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string $VERSION" "$INFO"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $BUILD_NUMBER" "$INFO" 2>/dev/null || \
	/usr/libexec/PlistBuddy -c "Add :CFBundleVersion string $BUILD_NUMBER" "$INFO"

if [[ "$MODE" == "build-only" ]]; then
	[[ -x "$APP_OUT/Contents/Resources/gimbal" ]] || die 'built app is missing its bundled Gimbal CLI'
	if ! "$APP_OUT/Contents/Resources/gimbal" --version | grep -Fq "v$VERSION"; then
		die "app's bundled CLI version does not match $VERSION"
	fi
	printf 'Unsigned app: %s\n' "$APP_OUT"
	exit 0
fi

[[ -x "$APP_OUT/Contents/Resources/gimbal" ]] || die 'app is missing its bundled Gimbal CLI'
if ! "$APP_OUT/Contents/Resources/gimbal" --version | grep -Fq "v$VERSION"; then
	die "app's bundled CLI version does not match $VERSION"
fi

MAIN_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$INFO")"
MAIN_EXECUTABLE="$APP_OUT/Contents/MacOS/$MAIN_NAME"
[[ -f "$MAIN_EXECUTABLE" ]] || die "app executable not found: $MAIN_EXECUTABLE"

sign_code() {
	local target="$1"
	local keychain_args=()
	local entitlement_args=()
	if [[ -n "$KEYCHAIN_PATH" ]]; then
		keychain_args+=(--keychain "$KEYCHAIN_PATH")
	fi
	if [[ "$target" == "$APP_OUT/Contents/Resources/gimbal" ]]; then
		entitlement_args+=(--entitlements "$CLI_ENTITLEMENTS")
	fi
	codesign --force --options runtime --timestamp "${keychain_args[@]}" "${entitlement_args[@]}" --sign "$SIGNING_IDENTITY" "$target"
}

printf 'Signing nested Mach-O files and code bundles...\n'
while IFS= read -r -d '' file; do
	[[ "$file" == "$MAIN_EXECUTABLE" ]] && continue
	if /usr/bin/file -b "$file" | grep -q 'Mach-O'; then
		sign_code "$file"
	fi
done < <(find "$APP_OUT/Contents" -type f -print0)

while IFS= read -r -d '' bundle; do
	[[ "$bundle" == "$APP_OUT" ]] && continue
	sign_code "$bundle"
done < <(find "$APP_OUT/Contents" -depth -type d \( -name '*.framework' -o -name '*.bundle' -o -name '*.xpc' -o -name '*.appex' -o -name '*.app' \) -print0)

codesign_keychain_args=()
if [[ -n "$KEYCHAIN_PATH" ]]; then
	codesign_keychain_args+=(--keychain "$KEYCHAIN_PATH")
fi
codesign --force --options runtime --timestamp "${codesign_keychain_args[@]}" --entitlements "$ENTITLEMENTS" --sign "$SIGNING_IDENTITY" "$MAIN_EXECUTABLE"
codesign --force --options runtime --timestamp "${codesign_keychain_args[@]}" --entitlements "$ENTITLEMENTS" --sign "$SIGNING_IDENTITY" "$APP_OUT"
codesign --verify --deep --strict --verbose=2 "$APP_OUT"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gimbal-macos-release.XXXXXX")"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

notarize() {
	local artifact="$1"
	local label="$2"
	local result status submission_id
	if ! result="$(notarytool submit "$artifact" --keychain-profile "$NOTARY_PROFILE" --wait --output-format json 2>&1)"; then
		printf '%s\n' "$result" >&2
		submission_id="$(printf '%s' "$result" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id", ""))' 2>/dev/null || true)"
		if [[ -n "$submission_id" ]]; then
			printf 'Inspect submission with: xcrun notarytool log %s --keychain-profile %s' "$submission_id" "$NOTARY_PROFILE" >&2
			print_notary_keychain_arg
			printf '\n' >&2
		fi
		die "$label notarization command failed"
	fi
	printf '%s\n' "$result"
	status="$(printf '%s' "$result" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status", ""))')"
	if [[ "$status" != Accepted ]]; then
		submission_id="$(printf '%s' "$result" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id", ""))')"
		if [[ -n "$submission_id" ]]; then
			printf 'Inspect submission with: xcrun notarytool log %s --keychain-profile %s' "$submission_id" "$NOTARY_PROFILE" >&2
			print_notary_keychain_arg
			printf '\n' >&2
		fi
		die "$label notarization was not accepted (status: ${status:-unknown})"
	fi
}

PRE_NOTARY_ZIP="$TMP_DIR/Gimbal-pre-notary.zip"
ditto -c -k --keepParent "$APP_OUT" "$PRE_NOTARY_ZIP"
printf 'Submitting app archive for notarization...\n'
notarize "$PRE_NOTARY_ZIP" 'app archive'
xcrun stapler staple "$APP_OUT"
xcrun stapler validate "$APP_OUT"
codesign --verify --deep --strict --verbose=2 "$APP_OUT"
spctl --assess --type execute --verbose=4 "$APP_OUT"

ZIP_OUT="$OUT_DIR/Gimbal-${VERSION}-macos-arm64.zip"
DMG_OUT="$OUT_DIR/Gimbal-${VERSION}-macos-arm64.dmg"
ditto -c -k --keepParent "$APP_OUT" "$ZIP_OUT"

DMG_STAGE="$TMP_DIR/dmg"
mkdir -p "$DMG_STAGE"
ditto "$APP_OUT" "$DMG_STAGE/Gimbal.app"
ln -s /Applications "$DMG_STAGE/Applications"
hdiutil create -volname Gimbal -srcfolder "$DMG_STAGE" -ov -format UDZO "$DMG_OUT"
codesign --force --timestamp "${codesign_keychain_args[@]}" --sign "$SIGNING_IDENTITY" "$DMG_OUT"
printf 'Submitting installable disk image for notarization...\n'
notarize "$DMG_OUT" 'disk image'
xcrun stapler staple "$DMG_OUT"
xcrun stapler validate "$DMG_OUT"

"$ROOT/scripts/generate-appcast.sh" "$VERSION" "$ZIP_OUT"

printf 'Notarized app: %s\n' "$APP_OUT"
printf 'Release ZIP: %s\n' "$ZIP_OUT"
printf 'Release DMG: %s\n' "$DMG_OUT"
