# Approval brief: native Mac control feasibility

**Approved by Tyler, November 1, 2026.** No installation or native experiment has
run at the time of approval. Amrita runtime use was coordinated with A; no release
authority is conferred. Tyler subsequently accepted Escape cancellation in place
of the unavailable literal Cancel control, with all other native/lock checks kept.

Goal: Gimbal initiates native control on Amrita while Locked Use remains enabled,
without manual unlock. A Desktop-dependent worker is acceptable. One Desktop
instance is reused.

The read-only [socket audit](socket-audit.md) found client connection overrides,
a separate coordination socket, and the historical authenticated app-tools route.
It did not establish a direct Gimbal endpoint into Desktop's native-capable
runtime. Coverage and unknowns are explicit; this is not proof of impossibility.

Approved authorization, governed by the [spec and definition of done](spec.md):

1. At most 20 minutes to verify the genuine Desktop bootstrap, install only the
   already-staged scoped worker, verify real discovery, saved Finder approval,
   task read/stop, and admission after the owner's binding turn ends. One fresh
   execution-owner chat is permitted if resumed discovery fails; no new coordinator
   or app-server. Any missing prerequisite ends the attempt.
2. At most 20 minutes for an actual Gimbal-originated locked Finder trial:
   accessibility and screenshot, type into Go to Folder without navigating,
   dismiss with Escape, and verify normal protected unlock/relock.
3. At most 20 minutes for a new execution task repeating the same behavior and
   reconciling owned resources. No manual unlock or security-setting changes.

An early failure can be inconclusive about the interface; it is not permission
to build around it. This gate tests the existing worker only. Substantial product
changes need a concrete post-gate contract amendment; a working operational
configuration may need no code changes.

Discovery is the likeliest early stop. A stale binding turn, failed re-admission
while locked, unsupported resolved Luna model, or unreadable/missing saved Finder
approval also ends admission before the native trial.

Terra 6.1 owns the approved experiment/implementation work; Sonnet independently
validates. Sol supervises here. Research/review notes are retained; run output
and screenshots stay out of git.

The approved scope includes scoped installation, targeted bootstrap follow-up,
one fresh worker-owner chat if necessary, and bounded Gimbal-created execution tasks.
