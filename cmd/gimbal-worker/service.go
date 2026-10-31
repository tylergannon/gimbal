package main

import (
	"context"
	"errors"
)

type serviceControl int

const (
	serviceStop serviceControl = iota
	serviceShutdown
	serviceInterrogate
)

type serviceState int

const (
	serviceStarting serviceState = iota
	serviceRunning
	serviceStopping
)

// serviceLoop is shared with the SCM adapter so readiness, failures and controls
// can also be exercised without a Windows service installation.
func serviceLoop(run func(context.Context, func()) error, controls <-chan serviceControl, report func(serviceState)) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state := serviceStarting
	report(state)
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, func() {
			select {
			case ready <- struct{}{}:
			case <-ctx.Done():
			}
		})
	}()
	stop := func() {
		if state != serviceStopping {
			state = serviceStopping
			report(state)
			cancel()
		}
	}
	for {
		select {
		case <-ready:
			ready = nil
			if state == serviceStarting {
				state = serviceRunning
				report(state)
			}
		case control, ok := <-controls:
			if !ok {
				controls = nil
				stop()
				continue
			}
			switch control {
			case serviceStop, serviceShutdown:
				stop()
			case serviceInterrogate:
				report(state)
			}
		case err := <-done:
			stopping := state == serviceStopping
			stop()
			if stopping && (err == nil || errors.Is(err, context.Canceled)) {
				return nil
			}
			if err == nil {
				return errors.New("task worker exited unexpectedly")
			}
			return err
		}
	}
}
