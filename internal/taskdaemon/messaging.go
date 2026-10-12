package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

// Messages uses this daemon's existing native owner. Duplicate admission IDs
// remain known for its entire lifetime; restarting native sessions is unsupported.
func (d *Daemon) Messages(bus backend.Messaging) backend.MessageHandler {
	var mu sync.Mutex
	seen := map[string]bool{}
	return func(ctx context.Context, r backend.MessageRequest) error {
		if r.ID == "" || r.Created.IsZero() {
			return fmt.Errorf("request ID and creation time required")
		}
		mu.Lock()
		if seen[r.ID] {
			mu.Unlock()
			return nil
		}
		seen[r.ID] = true
		mu.Unlock()
		publish := func(kind string, result *backend.MessageResult, activity json.RawMessage) error {
			return bus.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, SessionID: r.SessionID, Kind: kind, Result: result, Activity: activity})
		}
		if err := publish("accepted", nil, nil); err != nil {
			return err
		}
		complete := func(result backend.MessageResult, err error) error {
			if err != nil {
				result.Error = err.Error()
				result.InvalidOutput = errors.Is(err, gimbal.ErrInvalidOutput)
			}
			return publish("completed", &result, nil)
		}
		var err error
		switch r.Kind {
		case "session":
			if r.Session == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("session payload required"))
			}
			id, err := d.create(ctx, *r.Session)
			return complete(backend.MessageResult{ID: id}, err)
		case "turn":
			if r.Turn == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("turn payload required"))
			}
			err = d.turnEvents(*r.Turn, func(raw json.RawMessage) {
				if e := publish("activity", nil, raw); e != nil {
					log.Printf("request %s activity delivery: %v", r.ID, e)
				}
			})
		case "process":
			if r.Process == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("process payload required"))
			}
			err = d.process(*r.Process)
			if err == nil && r.Process.Service {
				err = publish("established", nil, nil)
			}
		case "close":
			if r.Target == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("close target required"))
			}
			return complete(backend.MessageResult{}, d.closeSession(ctx, *r.Target))
		case "cleanup":
			if r.ScopeTarget == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("scope required"))
			}
			stop, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return complete(backend.MessageResult{}, d.cleanup(stop, *r.ScopeTarget))
		case "interrupt":
			if r.Target == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("interrupt target required"))
			}
			d.mu.Lock()
			t := d.tasks[key(r.Target.Run, r.Target.ID)]
			d.mu.Unlock()
			if t != nil {
				t.cancel()
				stop, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				if t.stop != nil {
					err = t.stop(stop)
				}
				if err == nil {
					select {
					case <-t.done:
					case <-stop.Done():
						err = fmt.Errorf("cessation unconfirmed: %w", stop.Err())
					}
				}
			}
			return complete(backend.MessageResult{}, err)
		default:
			return complete(backend.MessageResult{}, fmt.Errorf("unsupported messaging action %q", r.Kind))
		}
		if err != nil {
			return complete(backend.MessageResult{}, err)
		}
		d.mu.Lock()
		t := d.tasks[key(r.Run, r.Task)]
		d.mu.Unlock()
		if t == nil {
			return complete(backend.MessageResult{}, fmt.Errorf("native admission produced no task"))
		}
		go func() {
			select {
			case <-ctx.Done():
				log.Printf("request %s completion delivery stopped; native ownership unresolved", r.ID)
				return
			case <-t.done:
			}
			t.mu.Lock()
			raw := append([]byte(nil), t.result...)
			nativeErr := t.err
			captures := map[string]string{}
			maps.Copy(captures, t.captures)
			t.mu.Unlock()
			var result backend.MessageResult
			if r.Kind == "turn" {
				var turn backend.TurnResult
				nativeErr = errors.Join(nativeErr, json.Unmarshal(raw, &turn))
				result.Output, result.Usage = turn.Output, turn.Usage
			} else {
				result.Process = &backend.ProcessResult{}
				nativeErr = errors.Join(nativeErr, json.Unmarshal(raw, result.Process))
				var e error
				result.Stdout, e = os.ReadFile(captures["stdout"])
				nativeErr = errors.Join(nativeErr, e)
				result.Stderr, e = os.ReadFile(captures["stderr"])
				nativeErr = errors.Join(nativeErr, e)
			}
			if err := complete(result, nativeErr); err != nil {
				log.Printf("request %s completion delivery failed: %v", r.ID, err)
			}
		}()
		return nil
	}
}
