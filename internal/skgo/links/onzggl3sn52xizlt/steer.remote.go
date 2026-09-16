package routes

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/live"
)

// steerTimeout bounds one delivery. The adapters bound their own control
// calls more tightly than this; the point of the outer bound is that a
// session which has gone unreachable cannot hold a browser's form open.
const steerTimeout = 30 * time.Second

// Steer is what the run page's form posts: which session of which run the
// person is watching, and what they typed at it.
type Steer struct {
	Run     string `json:"run"`
	Session string `json:"session"`
	Message string `json:"message"`
}

// Sent is the form's answer. Landed is false when the message reached the
// session but no turn was running to receive it, which is not an error: a
// steer sent to an idle session is dropped, and the person is told so.
type Sent struct {
	Landed bool `json:"landed"`
}

// steer sends one message into a running session, as the person watching the
// page. It is the page's half of #176: the run log records the message with
// Source "person" exactly as Runtime.Steer does, because this reaches the
// same run through the same table.
func steer(ctx context.Context, arg Steer) (Sent, error) {
	message := strings.TrimSpace(arg.Message)
	if message == "" {
		return Sent{}, skgo.Invalidf("message", "Type a message before sending it.")
	}
	runs := live.RunsFrom(ctx)
	if runs == nil {
		return Sent{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no table of runs in its context, so no session can be steered.")
	}
	run, err := runs.InProgress(arg.Run)
	if err != nil {
		return Sent{}, skgo.Errorf(http.StatusNotFound,
			"Run %s is not in progress, so there is nothing to steer.", arg.Run)
	}
	// The browser cannot cancel the delivery. A person who submits and then
	// navigates away has already sent the message, and cancelling here would
	// cut it off inside the adapter's control call instead.
	deliver, cancel := context.WithTimeout(context.WithoutCancel(ctx), steerTimeout)
	defer cancel()
	landed, err := run.Steer(deliver, arg.Session, message)
	if err != nil {
		return Sent{}, skgo.Errorf(http.StatusNotFound,
			"Session %s is not running in %s, so there is nothing to steer.", arg.Session, arg.Run)
	}
	return Sent{Landed: landed}, nil
}

var _ = skgo.Form(steer)
