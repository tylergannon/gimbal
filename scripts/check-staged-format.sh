#!/usr/bin/env bash
set -euo pipefail

failed=0
while IFS= read -r -d '' path; do
	case "$path" in
	*.go)
		if unformatted=$(git show ":$path" | gofmt -l); then
			if [[ -n $unformatted ]]; then
				printf 'ERROR: staged Go file needs gofmt: %s\n' "$path" >&2
				failed=1
			fi
		else
			printf 'ERROR: cannot check staged Go file: %s\n' "$path" >&2
			failed=1
		fi
		;;
	esac
done < <(git diff --cached --name-only --diff-filter=ACMR -z)

exit "$failed"
