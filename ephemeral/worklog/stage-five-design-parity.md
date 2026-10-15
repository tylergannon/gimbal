# Stage-five design parity

decision: User clarified that the task is completing the remaining 31-card Designer parity, not merely a file-reader refresh. Pinned current design sources remain at /tmp/gimbal-stage-five-designer; credentials and private run transcripts stay outside Git.
decision: Implementation begins at origin/main 925fb2e1 (v0.13.31), isolated from the root checkout's automation branch and unrelated research.
decision: Existing backend.Turn.Files and task-daemon materialization already carry exact file bytes to a selected machine. Reuse that seam for Discuss instead of introducing another upload/attachment subsystem or expiring URL context.
correction: The full web integration suite still expected legacy provider labels and the unkeyed launch-form URL. Updated its rendered-HTML assertions and submitted the actual keyed form; do not count hydration JSON as visible UI proof.
finding: A retained view can stop animation frames in WKWebView while its popover exit is pending. Unmount the workflow popover content on guide/admission so it cannot cover another tab; regression checks disable requestAnimationFrame.
finding: Keep tool names nonshrinking and truncate absolute file paths; real attached-file paths exposed a wrapped Read label in the native conversation.
decision: Reader-only native windows are identified by their route ID and preserve main-workspace persistence ownership.
finding: Live validation caught retained setup popovers whose portals remained above unrelated tabs, and inherited run controls still showing in agent focus. Gate each popup by its owning active view; unmount setup picker subtrees while inactive without losing the form draft. Hide run controls and the cancellation dialog in agent focus. Regression checks deliberately stall animation frames to reproduce WKWebView exit behavior.
