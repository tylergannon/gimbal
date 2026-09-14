# 2026-09-13 operator control arc

- Tyler's reservation on #165: an operator on the run page needs to kill an agent without the pointer. Answer: keep the cancel funcs the library already creates (scope.do, Group) reachable by id on the run, cancel with a cause (`Killed`), one ctx per turn in Generate. No wrapper around context.Context. Filed as #175.
- Arc #179 groups every Beta item that does not touch #173's surface (internal/observation, web/src, generated skgo). Held: runs index, page buttons, #135, #117.
- PR #158 merged first (tests pass locally; CI only builds docs, so every PR's Go tests are run locally before merge).
- Execution order: wave 1 = #175+#165, #118+#111, #106+#112, #163, #168 in parallel worktrees; then #159, #107+#113, #176; then #177, #178.

## Landed 2026-09-13 (evening)

Squash-merged in order: #183 (#163 fd limit), #184 (#111 ulid ids, #118 project seq dropped), #185 (#175 cancel by id with `Killed`, #165 Interrupt deleted; I added the ctx.Err wrap so a killed turn still matches context.Canceled), #186 (#168 sprint: validator informs planner, `-issue` file or number, `-dry-run`), #187 (#159 infallible Set, panic boundary), #188 (#176 registry via internal/live hook; release deferred so a panic still clears the table), #189 (#107 Steer returns landed; registry Steer carries it through), #190 (#106 just vet/test/attest; #112 probes tagged ignore).

- Every PR's Go suite was run locally before merge; CI only builds the docs site.
- The Claude CLI is logged out on this machine: #113 stays open (Refs), the sprint validator and the attest group/supervised legs did not run live. `claude login` then `go run ./ephemeral/attest/issue-113` is the check.
- Auto-merge is not enabled on the repo; merge by polling mergeStateStatus until it leaves UNSTABLE.
- Still running: #177 skill, #178 practice runs.

## Arc closed 2026-09-13 (night)

- #191 (#177 skill at .agents/skills/gimble-workflows, symlinked from .claude/skills; shapes as compiling Examples) and #198 (#178 four Loop practice runs, each through web.NewRuntime on the cheap tier) merged. Twelve of the arc's fourteen sub-issues are closed; #113 waits on `claude login`; #179 stays open only for it.
- The practice runs' operator list (ephemeral/attest/loop-practice/README.md) is the input for the page work held for #173: steer and kill records on the page (#197), values as text (#193), errors that are errors (#195, #196). Also filed: #192 runlog.Read races run.jsonl, #194 backlog.md empties when the planner ends.
- Lesson worth keeping: a planner ends a loop in seconds when the workflow records a deterministic verdict beside the worker's claim (shape 2), and spins to the cap when it only has filenames and flags (shape 3). The sprint workflow's validation command is that verdict.
- Kill by id proven twice live: attest/registry (gpt-5.6-luna) and shape 4's second goroutine holding only the run id.
