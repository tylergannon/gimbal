#!/usr/bin/env bash
set -euo pipefail

max_file_bytes=${GIMBAL_MAX_STAGED_FILE_BYTES:-1048576}
max_total_bytes=${GIMBAL_MAX_STAGED_TOTAL_BYTES:-5242880}
max_paths=${GIMBAL_MAX_STAGED_PATHS:-100}
max_added_lines=${GIMBAL_MAX_STAGED_ADDED_LINES:-10000}

staged_paths=()
while IFS= read -r -d '' path; do
	staged_paths+=("$path")
done < <(git diff --cached --name-only --diff-filter=ACMR -z)

if ((${#staged_paths[@]} == 0)); then
	exit 0
fi

run_log_paths=()
ephemeral_code_paths=()
total_bytes=0
largest_bytes=0
largest_path=

for path in "${staged_paths[@]}"; do
	if [[ $path =~ (^|/)\.gimbal/ ]] ||
		[[ $path =~ ^ephemeral/attest/ ]] ||
		[[ $path =~ ^ephemeral/(.*/)?(runs?|logs?|events|stages|sessions|commands|results?|screenshots?)/ ]] ||
		[[ $path =~ ^ephemeral/.*/([^/]*-logs|logs-[^/]*|run-logs[^/]*)/ ]] ||
		[[ $path =~ ^ephemeral/.*/(results?|proof|stdout|stderr|[^/]*-(stdout|stderr)|records)\.(md|txt|json)$ ]] ||
		[[ $path =~ ^ephemeral/.*\.(png|jpe?g|webm|html)$ ]] ||
		{ [[ $path =~ \.(jsonl|log)$ ]] && ! [[ $path =~ (^|/)testdata/ ]]; } ||
		{ [[ $path =~ (^|/)(run|scopes|sessions|turns|turn_usage|model_calls|commands)\.json$ ]] && ! [[ $path =~ (^|/)testdata/ ]]; } ||
		[[ $path =~ (^|/)checkpoint\.json$ ]]; then
		run_log_paths+=("$path")
	elif [[ $path =~ ^ephemeral/ ]] && ! [[ $path =~ ^ephemeral/legacy/ ]] &&
		[[ $path =~ \.(go|sh|py|ts|js|mjs|svelte)$ ]]; then
		ephemeral_code_paths+=("$path")
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
	printf 'ERROR: nothing a run produced is ever committed: no logs, no result files,\n' >&2
	printf 'no screenshots, no text dumps, nothing under ephemeral/attest/.\n' >&2
	printf 'Proof is saying what you saw, in the chat or the PR description. Unstage these:\n' >&2
	printf '  %s\n' "${run_log_paths[@]}" >&2
	exit 1
fi

if ((${#ephemeral_code_paths[@]} > 0)); then
	printf 'ERROR: code under ephemeral/ is never checked or maintained, so it is never committed.\n' >&2
	printf 'There are no proof programs. A repeatable check is a test beside the code it checks.\n' >&2
	printf 'Unstage these:\n' >&2
	printf '  %s\n' "${ephemeral_code_paths[@]}" >&2
	exit 1
fi

if [[ ${GIMBAL_ALLOW_LARGE_COMMIT:-} == 1 ]]; then
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
	printf 'ERROR: staged change is too large for an ordinary Gimbal commit.\n' >&2
	printf 'This usually means generated output, run artifacts, or copied third-party code was staged.\n' >&2
	printf 'Write and commit a summary; use a pinned submodule for substantial external source.\n' >&2
	printf 'Limits exceeded:\n' >&2
	printf '  %s\n' "${violations[@]}" >&2
	printf 'If Tyler explicitly approved this exact large commit, retry with GIMBAL_ALLOW_LARGE_COMMIT=1.\n' >&2
	exit 1
fi
