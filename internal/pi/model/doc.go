// Package model holds the shared Pi values and contracts: messages and
// content blocks, usage, model catalog entries, tools, streaming events,
// agent runtime contracts and the durable history entry declarations.
//
// It is a semantic port of the TypeScript modules packages/ai/src/types.ts,
// packages/ai/src/utils/transcript.ts, packages/ai/src/utils/text.ts,
// packages/ai/src/utils/uuid.ts, packages/agent/src/types.ts,
// packages/coding-agent/src/core/messages.ts and the type declarations of
// packages/coding-agent/src/core/session-manager.ts at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31.
//
// The package holds declarations and pure codecs only. The agent loop lives in
// internal/pi/agent, the store in internal/pi/history, streaming in
// internal/pi/wire.
//
// Portions are translated from the MIT-licensed Go port sky-valley/pi
// (github.com/sky-valley/pi, MIT, Sky Valley 2026), itself a port of Pi
// (MIT, Mario Zechner 2025).
package model
