//go:build !darwin && !linux

package gimbal

import (
	"errors"
	"os/exec"
)

func prepareServiceProcess(*exec.Cmd) error {
	return errors.New("scope-owned services require darwin or linux process groups")
}

func terminateServiceGroup(int) error { return nil }
func killServiceGroup(int) error      { return nil }
func serviceGroupAlive(int) (bool, error) {
	return false, nil
}
