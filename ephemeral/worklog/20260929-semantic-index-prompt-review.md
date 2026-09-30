correction: Tyler defines research-document and semantic indexing as factual collection, organization, compression, and navigation. Model-authored opinions, recommendations, deductions, engineering designs, and interpretive synthesis are outside the task. Judgment is limited to faithful selection and summarization.

finding: Existing producing prompts and coaches explicitly permit separately labeled interpretation and recommendations. Those permissions predate PR #410; its auditing additions did not replace the task contract.

finding: The existing Jev priority-queue assignment explicitly requests proposed engineering designs and actionable synthesis. Correcting workflow constants alone would leave contradictory task input reaching researchers and curation.

finding: The current research-eval rubric checks supported facts and retrieval but does not explicitly reject model-authored opinions or recommendations. Correct the task and that criterion before attributing repeated failures to model choice.

decision: Apply one factual-artifact contract across planning, collection, curation, document writing, coaches, audit repair, and review. Clips are verbatim excerpts; curation removes authored interpretation from leaves rather than routing readers to it. Keep existing orchestration and audit mechanics within this prompt correction.

decision: The requested live experiment uses a fresh project and the exact topic supplied by Tyler, without importing the prior assignment's synthesis instructions. Use Gemini Flash for planning, collection, and coaching, Haiku for curation, and Luna for writing and review.
