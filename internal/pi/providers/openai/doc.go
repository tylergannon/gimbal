// Package openai implements the OpenAI Chat Completions API surface used by
// Gimbal's Diffusion Router: request building, streaming, tool calls, reasoning
// replay and usage accounting.
//
// It is a semantic port of pi's packages/ai/src/api/openai-completions.ts at
// upstream pin d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31. Simple-options and
// transcript-message helpers live in internal/pi/wire; request and response
// types come from internal/pi/model. Cancellation is context.Context in place
// of an AbortSignal.
//
// The provider receives an explicit, already-resolved *model.Model, including
// its model.Compat blob. It never consults a config package or a global
// registry.
//
// Portions are translated from the MIT-licensed Go port sky-valley/pi
// (github.com/sky-valley/pi, MIT, Sky Valley 2026), itself a port of Pi
// (MIT, Mario Zechner 2025).
package openai
