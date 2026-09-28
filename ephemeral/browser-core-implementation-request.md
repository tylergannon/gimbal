Implement the browser runtime and generator portion of the accepted current
ephemeral/browser-implementation-design.md against all requirements in
ephemeral/browser-evaluator-build.md. Read repository instructions. You are
not alone in this worktree; preserve others' edits and accommodate their work.

Own only browser.go, browser_test.go, scope.go, session.go, supervise.go,
supervise_jev.go, their directly necessary root tests, doc.go, and
internal/generate source/tests needed for NewBrowser. The linter is another
worker's responsibility; backend/execution and validateproduct are other
workers' responsibility. Do not edit them. Keep the agreed public API stable
for the evaluator worker. Report a genuine needed contract change immediately.

Implement actual environment-bound browser creation, explicit access through
WithBrowser, scope lifetime, correct shutdown/error propagation, and narrowly
needed generator support. No generic resource framework or new public API
beyond the accepted design. Use the tested connector facts in
/tmp/gimbal-browser-build.AfTh8P/image/README.md. Test the important lifecycle,
access, same-directory browser identity and error cases. Run focused Go tests
for your changes. Do not claim the Docker evaluator proved from fakes.

Do not commit, launch agents, invoke Gimbal, or edit generated files by hand.
The manager handles integration, regeneration, full review and live validation.
Finish by listing changed files, actual tests/results and unresolved problems.
