# Independent file reader delivery

correction: Existing draft #572 is unaccepted reference material. Start from main 925fb2e1 and selectively extract only issue #573, proving it before #574.
finding: Computer Use initially reported failed automatic unlock; resetting its JS runtime did not help. Restarting the exact socket-owning helper restored Finder screenshot and navigation. Try the exact helper before requesting manual unlock when Locked Use is enabled.
decision: Reuse project-owned file identity, private persistent cache and existing remote TaskFile materialization. Do not inline file bytes into request or transport journals.
finding: Independent validation found that Discuss in a detached reader would navigate that reader into a second workspace draft. The document-only native handoff now brings the main workspace forward and navigates its retained view through canonical Discuss; it does not introduce general multiwindow state.
