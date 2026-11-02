//go:build !darwin && !linux

package processdebug

import "errors"

func selectedDaemon() (int, error) { return 0, errors.New("unsupported platform") }
