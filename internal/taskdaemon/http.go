package taskdaemon

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tylergannon/gimbal/internal/effects"
)

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), 400)
		return v, false
	}
	return v, true
}
func (d *Daemon) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /tasks/session", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Session](w, r)
		if !ok {
			return
		}
		id, err := d.create(context.WithoutCancel(r.Context()), v)
		reply(w, id, false, err)
	})
	m.HandleFunc("POST /tasks/turn", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Turn](w, r)
		if !ok {
			return
		}
		reply(w, "", false, d.turn(v))
	})
	m.HandleFunc("POST /tasks/process", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Process](w, r)
		if !ok {
			return
		}
		reply(w, "", false, d.process(v))
	})
	m.HandleFunc("POST /tasks/poll", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Poll](w, r)
		if !ok {
			return
		}
		_ = json.NewEncoder(w).Encode(d.poll(v))
	})
	m.HandleFunc("POST /tasks/interrupt", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Target](w, r)
		if !ok {
			return
		}
		reply(w, "", false, d.interrupt(v))
	})
	m.HandleFunc("POST /tasks/steer", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Target](w, r)
		if !ok {
			return
		}
		value, err := d.steer(r.Context(), v)
		reply(w, "", value, err)
	})
	m.HandleFunc("POST /tasks/close", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Target](w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		reply(w, "", false, d.closeSession(ctx, v))
	})
	m.HandleFunc("POST /tasks/cleanup", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Scope](w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		reply(w, "", false, d.cleanup(ctx, v))
	})
	m.HandleFunc("POST /tasks/state", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[effects.Scope](w, r)
		if !ok {
			return
		}
		_ = json.NewEncoder(w).Encode(d.state(v))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "task requests require application/json from a non-browser client", http.StatusForbidden)
			return
		}
		m.ServeHTTP(w, r)
	})
}
func reply(w http.ResponseWriter, id string, value bool, err error) {
	p := effects.Reply{ID: id, Value: value}
	if err != nil {
		p.Error = err.Error()
	}
	_ = json.NewEncoder(w).Encode(p)
}
