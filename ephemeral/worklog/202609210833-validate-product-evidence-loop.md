correction: Product evaluation needs Gemini screenshot review to feed unsupported visible claims back to the same evaluator while its browser and session remain live, not merely report mismatches after evaluation ends.
decision: Keep the tactic inline in each of the three explicit tester arms: assemble the full task and UI/UX report, review every cited image, allow one correction and one recheck, then preserve any remaining mismatch for triage.
decision: A reviewer that cannot open every cited image produces an evidence error rather than treating the evaluator's claim as disproved.
decision: Rebase onto main commit aeba1bf preserved its new saved-image grounding prompts and made the correction loop build on those constraints.
friction: GIMBLE103 correctly rejected setting visual feedback inside a bounded loop as a potentially repeated scope write -> write each verdict to the workload's local visual-review.md and have the evaluator read that named file.
