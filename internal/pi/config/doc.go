// Package config holds the headless configuration surface for the Pi port:
// settings, the models.json snapshot, the model catalog, model selection and
// API-key resolution.
//
// It is a Go translation of these TypeScript modules at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31:
//
//   - packages/coding-agent/src/core/settings-manager.ts
//   - packages/coding-agent/src/core/model-config.ts
//   - packages/coding-agent/src/core/models-store.ts
//   - packages/coding-agent/src/core/resolve-config-value.ts
//   - packages/coding-agent/src/core/runtime-credentials.ts
//   - packages/coding-agent/src/core/http-dispatcher.ts
//   - packages/coding-agent/src/core/model-resolver.ts
//
// The port is scoped to the Diffusion Router provider: there is no OAuth, no
// extension provider registration and no browser UI. The catalog loader reads
// the router's OpenAI-style model list. Paths, home directories and
// environment maps are passed explicitly wherever a caller controls them.
package config
