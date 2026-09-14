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
