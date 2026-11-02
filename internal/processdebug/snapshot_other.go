//go:build !darwin && !linux

package processdebug

import "errors"

func snapshotSystem() (map[int]process, error) { return nil, errors.New("unsupported platform") }
