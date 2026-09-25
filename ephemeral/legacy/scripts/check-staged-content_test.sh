#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
guard=$repo_root/scripts/check-staged-content.sh
test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT

new_repo() {
  local name=$1
  local path=$test_root/$name
  mkdir -p "$path"
  git -C "$path" init -q
  git -C "$path" config user.name test
  git -C "$path" config user.email test@example.com
  printf '%s\n' "$path"
}

expect_blocked() {
  local repo=$1
  local expected=$2
  if output=$(cd "$repo" && "$guard" 2>&1); then
    printf 'expected guard to reject staged content in %s\n' "$repo" >&2
    exit 1
  fi
  if [[ $output != *"$expected"* ]]; then
    printf 'guard output did not contain %q:\n%s\n' "$expected" "$output" >&2
    exit 1
  fi
}

small_repo=$(new_repo small)
printf 'short authored summary\n' >"$small_repo/summary.md"
mkdir -p "$small_repo/.gimbal"
printf 'exec go run ./cmd/server\n' >"$small_repo/.gimbal/run"
git -C "$small_repo" add summary.md
git -C "$small_repo" add .gimbal/run
(cd "$small_repo" && "$guard")

run_repo=$(new_repo run-log)
mkdir -p "$run_repo/.gimbal/runs/abc"
printf '{}\n' >"$run_repo/.gimbal/runs/abc/timeline.jsonl"
git -C "$run_repo" add -f .gimbal/runs/abc/timeline.jsonl
expect_blocked "$run_repo" 'run logs must not be committed'
if (cd "$run_repo" && GIMBAL_ALLOW_LARGE_COMMIT=1 "$guard" >/dev/null 2>&1); then
  printf 'large-commit override must not permit run logs\n' >&2
  exit 1
fi

large_repo=$(new_repo large-file)
dd if=/dev/zero of="$large_repo/copied-source.bin" bs=1048577 count=1 2>/dev/null
git -C "$large_repo" add copied-source.bin
expect_blocked "$large_repo" 'per-file limit'
(cd "$large_repo" && GIMBAL_ALLOW_LARGE_COMMIT=1 "$guard")

lines_repo=$(new_repo added-lines)
mkdir -p "$lines_repo/third_party"
awk 'BEGIN { for (i = 0; i < 10001; i++) print "copied line" }' >"$lines_repo/third_party/corpus.txt"
git -C "$lines_repo" add third_party/corpus.txt
expect_blocked "$lines_repo" 'added lines exceeds'

paths_repo=$(new_repo path-count)
mkdir -p "$paths_repo/generated"
for index in $(seq 1 101); do
  printf 'generated\n' >"$paths_repo/generated/$index.txt"
done
git -C "$paths_repo" add generated
expect_blocked "$paths_repo" 'staged paths exceeds'

total_repo=$(new_repo total-size)
mkdir -p "$total_repo/copied"
for index in $(seq 1 6); do
  dd if=/dev/zero of="$total_repo/copied/$index.bin" bs=900000 count=1 2>/dev/null
done
git -C "$total_repo" add copied
expect_blocked "$total_repo" 'staged bytes exceeds'

printf 'staged-content policy tests passed\n'
