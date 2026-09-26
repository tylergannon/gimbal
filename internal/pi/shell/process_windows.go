//go:build windows

package shell

import (
	"os"
	"os/exec"
)

// The port targets macOS/Linux; these stubs keep the package buildable on
// Windows without pretending to implement pi's taskkill-based tree cleanup.

func setProcessGroup(*exec.Cmd) {}

// KillProcessTree is a no-op on Windows in this port.
func KillProcessTree(int) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func exitSignalNumber(*os.ProcessState) (int, bool) { return 0, false }
