# Jev official documentation semantic index

This index routes questions about Jev 1.13 and TypeSafe's SDKs into a complete local mirror of the official documentation as retrieved on 2026-09-21.

## Scope

- **Token cache:** `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/`
- **Source coverage:** all 110 Markdown pages in the 2026-09-21 sitemap, plus `llms.txt`, `llms-full.txt`, and the sitemap itself.
- **Index shape:** task-first routes in [TOPICS.md](TOPICS.md), then source-family leaves with precise absolute `path:line` citations.
- **Authority:** leaves summarize official TypeSafe documentation. They do not convert vendor examples into independent proof. Use the separate ecosystem research corpus for independent reports.

## Route order

1. Start with [TOPICS.md](TOPICS.md) for a Gimble-oriented question.
2. Open the smallest relevant source-family route:
   - [Foundations and official patterns](routes/foundations/README.md)
   - [Cookbooks and measured examples](routes/cookbooks/README.md)
   - [JavaScript SDK](routes/javascript/README.md)
   - [Python SDK](routes/python/README.md)
3. Follow a leaf's citation bookmarks into the token cache before relying on a consequential claim.

## Citation convention

Leaves cite absolute local source paths followed by a line or line range. The local mirror preserves the page hierarchy from `https://docs.typesafe.ai/`; append the path under `pages/` to reconstruct the public page URL.

## Known debt and cautions

- Jev is text-only in this snapshot. Images, audio, and video require preprocessing by another system.
- Confidence is derived from the returned distribution and must be validated against accuracy on the target workload; it is not a guarantee for an individual result.
- The public docs leave latency SLAs, exact concurrency behavior, some retention/security details, and several retry/billing edge cases unspecified.
- JavaScript and Python SDKs changed rapidly in their first public week. Pin SDK and model versions for reproducible experiments and retain the concrete model returned by the API.
- Structural benchmark scores only test route reachability. Manually inspect cited source lines for important decisions.
