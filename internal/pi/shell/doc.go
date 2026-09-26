// Package shell holds pi's local shell execution and the bash tool: shell
// resolution, streaming output accumulation and truncation, timeout and
// cancellation, and the process-tree cleanup that keeps a killed command from
// leaving backgrounded grandchildren behind.
//
// It is a hand port of the following upstream modules at pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31:
//
//   - core/tools/bash.ts
//   - core/tools/output-accumulator.ts
//   - core/bash-executor.ts
//   - utils/shell.ts
//   - utils/child-process.ts
//   - utils/ansi.ts (the stripAnsi helper bash-executor.ts applies)
//
// Types come from internal/pi/model; truncation and size formatting come from
// internal/pi/files. Cancellation is context.Context in place of AbortSignal.
// PowerShell and the Windows branches of shell resolution are out of scope per
// the port plan; only macOS/Linux behavior is implemented.
//
// Portions are translated from the MIT-licensed Go port sky-valley/pi
// (github.com/sky-valley/pi, MIT, Sky Valley 2026), itself a port of Pi
// (MIT, Mario Zechner 2025).
package shell
