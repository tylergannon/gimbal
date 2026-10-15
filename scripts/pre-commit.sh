#!/usr/bin/env bash
set -euo pipefail

# Source commits use directly executable TypeScript; docs need only Bash and Git.
use_node=0
report_path=
args=("$@")
while (($#)); do
	case "$1" in
	--warm | --benchmark) use_node=1 ;;
	--report)
		shift
		if (($# == 0)); then printf 'ERROR: --report requires a path\n' >&2; exit 1; fi
		report_path=$1
		;;
	--help) printf 'Usage: bash scripts/pre-commit.sh [--warm] [--benchmark] [--report PATH]\n'; exit 0 ;;
	*) printf 'ERROR: unknown option: %s\n' "$1" >&2; exit 1 ;;
	esac
	shift
done
while IFS= read -r -d '' path; do
	case "$path" in
	*.go | *.sh | *.js | *.mjs | *.mts | *.ts | *.svelte | *.swift | *.yml | *.yaml | *.json | *.toml | *.sql | *.css | go.mod | go.sum) use_node=1 ;;
	esac
done < <(git diff --cached --name-only -z)
if ((use_node)); then
	if ! version=$(node --version 2>/dev/null); then
		printf 'ERROR: commit smoke checks require Node >=24.13.0.\n' >&2
		exit 1
	fi
	version=${version#v}
	major=${version%%.*}
	minor=${version#*.}
	minor=${minor%%.*}
	if ! [[ $major =~ ^[0-9]+$ && $minor =~ ^[0-9]+$ ]] || ((major < 24 || (major == 24 && minor < 13))); then
		printf 'ERROR: commit smoke checks require Node >=24.13.0; found %s.\n' "$version" >&2
		exit 1
	fi
	# Bash 3.2 treats an empty array as unset under nounset.
	exec node scripts/pre-commit.mts ${args[@]+"${args[@]}"}
fi

# No runtime bootstrap for docs, including mandatory failures and advisory timing.
timing=$(mktemp)
errors=$(mktemp)
trap 'rm -f "$timing" "$errors"' EXIT
TIMEFORMAT=%R
status=0
{ time {
	bash scripts/check-staged-content.sh &&
		git diff --cached --check &&
		bash scripts/check-staged-format.sh
} 2>"$errors"; } 2>"$timing" || status=$?
cat "$errors" >&2
elapsed=$(cat "$timing")
exceeded=false
if [[ ${elapsed%%.*} -ge 3 ]]; then
	exceeded=true
	printf 'WARNING: commit hook took way too long please fix me (three-second budget). Investigate slow index checks before the next commit. This commit may proceed.\n' >&2
fi
if ((status == 0)); then
	printf 'Documentation-only commit: staged content, whitespace and formatting checked.\n'
	printf 'Commit checks completed in %ss (three-second feedback budget).\n' "$elapsed"
fi
if [[ -z $report_path ]]; then report_path=$(git rev-parse --git-path gimbal-hook-last.json); fi
revision=$(git rev-parse HEAD 2>/dev/null || true)
checks='[]'
if ((status == 0)); then checks='["staged-content-policy", "staged-whitespace", "staged-go-format"]'; fi
if ! { mkdir -p "$(dirname "$report_path")" && printf '{"elapsed_seconds": %s, "budget_exceeded": %s, "incomplete_checks": [], "sha": "%s", "checks": %s}\n' "$elapsed" "$exceeded" "$revision" "$checks" >"$report_path"; }; then
	printf 'WARNING: could not record advisory hook timing.\n' >&2
fi
exit "$status"
