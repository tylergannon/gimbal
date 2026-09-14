# #154: Attest program for issue 149 deletes the previous run's logs on start

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/154. Read beta-implementation-handoff.md and beta-bug-triage.md for final session decisions and coordination notes.

`ephemeral/attest/issue-149/main.go` does `os.RemoveAll` on its logs directory before each run, so a validator's rerun removes the developer's committed run directory from the working tree. Use a run-scoped directory (or refuse to start when the directory is non-empty, as `run-prompt --logs` does) so successive proof runs accumulate instead of replacing each other.
