package opencode

// types.json contains the exact 60-component reference closure selected from
// OpenCode 1.18.31's pinned OpenAPI. HTTP calls and their small inline request
// and response envelopes stay handwritten.
//go:generate go tool oapi-codegen --config types.cfg.yaml types.json
