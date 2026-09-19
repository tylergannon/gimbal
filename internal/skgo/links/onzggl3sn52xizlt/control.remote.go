package routes

import (
	"context"
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/live"
)

// StopTurn identifies the running turn an operator intends to interrupt.
type StopTurn struct {
	Run  string `json:"run"`
	Turn string `json:"turn"`
}

// CancelRun identifies the live run whose root scope an operator confirmed
// they intend to cancel.
type CancelRun struct {
	Run string `json:"run"`
}

// ControlAccepted says the live runtime accepted an operator control. The
// observation stream remains the authority for the resulting terminal state.
type ControlAccepted struct {
	Accepted bool `json:"accepted"`
}

func stopTurn(ctx context.Context, arg StopTurn) (ControlAccepted, error) {
	run, err := liveRun(ctx, arg.Run)
	if err != nil {
		return ControlAccepted{}, err
	}
	cause := gimble.Killed{Target: arg.Turn, By: "person", Reason: "stopped from the run workspace"}
	if err := run.CancelTurn(arg.Turn, cause); err != nil {
		return ControlAccepted{}, skgo.Errorf(http.StatusNotFound,
			"Turn %s is no longer running in %s.", arg.Turn, arg.Run)
	}
	return ControlAccepted{Accepted: true}, nil
}

func cancelRun(ctx context.Context, arg CancelRun) (ControlAccepted, error) {
	run, err := liveRun(ctx, arg.Run)
	if err != nil {
		return ControlAccepted{}, err
	}
	cause := gimble.Killed{Target: "", By: "person", Reason: "cancelled from the run workspace"}
	if err := run.CancelScope("", cause); err != nil {
		return ControlAccepted{}, skgo.Errorf(http.StatusNotFound,
			"Run %s is no longer running.", arg.Run)
	}
	return ControlAccepted{Accepted: true}, nil
}

func liveRun(ctx context.Context, id string) (live.Controller, error) {
	runs := live.RunsFrom(ctx)
	if runs == nil {
		return nil, skgo.Errorf(http.StatusInternalServerError,
			"This server has no table of runs in its context, so the control cannot be delivered.")
	}
	run, err := runs.InProgress(id)
	if err != nil {
		return nil, skgo.Errorf(http.StatusNotFound,
			"Run %s is not in progress.", id)
	}
	return run, nil
}

var _ = skgo.Command(stopTurn)

var _ = skgo.Command(cancelRun)
