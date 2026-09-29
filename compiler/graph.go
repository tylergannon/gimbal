package compiler

import "github.com/tylergannon/gimbal/internal/generate"

// GenerateGraph writes graph registration source in the authored package. output
// is relative to dir. Regeneration replaces only this generator's own output.
// Import the authored package in the serving binary to register the graph.
func GenerateGraph(dir, entry, name, output string) error {
	return generate.GenerateGraph(dir, entry, name, output)
}
