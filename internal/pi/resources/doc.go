// Package resources is the headless resource layer for the Pi port: project
// instructions, skills, prompt templates and system-prompt assembly, plus the
// project-trust decision that gates them.
//
// It is a Go translation of these TypeScript modules at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31:
//
//   - packages/coding-agent/src/core/resource-loader.ts
//   - packages/coding-agent/src/core/skills.ts
//   - packages/coding-agent/src/core/prompt-templates.ts
//   - packages/coding-agent/src/core/system-prompt.ts
//   - packages/coding-agent/src/core/trust-manager.ts
//   - packages/coding-agent/src/core/project-trust.ts
//   - packages/coding-agent/src/core/source-info.ts
//   - packages/coding-agent/src/core/package-manager.ts (resolve and
//     resolveExtensionSources only; install/update/remove are out of scope)
//   - packages/coding-agent/src/utils/frontmatter.ts
//
// The port is headless: it does not execute TypeScript extensions, load
// terminal themes, or install, update or remove packages. Skills and prompt
// templates are discovered through the resolver, which honors settings
// entries, automatic discovery under the agent and project directories, and
// already-installed package paths. Project resources are gated by project
// trust.
package resources
