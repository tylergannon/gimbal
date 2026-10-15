# Session setup and workspace controls

scope: Issue 573 was independently accepted and merged as PR 579 before starting 574 from main 1e387b34. Reuse only the session/workspace subset of frozen draft e75c6c19; preserve accepted file behavior and existing recording/workflow controls.
finding: The old draft duplicates effort policy in conversation and rewrites effort-suffixed model IDs. Consolidate supported choices with existing model selection knowledge, reject unsupported explicit effort before admission, and preserve empty Default rather than forcing low.
finding: Old retained-dialog guards hide a rename dialog while inactive but keep its open flag; returning would reopen stale UI. Leaving must close the dialog without discarding the message form.
