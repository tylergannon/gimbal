# Evaluator user testing

decision: Record the user-approved roles and non-substitution contract before the next run; foundation at ephemeral/research/daily-validation/evaluator-guide-foundation.md is intended for separate external usage and internal Gimble-on-Gimble documentation.
correction: The short Opus historical-run review proved debrief mechanics, not new-feature delivery. Next exercise must have Opus use Gimble to deliver Scrabbler #1 through a PR.
decision: Keep the existing flat workflow; Opus 5 evaluator, Sonnet-or-stronger builders/validators; preserve the public preview and report all outer assistance.

friction: Opus launched the real Sonnet target run and explored its live UI, then returned a waiting message while expecting a background notification. The harness accepted that final return, ran a five-second empty debrief, closed the browser, and ultimately exited 0. Target QA was still running. A successful Generate return is not a terminal task outcome; document foreground-turn waiting expectations and examine the lifecycle boundary before calling this unattended journey proven.
decision: Preserve the failed evaluation as observed; no outer steering, replacement run, or manual completion of the evaluator assignment. Target implementation may finish independently, but that cannot retroactively complete the evaluator journey.

friction: Target validation completed successfully after 13m24s and reported direct Chromium checks of the feature plus 18 passing tests; the evaluator had already returned after 6m11s. Preserve the distinction: target validator evidence is not evaluator acceptance, and no feature PR exists.
decision: Preserved the sole Scrabbler source modification in /private/tmp/scrabbler-drag-drop-eval; did not commit or publish it on the evaluator's behalf. All test listeners stopped, and the user's separate preview on 43910 remained running.
doc_bug: Final synthesis usefully corrected Flash's overstated fit-to-view and whole-inspector claims. Include evidence limits and stage-specific outcomes in usage guidance rather than treating successful synthesis as full-task proof.

friction: Filed https://github.com/tylergannon/gimble/issues/317 for the premature Claude background-wait completion and empty successful same-session follow-up. The Claude tool explicitly promised a completion notification; root cause remains unestablished. A local Codex shell probe yielded a session_id and completion was retrieved with write_stdin, without an automatic-notification promise. Do not conflate app-server output notifications with waking model generation, or infer universal provider semantics from one tool surface.

decision: User is arranging the Claude lifecycle fix separately and requested rerunning with gpt-5.6-sol:medium as evaluator. Started a fresh worktree at the same Scrabbler baseline, retained the same Gimble binary and assignment (only paths/branch changed), and kept Sonnet 5 implementation roles, Flash screenshots, Astra synthesis, and report-only issue handling. No waiting-workaround prompt change was introduced for this rerun.

decision: Sol medium completed the real task and opened https://github.com/tylergannon/scrabbler/pull/8 (46339617eb4f4fc56a3dbc4455af133f8314a2e2), unmerged. Task 14m49s, same-session debrief 29s, no outer steering/rescue. This proves one complete journey, not universal model superiority or a fix for #317.
friction: Sol's capture wrapper assigned zsh's reserved `status` and failed after Gimble succeeded; it correctly separated and recovered from its own error. Keep observer/tool failures distinct from target status.
friction: Flash identified an omitted initial 500 screen, two captions overclaiming off-screen activity, and a wrong screenshot reference. Successful delivery does not remove the need for image-grounded evidence checking.

decision: At user request, uploaded 10 original screenshots via gimble upload-artifact and verified public image/png responses. Published #323 (caption accuracy), #324 (watcher-result discoverability), #325 (inspector readability/overflow), and #326 (initial history 500), with per-image explanatory captions. Added same-run screenshot evidence to #261 and #310. Retained observed-build identifiers and evidence limits; did not claim that screenshots diagnose internals or that a missing result in one panel means the result was lost.
