// Package readtools ports pi's built-in read, grep, find and ls tools.
//
// It is a hand port of the following upstream modules (pin d6af72e1):
//
//   - core/tools/read.ts
//   - core/tools/grep.ts
//   - core/tools/find.ts
//   - core/tools/ls.ts
//
// Along with the input-mime sniffer those tools depend on (utils/mime.ts).
// Each tool returns a model.ToolDefinition; the pluggable per-tool operations
// seams are exported so a host can delegate file access elsewhere (SSH, a
// remote sandbox), matching pi's ReadOperations/GrepOperations/FindOperations/
// LsOperations interfaces.
//
// The port searches in pure Go rather than shelling out to ripgrep and fd, so
// it implements gitignore traversal and glob matching itself (glob.go). That
// keeps the tools free of an installed-binary requirement; it is a deliberate
// architectural divergence from upstream, not a behavioral one.
package readtools
