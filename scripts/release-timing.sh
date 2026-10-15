#!/usr/bin/env bash
set -euo pipefail

# Measure this workflow only, including its initial queue. Never block delivery.
if ! started=$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" --jq '.created_at | fromdateiso8601'); then
	printf '::warning::Could not read this release workflow start time.\n'
	exit 0
fi
if ! [[ $started =~ ^[0-9]+$ ]]; then
	printf '::warning::Release workflow start time is unavailable.\n'
	exit 0
fi
elapsed=$(( $(date +%s) - started ))
if ((elapsed < 0)); then
	printf '::warning::Release workflow start time is in the future; timing unavailable.\n'
	exit 0
fi
printf 'Release workflow elapsed: %ss (goal 300s, maximum 600s; includes queueing).\n' "$elapsed"
if ((elapsed > 600)); then
	printf '::warning::Release took %ss, exceeding the 600s maximum. Fix the slow stage before the next release.\n' "$elapsed"
elif ((elapsed > 300)); then
	printf '::warning::Release took %ss, exceeding the 300s goal. Inspect this run for the slow stage.\n' "$elapsed"
fi
if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
	if ! printf 'Release workflow elapsed: **%ss**. Goal: 300s; maximum: 600s. Includes queueing.\n' "$elapsed" >> "$GITHUB_STEP_SUMMARY"; then
		printf '::warning::Could not write the release timing summary.\n'
	fi
fi
