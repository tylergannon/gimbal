# 2026-09-13 operator control arc

- Tyler's reservation on #165: an operator on the run page needs to kill an agent without the pointer. Answer: keep the cancel funcs the library already creates (scope.do, Group) reachable by id on the run, cancel with a cause (`Killed`), one ctx per turn in Generate. No wrapper around context.Context. Filed as #175.
- Arc #179 groups every Beta item that does not touch #173's surface (internal/observation, web/src, generated skgo). Held: runs index, page buttons, #135, #117.
- PR #158 merged first (tests pass locally; CI only builds docs, so every PR's Go tests are run locally before merge).
- Execution order: wave 1 = #175+#165, #118+#111, #106+#112, #163, #168 in parallel worktrees; then #159, #107+#113, #176; then #177, #178.
