#!/usr/bin/env bash
set -euo pipefail

# Keep commit-msg mandatory and fast, including documentation-only commits.
message=$(git stripspace --strip-comments < "$1")
header=${message%%$'\n'*}
if [[ $header =~ ^[[:alpha:]]+(\([^\(\)]+\))?!: ]] ||
	LC_ALL=C grep -Eq '^BREAKING[ -]CHANGE:' <<< "$message"; then
	printf 'ERROR: Gimbal is still pre-release and has no users. Major version increments are forbidden. Remove the breaking-change marker (! or BREAKING CHANGE) and use feat: or fix: as appropriate.\n' >&2
	exit 1
fi
if ! [[ $header =~ ^[[:alpha:]]+(\([^\(\)]+\))?:[[:space:]][^[:space:]] ]]; then
	printf 'ERROR: Use a Conventional Commit title, for example feat: add a capability or fix: repair a bug.\n' >&2
	exit 1
fi
