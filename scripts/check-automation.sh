#!/usr/bin/env bash
set -euo pipefail

# Nested skill modules are installed independently and are outside root ./....
# Make any residual Python invocation fail during these executable checks.
blocked="$(mktemp -d "${TMPDIR:-/tmp}/gimbal-no-python.XXXXXX")"
trap 'rm -f "$blocked/python" "$blocked/python3"; rmdir "$blocked"' EXIT
for name in python python3; do
  printf '#!/bin/sh\nprintf "Python is not a Gimbal automation prerequisite.\\n" >&2\nexit 99\n' > "$blocked/$name"
  chmod +x "$blocked/$name"
done
export PATH="$blocked:$PATH"
export GOWORK=off

while IFS= read -r -d '' source; do
  case "$source" in
    ephemeral/*) continue ;;
    *.py) if [[ -e "$source" ]]; then printf 'Maintained Python helper remains: %s\n' "$source" >&2; exit 1; fi ;;
  esac
done < <(git ls-files -z '*.py')

# Catch embedded interpreter commands as well as standalone helper files.
if matches=$(git grep -n -i -E 'python[0-9.]*([[:space:]]+-[cm]|",[[:space:]]*"-[cm])' -- '*.go' '*.sh' '*.mts' '*.ts' '*.mjs' ':!ephemeral/**' ':!third_party/**' ':!scripts/check-automation.sh'); then
  printf 'Maintained inline Python command remains:\n%s\n' "$matches" >&2
  exit 1
else
  status=$?
  if ((status != 1)); then exit "$status"; fi
fi

go test -count=1 ./internal/automation/...
go vet ./internal/automation/...
golangci-lint run ./internal/automation/...
node --test scripts/*.test.mts
for module in .agents/skills/*/scripts/*/go.mod; do
  [[ -f "$module" ]] || continue
  directory=${module%/go.mod}
  go -C "$directory" test -count=1 ./...
  go -C "$directory" vet ./...
  (cd "$directory" && golangci-lint run ./...)
done
