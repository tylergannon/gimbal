#!/bin/bash
# Deterministic recorder checks only: no microphone permissions or capture.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"
go test -race ./internal/transcript
go test -race -tags synthetic ./internal/transcript
swift test --package-path native/Transcription
package_bin=$(swift build --package-path native/Transcription --show-bin-path)
out="$repo_root/native/build/check-recorder"
mkdir -p "$out"
target="$(uname -m)-apple-macos27.0"
swiftc -target "$target" -I "$package_bin" -I "$package_bin/Modules" -L "$package_bin" -lTranscriptSource -parse-as-library native/Sources/TranscriptCore.swift native/Tests/CoreTests.swift -o "$out/core-tests"
"$out/core-tests"
swiftc -target "$target" -I "$package_bin" -I "$package_bin/Modules" -L "$package_bin" -lTranscriptSource -parse-as-library native/Sources/TranscriptCore.swift native/Sources/RecorderController.swift native/Tests/ControllerTests.swift -o "$out/controller-tests"
"$out/controller-tests"
swiftc -target "$target" -I "$package_bin" -I "$package_bin/Modules" -L "$package_bin" -lAppleTranscription -lTranscriptSource -parse-as-library native/Sources/RecordingInput.swift native/Tests/RecordingInputTests.swift -o "$out/input-tests"
"$out/input-tests"
swiftc -target "$target" -I "$package_bin" -I "$package_bin/Modules" -L "$package_bin" -lScriptedTranscription -lTranscriptSource -parse-as-library native/Sources/TranscriptCore.swift native/Sources/RecorderController.swift native/Tests/ScriptedControllerTests.swift -o "$out/scripted-controller-tests"
"$out/scripted-controller-tests"
