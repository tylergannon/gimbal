# Issue 272 after the runs-card redesign

The current RunsList card CSS constrains the grid and clamps ending summaries to two lines with overflow wrapping. The old table-width repair no longer applies. An independent read-only browser assessment confirmed the card contains a 51,000-character unbroken summary at desktop and phone widths.

Loading feedback remains missing during client navigation to Runs. The root layout has no navigation indicator and the Runs page renders only empty or populated content. A delayed Runs data response leaves the previous page visible without feedback.

Next scope: show loading feedback from the already-mounted layout while navigating to Runs, with a skeleton appropriate to cards. Keep existing cards visible during background refresh. Add browser regressions for delayed navigation and long summaries. A page-local skeleton cannot appear before an initial synchronous Go document load; changing that architecture is a separate decision, not implicit in this fix.

This is a reassessment only; issue 272 remains open and its implementation is outside bundle 2.
