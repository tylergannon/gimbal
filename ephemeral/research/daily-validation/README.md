# Daily product validation research

- [Feature inventory](feature-inventory.md): 63 feature-family entries grouped by product use, with observable checks and limits.
- [Prior art](prior-art.md): a compact tool comparison, primary sources, and integration questions.
- [Proposal](proposal.md): the recommended next step in roughly three-quarters of a page.
- [Detailed feature research](features.md): the workflow-produced inventory, with subsequent editorial corrections.

The two research subjects were run through Gimble's `research-document` workflow with **gemini-3.8-flash-high for every role**. Feature research completed after two editorial passes. Prior-art research completed on its third attempt; the first two stopped on agy unsettled-tool transport errors. The final documents were edited against current source and official documentation after workflow completion. No product implementation or daily schedule was created.

`features-sources/`, `prior-art-sources/`, and `prior-art-final-sources/` contain the local, uncommitted research cache. The repository content policy requires committing the summaries rather than this large generated cache. Links into those directories resolve in this worktree; original repository paths and primary web links remain in the documents for readers elsewhere. These include automated interpretations, not just original quotations, and are not independent proof. In particular, earlier notes misstate Playwright recording capabilities and contain overextended design suggestions. Use the final documents and [checked source corrections](checked-sources/INDEX.md) when those conflict. Run logs remain in the local `.gimble` store and are not committed.
