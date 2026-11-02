//go:build !darwin && !linux

package processdebug

import "errors"

func snapshotSystem() (map[int]process, error) { return nil, errors.New("unsupported platform") }

func initialIdentity(int) (process, error) { return process{}, errors.New("unsupported platform") }
