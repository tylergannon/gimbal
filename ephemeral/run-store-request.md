# Implementation and acceptance

Implement the reviewed design in run-store-design.md and the acceptance in run-store-session.md. The user approved five milestones, in order, with a commit after each: storage; transparent engine identity; executor/transport integration; observation/cleanup ownership; integrated demonstration.

Unit tests are appropriate where they protect meaningful contracts and failure modes. Completion also requires running application proof and independent adversarial assessment of whether the storage seam is well designed, completely implemented, and whether any application path circumvents it. Report findings and limits honestly; a desired attestation is not a prescribed review verdict.

Implementation baseline is Gimbal c5c9b142. Scope is durable recording and directory reopen; workflow replay is subsequent work. Runtime changes belong in this Gimbal worktree, alongside this task's Gimbal View worktree.
