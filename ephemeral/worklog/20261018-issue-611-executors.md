# Issue 611

decision: Native adapters remain responsible for harness semantics. Independent command and agent processes own acceptance, execution, file retention, and controls; caller observation does not own their lifetimes.
decision: Operation locks establish live ownership. A missing socket cannot establish native loss. Retained terminal operations deduplicate for seven days; session continuation state is not vacuumed.

correction: Tyler requires unversioned model aliases in AGENTS.md (luna, haiku, flash), not hard-coded model versions.
