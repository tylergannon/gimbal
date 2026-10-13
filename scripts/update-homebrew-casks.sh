#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: scripts/update-homebrew-casks.sh <tap-checkout> <version> <cli-sha256> <app-dmg-sha256> <tag>" >&2
  exit 2
fi

tap="$1"
version="$2"
cli_sha="$3"
app_sha="$4"
tag="$5"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid semantic version" >&2; exit 2; }
[[ "$cli_sha" =~ ^[0-9a-f]{64}$ ]] || { echo "invalid CLI SHA-256" >&2; exit 2; }
[[ "$app_sha" =~ ^[0-9a-f]{64}$ ]] || { echo "invalid app SHA-256" >&2; exit 2; }
[[ "$tag" == "v$version" ]] || { echo "tag/version mismatch: $tag vs $version" >&2; exit 2; }

mkdir -p "$tap/Casks"
cat > "$tap/Casks/gimbal.rb" <<EOF
cask "gimbal" do
  version "$version"
  sha256 "$cli_sha"

  url "https://github.com/tylergannon/gimbal/releases/download/v#{version}/gimbal_#{version}_darwin_arm64.tar.gz"
  name "Gimbal"
  desc "Go runtime for agent workflows with a live web view"
  homepage "https://github.com/tylergannon/gimbal"

  depends_on arch: :arm64
  depends_on macos: :golden_gate

  binary "gimbal"
end
EOF

cat > "$tap/Casks/gimbal-app.rb" <<EOF
cask "gimbal-app" do
  version "$version"
  sha256 "$app_sha"

  url "https://github.com/tylergannon/gimbal/releases/download/v#{version}/Gimbal-#{version}-macos-arm64.dmg"
  name "Gimbal"
  desc "Native recording application for Gimbal"
  homepage "https://github.com/tylergannon/gimbal"

  depends_on arch: :arm64
  depends_on macos: :golden_gate

  app "Gimbal.app"
end
EOF

git -C "$tap" add Casks/gimbal.rb Casks/gimbal-app.rb
if git -C "$tap" diff --cached --quiet; then
  echo "Homebrew casks already match release $tag"
  exit 0
fi
git -C "$tap" -c user.name="github-actions[bot]" \
  -c user.email="41898282+github-actions[bot]@users.noreply.github.com" \
  commit -m "Update Gimbal casks to $tag"
git -C "$tap" push
