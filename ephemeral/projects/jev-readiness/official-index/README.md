# Jev official documentation semantic index

This index routes questions about Jev 1.13 and TypeSafe's SDKs through summaries of the official documentation retrieved on 2026-09-21. It is the primary semantic index for this research; recheck version-sensitive facts against the linked live docs.

## Scope

- **Source catalog:** [saved llms.txt](../sources/official-docs/llms.txt) from 2026-09-21.
- **Source coverage:** summaries from the 110 Markdown pages in the 2026-09-21 sitemap, linked to official pages rather than copied into this repository.
- **Index shape:** task-first routes in [TOPICS.md](TOPICS.md), then source-family leaves with links to official source pages.
- **Authority:** leaves summarize official TypeSafe documentation. They do not convert vendor examples into independent proof. The [Go SDK and supervision decision](../sdk-and-supervision.md) is separate and includes independently checked community SDK evidence.

## Route order

1. Start with [TOPICS.md](TOPICS.md) for a Gimbal-oriented question.
2. Open the smallest relevant source-family route:
   - [Foundations and official patterns](routes/foundations/README.md)
   - [Cookbooks and measured examples](routes/cookbooks/README.md)
   - [JavaScript SDK](routes/javascript/README.md)
   - [Python SDK](routes/python/README.md)
3. Open a leaf's official source links before relying on a consequential claim.

## Citation convention

Leaves cite official Markdown page URLs. The saved `llms.txt` catalog retains line references where a leaf discusses that catalog itself.

## Known debt and cautions

- Jev is text-only in this snapshot. Images, audio, and video require preprocessing by another system.
- Confidence is derived from the returned distribution and must be validated against accuracy on the target workload; it is not a guarantee for an individual result.
- The public docs leave latency SLAs, exact concurrency behavior, some retention/security details, and several retry/billing edge cases unspecified.
- JavaScript and Python SDKs changed rapidly in their first public week. Pin SDK and model versions for reproducible experiments and retain the concrete model returned by the API.
- Structural benchmark scores only test route reachability. Manually inspect cited source pages for important decisions.
