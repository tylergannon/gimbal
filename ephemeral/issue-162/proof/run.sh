#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
proof="$repo/ephemeral/issue-162/proof"
results="$proof/results"
mkdir -p "$results"

temporary=$(mktemp -d)
baseline_tree="$temporary/baseline"
changed_tree="$temporary/changed"
cleanup() {
	git -C "$repo" worktree remove --force "$baseline_tree" >/dev/null 2>&1 || true
	git -C "$repo" worktree remove --force "$changed_tree" >/dev/null 2>&1 || true
	rm -rf -- "$temporary"
}
trap cleanup EXIT INT TERM

record() {
	name=$1
	shift
	{
		printf 'cwd: %s\ncommand:' "$repo"
		printf ' %s' "$@"
		printf '\n'
	} >"$results/$name.command.txt"
	set +e
	(
		cd "$repo"
		"$@"
	) >"$results/$name.stdout.txt" 2>"$results/$name.stderr.txt"
	LAST_CODE=$?
	set -e
	printf '%s\n' "$LAST_CODE" >"$results/$name.exit.txt"
}

expect_code() {
	name=$1
	want=$2
	if [ "$LAST_CODE" -ne "$want" ]; then
		printf '%s: exit %s, want %s\n' "$name" "$LAST_CODE" "$want" >&2
		exit 1
	fi
}

git -C "$repo" worktree add --detach "$baseline_tree" d5b507c >/dev/null
git -C "$repo" worktree add --detach "$changed_tree" 218799b >/dev/null
(cd "$baseline_tree" && go build -o "$temporary/baseline-gimble" ./cmd)
(cd "$changed_tree" && go build -o "$temporary/changed-gimble" ./cmd)
baseline_size=$(wc -c <"$temporary/baseline-gimble" | tr -d ' ')
changed_size=$(wc -c <"$temporary/changed-gimble" | tr -d ' ')
delta=$((changed_size - baseline_size))
percent=$(awk -v base="$baseline_size" -v delta="$delta" 'BEGIN { printf "%.2f", delta * 100 / base }')
{
	printf 'toolchain: %s\n' "$(go version)"
	printf 'command: go build -o OUTPUT ./cmd\n'
	printf 'baseline: d5b507c %s bytes\n' "$baseline_size"
	printf 'changed: 218799b %s bytes\n' "$changed_size"
	printf 'increase: %s bytes (%s%%)\n' "$delta" "$percent"
} >"$results/binary-size.txt"

binary="$temporary/gimble"
(cd "$repo" && go build -o "$binary" ./cmd)

# The isolated duplicate probe intentionally runs before repository tests.
git -C "$changed_tree" apply "$proof/duplicate-role.patch"
record duplicate-injected sh -c 'cd "$1" && exec "$2" lint ./internal/workflows/sprint' proof "$changed_tree" "$binary"
expect_code duplicate-injected 3
grep -q 'GIMBLE103-SET-MISUSE/DUPLICATE-KEY' "$results/duplicate-injected.stderr.txt"

record dispatch-fail-run go run ./ephemeral/issue-162/proof/testdata/dispatch_fail
expect_code dispatch-fail-run 0
record dispatch-fail-lint "$binary" lint ./ephemeral/issue-162/proof/testdata/dispatch_fail
expect_code dispatch-fail-lint 3
grep -q 'GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS' "$results/dispatch-fail-lint.stderr.txt"

record dispatch-pass-run go run ./ephemeral/issue-162/proof/testdata/dispatch_pass
expect_code dispatch-pass-run 0
record dispatch-pass-lint "$binary" lint ./ephemeral/issue-162/proof/testdata/dispatch_pass
expect_code dispatch-pass-lint 0

record rules-bad "$binary" lint ./ephemeral/issue-162/proof/testdata/rules_bad
expect_code rules-bad 3
for id in GIMBLE101 GIMBLE102 GIMBLE103 GIMBLE104 GIMBLE105 GIMBLE106 GIMBLE107; do
	grep -q "$id" "$results/rules-bad.stderr.txt"
done
record rules-good "$binary" lint ./ephemeral/issue-162/proof/testdata/rules_good
expect_code rules-good 0

record sprint-source "$binary" lint ./internal/workflows/sprint
expect_code sprint-source 3
grep -q 'sprints.go.*GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY' "$results/sprint-source.stderr.txt"
record root-source "$binary" lint -test=false .
expect_code root-source 0
record codex-review-source "$binary" lint ./ephemeral/review/codex-loop-api
expect_code codex-review-source 3
record claude-review-source "$binary" lint ./ephemeral/review/claude-loop-api/live
expect_code claude-review-source 0

record vettool-sprint go vet -vettool="$binary" ./internal/workflows/sprint
expect_code vettool-sprint 1
grep -q 'GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY' "$results/vettool-sprint.stderr.txt"

vet_config="$temporary/settings.cfg"
printf '{"ImportPath":"example.com/p","GoFiles":[]}\n' >"$vet_config"
record ordinary-run-prompt "$binary" run-prompt -h "$vet_config"
expect_code ordinary-run-prompt 0

record build just build
expect_code build 0
record go-test go test -count=1 ./...
expect_code go-test 0
record go-vet go vet ./...
expect_code go-vet 0
record just-vet just vet
expect_code just-vet 3
grep -q 'sprints.go.*GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY' "$results/just-vet.stderr.txt"

{
	printf 'PASS: injected duplicate reported before tests\n'
	printf 'PASS: failing dispatch ran deterministically and lint rejected direct and aliased map calls\n'
	printf 'PASS: explicit switch, helper, Group callback, and single-target alias linted cleanly\n'
	printf 'PASS: every accepted rule had a CLI failure and a clean counterpart\n'
	printf 'PASS: sprint and codex review retained required negative findings; root and claude review were clean\n'
	printf 'PASS: standalone, vet protocol, and ordinary CLI routes returned their exact expected codes\n'
	printf 'PASS: full tests and stock vet passed; combined just vet failed at required lint findings\n'
} >"$results/summary.txt"

cat "$results/summary.txt"
