#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec swift test --package-path native/Transcription
