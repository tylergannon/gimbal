# Run-store foundation

decision: Implement the five ordered milestones with a commit after each, then independent adversarial review of the seam, completeness, bypasses, and application proof. Scope and reviewed mechanics are in ../run-store-design.md and ../run-store-session.md.

decision: This Gimbal worktree is a sibling of the attached Gimbal View worktree in the same task directory; runtime implementation belongs in Gimbal, per both repositories' instructions.

decision: Unit tests cover meaningful ownership, persistence, identity and failure contracts. Final acceptance includes running application behavior and an independent reviewer, not only passing checks.

decision: Launchers allocate bare ULIDs, the runner forwards invocation.Request.RunID, and the engine accepts only its explicit Run argument. Scope/session/action identity is separate from existing prefix-based display/control addresses. Constructor I/O happens outside scope.mu.

correction: Registry.add previously replaced a live object on duplicate registration, and IDs listed bare directories. Reserve without replacement and distinguish missing runs from incomplete initialization before opening an external projection.
