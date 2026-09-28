# Required Jev supervision

decision: Tyler requires TYPESAFE_API_KEY when starting Gimbal and explicitly
rejects a timed-supervision fallback or switch. Remove the replaced engine and
WithInterval API. Enforce the key requirement at hosted instance startup and
standalone Run entry, before creating state or starting work.

scope: Batched questions and separate coding/validation coaching are researched
in Gimbal View's supervision-readout worktree. This implementation changes
credential requirements and removes timed review; it does not claim those
proposed prompts are implemented or validated. Existing Jev escalation
cooldowns are a separate unresolved design concern.

finding: Existing timed-supervision lifecycle tests must emit completed
reasoning and use a local fake Jev HTTP endpoint to exercise the replacement
path. Tests for the deleted timed transcript buffer are removed with it.

friction: The desktop managed-worktree tool is bound to this chat's Gimbal View
repository and has no cross-repository argument. Used a separate Git worktree
for the authorized Gimbal runtime edits after inspecting the available tool.

finding: Independent validation found no material requirement failure. The
unchanged docs/definition-of-done.md retains obsolete timed-supervision text;
the maintained README, Godoc, and operating/authoring skills now state the
required-key and event-driven behavior. Protocol prohibits edits under docs/
without express permission, so this historical policy inconsistency is noted.

finding: The built CLI rejects absent-key server and run-prompt startup before
creating their state directories, while help still succeeds. A nonempty dummy
key allows a listener to serve HTTP, establishing presence validation only,
not credential validity. Simulated four-minute tests demonstrate no timer
review after missing thinking or a failed Jev screen. Full build, Go/frontend
tests, vet/lint, and formatting checks passed. Frontend tests log a nonfatal
ResizeObserver warning. No live Jev judgment-quality claim is made.
