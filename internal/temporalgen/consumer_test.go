package temporalgen_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// The only Temporal emitter lives in the separate consumer module. These tests
// keep the original paired specimen coverage against that consumer executable.
func emitSource(dir, entry, name, output string) error {
	root, err := filepath.Abs("../..")
	if err != nil {
		return err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "-C", filepath.Join(root, "examples/temporal"), "run", "./cmd/generate", "-dir", dir, "-entry", entry, "-name", name, "-output", output)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", b, err)
	}
	return nil
}
