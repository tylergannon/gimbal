# Fast feedback and coherent release instructions

correction: User asked for a complete release-skill rewrite aligned with the active generator/test performance lanes, meaningful commit-hook tests, and measured release timing with deterministic issue creation. Timing overruns are advisory housekeeping findings, never performance-induced product-test or publication failures.

decision: Skill reduced from 264 to 74 lines. Normal workflow is scoped PR checks, one complete main gate, automatic platform builds/publication, and requested local installation. Rare build/catalog/signing knowledge remains in references. Removed the prices workflow's generated PR instructions that reintroduced pre/post-tag build rituals.

decision: Current published consumer is v0.13.9 at source c460bcbc; persistent generation cache PR528 remains in progress in the linked lane. No speculative cache or native-reload behavior is counted as landed performance. Commit-hook pack executes 69 uncached test/subtest cases; warm complete-hook runs measured 0.492–1.015s. Cold-cache overrun prints the requested warning, records unfinished checks, exits 0, and leaves actual correctness failures intact.

decision: Measured five public main→feed/Homebrew releases through named Actions run timestamps: v0.13.4 873s, v0.13.5 802s, v0.13.6 842s, v0.13.8 603s, v0.13.9 801s. Current v0.13.9 active critical path 784s, largest parallel platform 434s; parent dispatch waiting is excluded from active totals. Goal 300s/max 600s; hook goal 1s/max 3s. Queue and retries remain in elapsed delivery time.

decision: Separate trusted-main Release performance observer checks requested/completed main runs and every five minutes; hourly warm representative hook benchmark. Source-SHA incident markers deduplicate issues. Real v0.13.9 check created issue529; repeating the same check reused it with created=false. Performance overruns exit 0. Missing timestamps/API failure remain explicit observer failures rather than claims of a good measurement.

friction: Initial hook fixtures asserted wall time under three seconds, making slow hardware a release blocker despite advisory CLI behavior. Independent review caught this; removed performance assertions, retained correctness/warning/cleanup checks and a generous true-hang timeout. Explicit warmup allows normal module fetching, while timed hooks remain offline.
