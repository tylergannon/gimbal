# Product validation

`gimble run validate-product --suite-file /absolute/path/to/suite.yaml`
reads JSON or YAML. The adjacent Gimble suite is a small starting example, not a
complete feature inventory. Create its disposable target directory before running;
use a different observer directory with `--work-dir`.

Install `playwright-cli` and its browser, GoTTY (for CLI cases), and ffmpeg first.
Set their executable paths in `tools` when they are not on PATH. Agent roles
`product-operation` and `product-validation` default to `gpt-5.6-luna`; the matching
CLI flags override them. Read `gimble run validate-product --help` for the contract.

All project details belong in the suite. `product.prepare` optionally builds or
fetches a desired revision; `product.start` launches a foreground service and needs
`product.ready` to check readiness. Omit `start` to use an existing target. The
optional `revision` field is a caller-supplied label. `cli` names an executable,
not a command with arguments. Relative paths resolve from the suite file.

Each feature supplies an ID, `browser` or `cli` surface, optional setup, actions to
exercise, and expected behavior. CLI interaction is filmed through a local GoTTY
terminal. Reports, supporting evidence, and per-feature WebM recordings are saved
under a unique directory in `output_dir`. A failed or blocked feature makes the
run unsuccessful. Cancellation attempts bounded recording finalization and cleanup;
hard process termination cannot guarantee either. Schedule invocations externally
for daily runs.
