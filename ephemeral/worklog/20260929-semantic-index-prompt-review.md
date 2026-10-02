correction: Tyler defines research-document and semantic indexing as factual collection, organization, compression, and navigation. Model-authored opinions, recommendations, deductions, engineering designs, and interpretive synthesis are outside the task. Judgment is limited to faithful selection and summarization.

finding: Existing producing prompts and coaches explicitly permit separately labeled interpretation and recommendations. Those permissions predate PR #410; its auditing additions did not replace the task contract.

finding: The existing Jev priority-queue assignment explicitly requests proposed engineering designs and actionable synthesis. Correcting workflow constants alone would leave contradictory task input reaching researchers and curation.

finding: The current research-eval rubric checks supported facts and retrieval but does not explicitly reject model-authored opinions or recommendations. Correct the task and that criterion before attributing repeated failures to model choice.

decision: Apply one factual-artifact contract across planning, collection, curation, document writing, coaches, audit repair, and review. Clips are verbatim excerpts; curation removes authored interpretation from leaves rather than routing readers to it. Keep existing orchestration and audit mechanics within this prompt correction.

decision: The requested live experiment uses a fresh project and the exact topic supplied by Tyler, without importing the prior assignment's synthesis instructions. Use Gemini Flash for planning, collection, and coaching, Haiku for curation, and Luna for writing and review.

finding: The first collection attempt saved a model-authored combined integration summary as an original, with broad site URLs. Stop before auditing that cache. The source-cache contract must explicitly require downloaded originals or mechanically extracted text from one identifiable URL/document; authored summaries belong only in index leaves.

finding: Neutral question wording can still presuppose undocumented implementations and capabilities. State that planning must not assume those exist. Collection also needs a boundary excluding generated research reports and Gimbal run records from its evidence.

correction: Preserving the existing orchestration was an unjustified constraint. Tyler explicitly authorizes replacing it to produce good semantic indexes and forbids exhaustive audits and proof machinery. Jev is optional, useful only for focused checks serving the result.

finding: The live trial's all-pairs audit diverted work from source fidelity and retrieval. Remove claim extraction, pairwise auditing, their CLI/package, and eval accounting; retain factual collection, compact routing, a few independent source checks, targeted repair, and document compression.

finding: Focused source review produced useful qualification and citation corrections, but mistook UTC retrieval dates for future dates relative to the client calendar. Supply the actual UTC research timestamp to collection and index review rather than treating an unspecified timezone difference as a defect.

finding: Haiku repaired many source-fidelity defects but repeatedly softened unsupported absence/design assertions rather than deleting them. Make that repair distinction explicit, give positive reader observations a Summary field separate from MaterialIssues, and recheck existing findings without expanding the review. Reuse downloaded originals for the next clean-index trial with Luna curation.
