#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Inventory and invalid usage stay ordinary CLI operations. Capture launches its
# own app through LaunchServices so TCC attributes speech/microphone permission
# to the probe, rather than the terminal or agent app that invoked this script.
if [[ $# -ne 4 || "$1" != --input || "$3" != --duration || ! "$4" =~ ^[+]?[0-9]+$ ]]; then
  exec swift run --package-path native/Transcription transcription-probe "$@"
fi
duration=${4#+}
while [[ ${#duration} -gt 1 && "$duration" == 0* ]]; do duration=${duration#0}; done
if (( ${#duration} > 3 )); then
  exec swift run --package-path native/Transcription transcription-probe "$@"
fi
duration=$((10#$duration))
if (( duration < 1 || duration > 120 )); then
  exec swift run --package-path native/Transcription transcription-probe "$@"
fi

swift build --package-path native/Transcription --product transcription-probe
bin_dir=$(swift build --package-path native/Transcription --show-bin-path)
probe_dir="$PWD/native/Transcription/.build/probe"
probe_name=$(/usr/libexec/PlistBuddy -c "Print :CFBundleName" native/Transcription/ProbeInfo.plist)
bundle="$probe_dir/$probe_name.app"
mkdir -p "$bundle/Contents/MacOS"
cp "$bin_dir/transcription-probe" "$bundle/Contents/MacOS/transcription-probe"
cp native/Transcription/ProbeInfo.plist "$bundle/Contents/Info.plist"
/usr/bin/codesign --force --sign - "$bundle"

run_dir=$(mktemp -d "$probe_dir/run.XXXXXX")
trap 'rm -f "$run_dir/stdout.log" "$run_dir/stderr.log"; rmdir "$run_dir"' EXIT
touch "$run_dir/stdout.log" "$run_dir/stderr.log"
if ! /usr/bin/open -n -g -W --stdout "$run_dir/stdout.log" --stderr "$run_dir/stderr.log" -a "$bundle" --args "$@"; then
  cat "$run_dir/stdout.log"
  cat "$run_dir/stderr.log" >&2
  printf 'Unable to launch transcription probe.\n' >&2
  exit 1
fi
cat "$run_dir/stdout.log"
cat "$run_dir/stderr.log" >&2
# open reports launch success, not the executable's exit status. Only the probe's
# completion marker confirms that preparation, capture and drain all finished.
if ! /usr/bin/grep -Eq '^Probe ended; emitted [0-9]+ events\.$' "$run_dir/stdout.log"; then
  printf 'Transcription probe did not complete successfully.\n' >&2
  exit 1
fi
