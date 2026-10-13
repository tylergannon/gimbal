# Automatic release

correction: Merged frontend changes never reached the public updater because only version tags started the build/publication workflow. Delivery must be assessed at the published application, rather than the merge.

decision: Reuse the existing release pipeline and tag-only signing environment. GITHUB_TOKEN-created tags do not trigger push workflows, so the checked-main job explicitly dispatches the existing pipeline on the new tag.

decision: Push only changelog metadata and its version tag atomically after checking the exact source revision. A newer main wins; the stale candidate skips. Record Release-Source in the metadata commit so retries preserve the original version.

friction: A failed Homebrew update after release creation made reruns fail at `gh release create` -> retry replaces same-tag assets and edits the existing release before updating the tap.

decision: Independent review identified that a historical source retry could republish an older updater feed after a newer release. Skip superseded tags and cover the case with a real bare-remote test. Automatic publication remains serialized through completion while pending main pushes coalesce.

friction: The actual Ubuntu PR gate exposed that its runner lacks zsh, Gimbal's explicit runtime command/service shell. Provision zsh in CI before exercising those paths; retain the real tests and production shell behavior.

friction: After provisioning zsh, the Ubuntu gate passed all Go tests but exposed an absent Playwright Chromium binary at frontend browser startup. Install Chromium with its system dependencies after the frontend dependencies/build are prepared, retaining all browser tests. Separate build/runtime/check steps make the next failure boundary visible.
