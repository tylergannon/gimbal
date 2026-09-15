# Usable builtin workflows

Native proof on 2026-09-14, using Codex `gpt-5.6-luna`, Claude `haiku`, and Gemini `gemini-3.8-flash-low`. These are real harness runs, not the fake adapters used by unit tests.

| Route | Run | Result |
| --- | --- | --- |
| `lfg` | `01M2HJDKYV9DBEVF5822RRMBNM.lfg` | One worker wrote the greeting from scoped input; seven supervisor looks; fixed check `LFG_GREETING_OK`. |
| `plan` | `01M2HJDMNVNWA4AFSF35THCR5H.plan` | Three independent drafts, three cross-critiques, synthesis; saved the slug implementation plan. |
| `work` | `01M2HJJ71JVCSA0Q3BHFT70AP2.work` | Guide selected LFG for the small request; supervised worker produced exact output; fixed check `GUIDED_LFG_OK`. |
| `sprint` | `01M2HJTP7P3YZ84H789KCG1DSD.sprint` | Consumed the saved plan, dispatched a supervised coder, independently validated the files, passed the fixed 12-case check, and finished locally. |

All four have successful run completion and durable `complete` records. See `completed-proof-audit.md`, `sprint-proof.json`, the command-output files, the fixture source/check files, and `planning-artifacts/plan.md`. The three `*-native.tar.gz` archives preserve the complete `.gimble` request and run records, including native transcripts. Extract each archive into its corresponding workspace to restore the original layout. The plan archive also includes the Sprint runs. The earlier absolute-path startup failure is retained in `sprint-path-failure.txt` and the plan archive; it led to a path normalization regression fix.

The saved fixed checks were rerun independently and passed. Post-run input hashes are in the audit; they are not represented as pre-run attestations. The planning run was observed before implementation: its native transcript and completion time distinguish it from the later Sprint.

After these native runs, review fixes restricted final task-check replay to failed checks, added nonlocal delivery preflight, moved input cancellation ownership to the CLI, clarified original-goal precedence, and propagated explicit checks into planning prompts. Focused tests cover those changes; the entire native sequence was not repeated. PR/merge publication was not exercised against a remote; native delivery proof is for the default local mode. A short worker can finish before a supervisor's first interval; these fixtures observed actual looks.

Validation: `go generate ./...`, `go test ./...`, `go vet ./...`, `gimble lint ./...`, web production build, and all 17 web tests passed. Round-02 independent review found the check handoff omission; round-03 independently verified its fix. Interactive clarification and cancellation use fake-adapter/reader tests; the native guided run used `-yes`.
