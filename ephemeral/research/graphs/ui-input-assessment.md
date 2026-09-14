# Assessment of the supplied hierarchy design

Source: Tyler's attached Screenshot 2026-09-14 at 10.04.55 AM.png, copied unchanged to `ui-input-hierarchy.png`. This is a critique of the visible frame, not a claim about unshown tabs or interactions.

The left outline exposes plan / implement / laps / sessions / verify. The selected lap gets a breadcrumb, outcome/duration, four local tabs, and a table of two sessions. The useful capability is location and ownership navigation.

The visible frame does not answer the main workflow questions:

- What precedes and follows this work? The folder order supplies a weak suggestion, not control flow.
- What executes in parallel, what joins, what repeats, and what selects the next task?
- Which supervisor watches which turn, and what did a steer actually affect?
- What commands did the workflow itself run, including work outside agent sessions?
- Did the failed lap stop the run or was the failure handled? A red ancestor plus a running run plus a later completed lap is ambiguous without outcome/continuation context.
- Is this the whole workflow shape, a partial static graph, or only already observed work?

The `Shape` tab exists in the screenshot; its contents are not visible, so this assessment does not claim that no shape view exists anywhere. The issue is that the shown landing/detail composition gives almost all the main area to two inventory rows. A row of Agents / Shape / Metrics / Contents at every scope makes a user assemble the workflow mentally across tabs and levels.

Preserve the outline as a compact navigator or optional drawer. Put executable shape in the main area. Selection should open details without discarding the graph, position, iteration selection, or run context. Use a common selection across map, timeline, and inspector. Scope nesting is one relationship among several; it should not determine every visual axis.

The new design must demonstrate the provided difficult state: a run continues after a failed iteration. Show the failed instance and current activity together without falsely turning the whole run into a failed run or claiming that a completed scope's elapsed duration is all agent time.
