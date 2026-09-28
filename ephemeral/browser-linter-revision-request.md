Revise only internal/gimballint's newly implemented browser rule. You are not
alone; preserve others' edits. Do not commit, invoke Gimbal, or launch agents.
Read the current browser design and repository instructions.

The manager accepts the ownership rule but rejects the added rule that every
generic parameter erases the browser type. A generic identity/helper instantiated
with *gimbal.Browser has a typed parameter and result; the design permits typed
helper calls. In checkCall, inspect the instantiated parameter type and diagnose
actual interface conversions, without treating fn.Origin TypeParam as one.
Remove the extra generic restriction from rules.md. Update the corresponding
test: normal generic identity/use must pass, while a call explicitly instantiated
with any and supplied a browser must still fail. Preserve existing ownership
tracking at the typed result and the documented local-analysis limits.

Run focused linter tests and report the change and results. Do not broaden the
analysis or remove any other accepted escape checks.
