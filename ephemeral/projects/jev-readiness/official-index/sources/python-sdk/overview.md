# Python SDK overview and quickstart

## Purpose

This leaf covers the top-level Python SDK page that sits outside the recursively documented API directory and completes coverage of the official documentation corpus.

## Key concepts

- The official Python package is `typesafe-sdk`, with both synchronous and asynchronous clients and a link to the SDK source repository. [python](https://docs.typesafe.ai/sdk/python.md)
- Authentication is read from `TYPESAFE_API_KEY`; the page demonstrates mixing `Noul`, `Choice`, and `Score` questions against one structured state in a single request. [python](https://docs.typesafe.ai/sdk/python.md) [python](https://docs.typesafe.ai/sdk/python.md)
- Both clients are context managers, and the async and sync examples expose primitive-specific result maps rather than one untyped answer bag. [python](https://docs.typesafe.ai/sdk/python.md) [python](https://docs.typesafe.ai/sdk/python.md)

## Citation bookmarks

- Package and source: [python](https://docs.typesafe.ai/sdk/python.md)
- Install and authenticate: [python](https://docs.typesafe.ai/sdk/python.md)
- Async example: [python](https://docs.typesafe.ai/sdk/python.md)
- Sync example: [python](https://docs.typesafe.ai/sdk/python.md)

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
