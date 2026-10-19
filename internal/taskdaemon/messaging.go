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
		if err := r.Validate(); err != nil {
			return err
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
			if err == nil && (result.Process == nil || result.Process.Error == "" && result.Process.ExitCode == 0) {
				var requirements backend.Requirements
				if r.Turn != nil {
					requirements = r.Turn.Requires
				}
				if r.Process != nil {
					requirements = r.Process.Requires
				}
				for _, requirement := range requirements {
					result.ReadyResources = append(result.ReadyResources, backend.ResourceFact{Scope: backend.Scope{Run: r.Run, Scope: r.Scope}, Name: requirement.Store, Kind: "store", Consumer: r.Task})
				}
				if r.Process != nil {
					for _, name := range r.Process.Yields {
						result.ReadyResources = append(result.ReadyResources, backend.ResourceFact{Scope: backend.Scope{Run: r.Run, Scope: r.Scope}, Name: name, Kind: "publication", Consumer: r.Task})
					}
				}
			}
			if err != nil {
				result.Error = err.Error()
				result.Interrupted = r.Kind == "identified-turn" && errors.Is(err, context.Canceled)
				result.Unresolved = r.Kind == "cleanup" || r.Kind == "interrupt" || r.Kind == "identified-close"
				result.InvalidOutput = errors.Is(err, gimbal.ErrInvalidOutput)
			}
			return publish("completed", &result, nil)
		}
		var err error
		switch r.Kind {
		case "identified-session":
			if r.Session == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("session payload required"))
			}
			id, err := d.create(ctx, *r.Session)
			return complete(backend.MessageResult{ID: id}, err)
		case "identified-turn":
			if r.Turn == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("turn payload required"))
			}
			err = d.turnEvents(*r.Turn, func(raw json.RawMessage) {
				if e := publish("activity", nil, raw); e != nil {
					log.Printf("request %s activity delivery: %v", r.ID, e)
				}
			})
		case "identified-process":
			if r.Process == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("process payload required"))
			}
			err = d.process(*r.Process)
			if err == nil && r.Process.Service {
				err = publish("established", nil, nil)
			}
		case "identified-close":
			if r.Target == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("close target required"))
			}
			return complete(backend.MessageResult{}, d.closeSession(ctx, *r.Target))
		case "cleanup":
			if r.ScopeTarget == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("scope required"))
			}
			stop, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			return complete(backend.MessageResult{}, d.cleanupNative(stop, *r.ScopeTarget))
		case "interrupt":
			if r.Target == nil {
				return complete(backend.MessageResult{}, fmt.Errorf("interrupt target required"))
			}
			d.mu.Lock()
			t := d.tasks[key(r.Target.Run, r.Target.ID)]
			if t == nil {
				t = d.admitting[key(r.Target.Run, r.Target.ID)]
			}
			// Selection and cancellation share native admission's owner lock. A
			// preflight cannot become separately owned native work between them.
			if t != nil {
				t.cancel()
			}
			d.mu.Unlock()
			if t != nil {
				stop, cancel := context.WithTimeout(ctx, 30*time.Second)
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
			if r.Kind == "identified-turn" {
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
