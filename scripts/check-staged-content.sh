#!/usr/bin/env bash
set -euo pipefail

max_file_bytes=${GIMBLE_MAX_STAGED_FILE_BYTES:-1048576}
max_total_bytes=${GIMBLE_MAX_STAGED_TOTAL_BYTES:-5242880}
max_paths=${GIMBLE_MAX_STAGED_PATHS:-100}
max_added_lines=${GIMBLE_MAX_STAGED_ADDED_LINES:-10000}

staged_paths=()
while IFS= read -r -d '' path; do
	staged_paths+=("$path")
done < <(git diff --cached --name-only --diff-filter=ACMR -z)

if ((${#staged_paths[@]} == 0)); then
	exit 0
fi

run_log_paths=()
total_bytes=0
largest_bytes=0
largest_path=

for path in "${staged_paths[@]}"; do
	if [[ $path =~ (^|/)\.gimble/runs?/ ]] ||
		[[ $path =~ ^ephemeral/(.*/)?(runs?|logs?|events|stages)/ ]] ||
		[[ $path =~ ^ephemeral/.*/([^/]*-logs|logs-[^/]*|run-logs[^/]*)/ ]] ||
		[[ $path =~ (^|/)(timeline|current|steering|worktrees)\.jsonl$ ]] ||
		[[ $path =~ (^|/)checkpoint\.json$ ]] ||
		[[ $path =~ (^|/)mcp-(stdout|stderr)\.log$ ]]; then
		run_log_paths+=("$path")
	fi

	if bytes=$(git cat-file -s ":$path" 2>/dev/null); then
		((total_bytes += bytes)) || true
		if ((bytes > largest_bytes)); then
			largest_bytes=$bytes
			largest_path=$path
		fi
	fi
done

if ((${#run_log_paths[@]} > 0)); then
	printf 'ERROR: Gimble run logs must not be committed.\n' >&2
	printf 'Keep raw runs local and commit a small authored summary instead.\n' >&2
	printf 'Blocked staged paths:\n' >&2
	printf '  %s\n' "${run_log_paths[@]}" >&2
	exit 1
fi

if [[ ${GIMBLE_ALLOW_LARGE_COMMIT:-} == 1 ]]; then
	exit 0
fi

added_lines=0
while IFS=$'\t' read -r added _ _; do
	if [[ $added =~ ^[0-9]+$ ]]; then
		((added_lines += added)) || true
	fi
done < <(git diff --cached --numstat --diff-filter=ACMR)

violations=()
if ((${#staged_paths[@]} > max_paths)); then
	violations+=("${#staged_paths[@]} staged paths exceeds $max_paths")
fi
if ((added_lines > max_added_lines)); then
	violations+=("$added_lines added lines exceeds $max_added_lines")
fi
if ((total_bytes > max_total_bytes)); then
	violations+=("$total_bytes staged bytes exceeds $max_total_bytes")
fi
if ((largest_bytes > max_file_bytes)); then
	violations+=("$largest_path is $largest_bytes bytes; per-file limit is $max_file_bytes")
fi

if ((${#violations[@]} > 0)); then
	printf 'ERROR: staged change is too large for an ordinary Gimble commit.\n' >&2
	printf 'This usually means generated output, run artifacts, or copied third-party code was staged.\n' >&2
	printf 'Write and commit a summary; use a pinned submodule for substantial external source.\n' >&2
	printf 'Limits exceeded:\n' >&2
	printf '  %s\n' "${violations[@]}" >&2
	printf 'If Tyler explicitly approved this exact large commit, retry with GIMBLE_ALLOW_LARGE_COMMIT=1.\n' >&2
	exit 1
fi
