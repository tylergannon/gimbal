Implement only the browser handle lifetime linter described by the accepted
current ephemeral/browser-implementation-design.md and authorized by
ephemeral/browser-evaluator-build.md. Read repository instructions. You are
not alone; preserve others' edits and accommodate their work.

Own internal/gimballint only: analyzer, meaningful tests/testdata, its gimbal
test stub and rules.md. Browser core, generator, backend and evaluator belong
to other workers. Track browser values by type and the supported ownership
rules in the design, including helper-returned values and groups created
outside the owner. Reject supported lifetime escapes while allowing normal
in-scope WithBrowser and safe nested scopes. Do not invent whole-program
analysis or promise unsupported alias/control-flow forms.

Run focused linter tests. Do not commit, launch agents, invoke Gimbal, or edit
outside your assigned package. Report changed files, actual tests and any
remaining contract question. No additional public API is authorized here.
