Independently validate the implemented browser-evaluator slice at the current
revision. Read repository instructions and ephemeral/browser-evaluator-build.md
for the intended claims, but derive your verdict from actual behavior. Do not
accept implementation summaries as proof.

Built artifacts and operating setup:
- Controller: /private/tmp/gimbal-browser-build.AfTh8P/gimbal
- Linux arm64 worker: /private/tmp/gimbal-browser-build.AfTh8P/gimbal-worker-linux-arm64
- Consumer image: gimbal-browser-evaluator:local
- Docker is running. Temporal is 127.0.0.1:7233; the existing Postgres container
  gimbal-postgres-1 exposes port 5433. Configuration is at
  /private/tmp/gimbal-browser-build.AfTh8P/execution.json.
- The prepared practical TodoMVC task and suite are in
  /private/tmp/gimbal-browser-build.AfTh8P/evaluation. The suite omits issue_repo.

Exercise the real user entry point and inspect results. Give your own temporary
instance a unique path under /private/tmp/gimbal-browser-build.AfTh8P/validation.
Use gpt-5.6-luna for all evaluator roles. Do not provision credentials, inspect
secret contents, or copy host login state. Authentication has not been supplied;
if the worker cannot run the evaluator, report the exact boundary reached and
which claims remain unproved. Never use a dummy key to present a provider pass.

Separately run the supported no-model worker readiness and cancellation
integration checks using the built worker and prepared image. Verify their
workspace effects and resource cleanup. Inspect the available scope/linter/
cancellation tests and run focused cases where needed to assess what they
actually establish. A fake harness is not a real model turn. You may inspect
external image recipe/probe files at /tmp/gimbal-browser-build.AfTh8P/image,
but distinguish prior artifacts from behavior you personally exercised.

Own only ephemeral/reviews/browser-independent-validation.md. Operational files
belong under the external validation directory, never Git. Report the revision,
commands, results, credible realized claims and unmet claims concisely. Do not
modify product code, commit, publish issues, or launch further agents. Preserve
others' edits and shared infrastructure; stop your own instance and remove only
resources your checks created through their normal cleanup paths.
