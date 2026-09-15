# Sprint Plan and semantic-index builtin proof

Issue: https://github.com/tylergannon/gimble/issues/221

The fixture is a local 12-document cache and a project containing only a small design request. Expected retrieval facts were saved in `retrieval-expectations.json` before indexing, and were not supplied to the retrieving agent. `source-before.json` and `source-after-build.json` show that indexing preserved the corpus.

## Native build and retrieval

`index-build.txt` records run `01M2JQPYX8C7KVNGH4SRYFKNGM.semantic index`: twelve Codex gpt-5.6-luna readers, at most three concurrent, produced twelve cited leaves and no source debt. `initial-index/` preserves that output and `initial-cache/` preserves the original sources.

`retrieval-output.txt` records run `01M2JR210FJQ6VZCZ94ESWS7JF.lfg`: a Codex gpt-5.6-luna worker, supervised by Claude Haiku, walked the README, routes, three relevant leaves and source files. Its answers matched the prewritten expectations: 42-minute inactivity with no failed-login refresh; 06:20–06:40 UTC normal deployments; zero-based retry delays increasing by seven seconds and capped at 28. The full tool transcript is preserved with the native records. Generated eval target lists were not treated as retrieval scores.

## Planning

`plan-final-output.txt`, run `01M2JRQF1DHQDYS9AXF1J7DNTS.plan`, completed successfully with Codex gpt-5.6-luna, Claude Haiku and Gemini gemini-3.8-flash-low. It automatically discovered the project index pointer, retrieved the policy through the bounded routes, produced three independent drafts and three cross-critiques, printed their comparison before asking the missing filename, and saved the exact answer. The final plan uses `retry.py` and `test_retry.py` at the project root, preserves the formula and `python3 -m unittest discover`, and records actual accepted/rejected choices and interview effects in `merge-notes.md`. `completed-plan/` contains the eleven readable artifacts plus request; `planning-result.json` records checks. No Python implementation files were created, and the source cache stayed unchanged.

The full Plan ran with the original strict agy ACTIVE-step guard. The final code additionally recognizes explicit native ERROR states (proved separately below) and rejects blank final decisions after an interview (regression test). The successful final response supplied nonempty final-turn decisions, so it exercises the same successful planning path.


`plan-output.txt` preserves an incomplete first run: Gemini Flash returned an external 503 capacity error during drafting. This is not evidence of a successful plan. Its orientation also drifted into a proposed plan; the prompt was narrowed to observed facts, policy, citations and unresolved questions before the retry.

## Incremental checks

`check_incremental.py` intentionally changes one policy and deletes a different source, then checks stale audit failure without writes, exactly one agent read during update, deletion of the obsolete leaf, preservation of ten unchanged leaves, and a clean audit without model calls or writes. This is a one-shot fixture mutation, not an idempotent production tool.

## Scope

The builtin indexes local regular UTF-8 files up to 256 KiB each; unsupported files are explicit coverage debt. It does not sync remote sources. This small native corpus proves the end-to-end path; it is not a throughput or retrieval-quality claim for a billion-token cache.

The second planning attempt (`plan-retry-output.txt`, run `01M2JR8Y69M436854J19YMHRCK.plan`) demonstrated facts-only orientation and Codex/Claude drafts, then failed in the Gemini adapter with `result arrived with unsettled tool step 12`. That failure is preserved separately from any successful final run.

The incremental checker passed. `incremental-result.json` records the assertions; `index-audit-stale.txt` shows one changed and one deleted source, `index-update-clean.txt` shows one reader turn, and `index-audit-clean.txt` reports no stale/deleted/added sources afterward. `source-intended-update.json` captures the only intentional corpus edits. The installed `index/` now uses bounded routing pages; unchanged leaf bytes remained intact.

`retrieval-paged-output.txt`, run `01M2JRN8KZ9BB63Z1H2MNYZ0F5.lfg`, demonstrates the final routing shape: README → routes.md → routes/page-0003.md → cited retry-policy leaf → source line 3. The Luna worker, supervised by Haiku, returned the correct values for attempts -1, 0, 1, 4 and 10 without modifying files.

## Adapter recovery and checks

[The agy probe](agy-error-probe/summary.md) identified the missing terminal `ERROR` handling and proves the fix through a native Gemini Flash run: failed lookup recorded as failed and ended, later successful read retained, run completed. Genuinely unfinished ACTIVE tools still fail. No heuristic based on subsequent steps was accepted.

Full `go test ./...`, `go vet ./...`, `golangci-lint run ./...`, `gimble lint ./...`, and the production web build passed. Commit hooks also passed. Implementation checkpoint: `c4b33f2`. Independent review: `ephemeral/reviews/planning-index-final-review.md`.

Raw native records and original fixture snapshots remain local in the ignored `native-records.tar.gz`; they are not committed as run logs. Stored absolute paths name the worktree used for the attestations.
