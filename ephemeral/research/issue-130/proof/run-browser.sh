#!/bin/sh
set -eu
cd "$(dirname "$0")"
pnpm --ignore-workspace install --frozen-lockfile
if [ -z "${GIMBLE_PLAYWRIGHT_EXECUTABLE:-}" ]; then
	GIMBLE_PLAYWRIGHT_EXECUTABLE=$(find "$HOME/Library/Caches/ms-playwright" -path '*/chrome-headless-shell' -type f | sort | tail -1)
	export GIMBLE_PLAYWRIGHT_EXECUTABLE
fi
pnpm --ignore-workspace exec playwright test --config playwright.config.ts
