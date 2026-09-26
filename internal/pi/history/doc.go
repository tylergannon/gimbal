// Package history is Pi's durable session history: append-only JSONL session
// files whose entries form a tree, plus the projection of a leaf path into the
// model context.
//
// It is a semantic port of packages/coding-agent/src/core/session-manager.ts,
// session-cwd.ts, session-export.ts and messages.ts at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31. Entry and message declarations
// come from internal/pi/model. Cancellation is context.Context in place of
// AbortSignal.
//
// This package supports the current v3 session format only. Historical
// migrations and extension execution are out of scope: a session whose header
// declares any other version is rejected rather than silently upgraded.
//
// Storage is Gimbal-owned: callers pass the session directory (typically under
// a Gimbal state root) explicitly. Nothing here reads ~/.pi.
//
// Portions are translated from the MIT-licensed Go port sky-valley/pi
// (github.com/sky-valley/pi, MIT, Sky Valley 2026), itself a port of Pi
// (MIT, Mario Zechner 2025).
package history
