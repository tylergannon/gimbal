// Package web embeds the SvelteKit build the skgo adapter writes.
package web

import "embed"

// Build holds the adapter output rooted at build/.
// Run just build before compiling the CLI to include the current frontend.
//
//go:embed all:build
var Build embed.FS
