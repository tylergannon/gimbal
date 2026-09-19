# Daily validation research

doc_bug: skills/gimble-runs/SKILL.md says review is the current built-in, but installed help includes research-document and other workflows; use installed help and source for inventory.
decision: User requested two research-document workflows using Gemini and a short proposal, not implementation or scheduling. All six roles in both runs explicitly select gemini-3.8-flash-high; installed defaults are currently Codex Luna/Astra.
decision: Research target 3db61eb99397fa6e01898b0746529b5fefbd3e14 matches fetched origin/main and installed Gimble at task start. Source caches use Markdown/text to avoid adding code beneath ephemeral.

friction: First prior-art research-document run failed in planning with `agy: result arrived with unsettled tool step 4`; retried the unchanged workflow on the same Gemini model. This is a harness failure, not an editorial verdict.

friction: Second prior-art run also failed with `agy: result arrived with unsettled tool step 14`, after writing useful sources. A final bounded attempt reuses those sources and limits the plan to five topics. No product fix is included in this research task.

friction: Automated prior-art synthesis incorrectly denied Playwright CLI/MCP video support, invented Gimble video-serving behavior, and drifted toward a custom PTY engine and unsupported quantitative comparisons. Current official READMEs corrected those claims; final prose was manually condensed against primary sources. Editorial success alone did not establish factual accuracy.

friction: Repository staged-content-policy rejects the 243-file research cache (over 100 paths and 10,000 added lines). Kept the source cache locally and committed the readable top-level documents/briefs rather than overriding the policy.
