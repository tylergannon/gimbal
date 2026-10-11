package workspace

import (
	"context"
	"time"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/gimbal/internal/transcript"
	"github.com/tylergannon/skgo"
)

func store(ctx context.Context, id string) (*transcript.DurableStore, error) {
	owner := host.OwnerFrom(ctx)
	if owner != nil {
		for _, p := range owner.Projects() {
			if p.ID() == id {
				return p.Recordings(), nil
			}
		}
	}
	return nil, skgo.Errorf(404, "Project not found")
}
func openSession(ctx context.Context, event app.RequestEvent[params.Params], r transcript.OpenRequest) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.Open(r)
}
func acceptTranscript(ctx context.Context, event app.RequestEvent[params.Params], r transcript.Update) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.Accept(r)
}
func activateSession(ctx context.Context, event app.RequestEvent[params.Params], r transcript.SessionRequest) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.State(r.SessionID, "recording", nil)
}
func closeSession(ctx context.Context, event app.RequestEvent[params.Params], r transcript.CloseRequest) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.State(r.SessionID, "stopped", &r.Sequence)
}
func abortSession(ctx context.Context, event app.RequestEvent[params.Params], r transcript.SessionRequest) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.State(r.SessionID, "interrupted", nil)
}

type StateRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionID"`
	Status    string `json:"status"`
}

func touchSession(ctx context.Context, event app.RequestEvent[params.Params], r StateRequest) (transcript.Ack, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.Ack{}, e
	}
	return s.State(r.SessionID, r.Status, nil)
}

type RecordingRequest struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}
type HistoryRequest struct {
	Project string `json:"project"`
	After   string `json:"after"`
}

func recordingHistory(ctx context.Context, r HistoryRequest) (transcript.History, error) {
	s, e := store(ctx, r.Project)
	if e != nil {
		return transcript.History{}, e
	}
	return s.List(r.After), nil
}
func watchRecording(ctx context.Context, r RecordingRequest, yield func(transcript.Recording) error) error {
	s, e := store(ctx, r.Project)
	if e != nil {
		return e
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		v, e := s.Get(r.ID)
		if e != nil {
			return e
		}
		if e = yield(v); e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

var _ = skgo.Command(openSession)
var _ = skgo.Command(acceptTranscript)
var _ = skgo.Command(activateSession)
var _ = skgo.Command(closeSession)
var _ = skgo.Command(abortSession)
var _ = skgo.Command(touchSession)
var _ = skgo.Query(recordingHistory)
var _ = skgo.LiveQuery(watchRecording)
