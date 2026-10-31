package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
)

func runHost(command *cobra.Command, config workerConfig) error {
	if !config.service {
		ctx, cancel := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return runWorker(ctx, config, command.OutOrStdout(), func() {})
	}
	handler := &workerService{result: make(chan error, 1), run: func(ctx context.Context, ready func()) error {
		return runWorker(ctx, config, io.Discard, ready)
	}}
	// Enter the dispatcher before environment IO, state recovery or listener setup.
	if err := svc.Run(config.serviceName, handler); err != nil {
		return fmt.Errorf("Windows SCM dispatcher: %w", err)
	}
	return <-handler.result
}

type workerService struct {
	run    func(context.Context, func()) error
	result chan error
}

func (h *workerService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	controls := make(chan serviceControl)
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		defer close(controls)
		for {
			select {
			case <-finished:
				return
			case request, ok := <-requests:
				if !ok {
					return
				}
				var control serviceControl
				switch request.Cmd {
				case svc.Stop:
					control = serviceStop
				case svc.Shutdown:
					control = serviceShutdown
				case svc.Interrogate:
					control = serviceInterrogate
				default:
					continue
				}
				select {
				case controls <- control:
				case <-finished:
					return
				}
			}
		}
	}()
	err := serviceLoop(h.run, controls, func(state serviceState) {
		var status svc.Status
		switch state {
		case serviceStarting:
			status = svc.Status{State: svc.StartPending, CheckPoint: 1, WaitHint: 10000}
		case serviceRunning:
			status = svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
		case serviceStopping:
			status = svc.Status{State: svc.StopPending, CheckPoint: 1, WaitHint: 10000}
		}
		changes <- status
	})
	// x/sys/svc sends Stopped with these exit codes after Execute returns.
	if h.result != nil {
		h.result <- err
	}
	if err != nil {
		return true, 1
	}
	return false, 0
}

func userEnvironment(name string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", errors.New("cannot open service account user environment")
	}
	defer key.Close()
	value, kind, err := key.GetStringValue(name)
	// Imports are literal strings. Avoid order-dependent expansion of registry
	// references (PATH/profile locations should be explicit in the file).
	if err != nil || kind != registry.SZ {
		return "", errors.New("user environment import requires an existing REG_SZ value")
	}
	return value, nil
}
