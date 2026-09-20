# Bug 317 investigation

decision: Investigate structured-output and native wakeup semantics before selecting a repair; application behavior remains unchanged pending that choice.

friction: Normalized events hide the orphan-task result that can precede a resumed prompt's answer -> use raw SDK events and native session history when investigating result attribution.

decision: A valid structured result and native terminal_reason completed can coexist with unfinished background work; neither establishes assignment completion. Origin distinguishes task-notification results from ordinary prompt results in the observed cases.
