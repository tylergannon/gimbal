# Python SDK overview and quickstart

## Purpose

This leaf covers the top-level Python SDK page that sits outside the recursively documented API directory and completes coverage of the official documentation corpus.

## Key concepts

- The official Python package is `typesafe-sdk`, with both synchronous and asynchronous clients and a link to the SDK source repository. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:5-13`
- Authentication is read from `TYPESAFE_API_KEY`; the page demonstrates mixing `Noul`, `Choice`, and `Score` questions against one structured state in a single request. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:19-35` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:41-65`
- Both clients are context managers, and the async and sync examples expose primitive-specific result maps rather than one untyped answer bag. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:45-64` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:68-93`

## Citation bookmarks

- Package and source: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:5-13`
- Install and authenticate: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:19-35`
- Async example: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:39-65`
- Sync example: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python.md:68-93`

## Themes

- One request can fan several atomic judgments over the same state.
- Client lifecycle is explicit in both Python styles.
- Primitive-specific result collections make the API easy to inspect but must be correlated with the original question keys.

## Gotchas

- This page is a quickstart, not an operational contract: it does not define retry, timeout, concurrency, cancellation, logging, or version behavior.
- It does not establish multimodal support; the model capability pages remain authoritative for the text-only boundary.

## Task recipes

- For a quick Python probe, start here, then follow [client lifecycle and calls](client-lifecycle-and-calls.md) before relying on behavior under failure.
- For result metadata and request identity, continue to [questions, results, and observability](questions-results-and-observability.md).
- For production retry and version risks, continue to [retries, errors, and version drift](retries-errors-and-version-drift.md).
