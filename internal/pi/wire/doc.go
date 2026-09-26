// Package wire holds the streaming and request-side helpers shared by Pi's API
// implementations: the AssistantMessageEventStream, streaming JSON repair and
// partial parsing, provider and assistant-call retry, context-overflow
// detection, transcript message transformation and context token estimation.
//
// It is a semantic port of the TypeScript modules under
// packages/ai/src/utils and packages/ai/src/api (transform-messages.ts,
// simple-options.ts) at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31. Types come from
// internal/pi/model; cancellation is context.Context in place of AbortSignal.
//
// Portions are translated from the MIT-licensed Go port sky-valley/pi
// (github.com/sky-valley/pi, MIT, Sky Valley 2026), itself a port of Pi
// (MIT, Mario Zechner 2025).
package wire
