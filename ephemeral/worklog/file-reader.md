# Independent file reader delivery

correction: Existing draft #572 is unaccepted reference material. Start from main 925fb2e1 and selectively extract only issue #573, proving it before #574.
finding: Computer Use initially reported failed automatic unlock; resetting its JS runtime did not help. Restarting the exact socket-owning helper restored Finder screenshot and navigation. Try the exact helper before requesting manual unlock when Locked Use is enabled.
decision: Reuse project-owned file identity, private persistent cache and existing remote TaskFile materialization. Do not inline file bytes into request or transport journals.
finding: Independent validation found that Discuss in a detached reader would navigate that reader into a second workspace draft. The document-only native handoff now brings the main workspace forward and navigates its retained view through canonical Discuss; it does not introduce general multiwindow state.
finding: Native file handoff left previously closed setup popovers painted above the retained draft in WKWebView. Unmount only the setup-picker subtree while inactive, keeping the message form and choice state mounted. This is needed for an unobstructed file handoff, not a session-picker redesign.
friction: Calling CUA getAXState after Command-Q relaunched the path-bound app without its catalog environment. Stop the exact test process from the shell and do not observe an exited app until it has been relaunched with the disposable environment.
