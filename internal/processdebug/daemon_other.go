//go:build !darwin && !linux

package processdebug

import "errors"

func selectedDaemon() (process, error) { return process{}, errors.New("unsupported platform") }
