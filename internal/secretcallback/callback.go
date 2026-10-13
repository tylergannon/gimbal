// Package secretcallback exposes operation-bound values to native agents.
package secretcallback

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

// The callback is private to this native operation. Its token and returned
// values never become workflow message or observation fields.
func Open(ctx context.Context, secrets backend.SecretsBackend, owner backend.Owner, requires backend.Requirements) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", func() {}, err
	}
	path := "/secrets/" + ulid.Make().String()
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		if ctx.Err() != nil || r.Header.Get("Origin") != "" {
			http.Error(w, "operation unavailable", http.StatusForbidden)
			return
		}
		values, err := secrets.Acquire(r.Context(), backend.Scope{Run: owner.Run, Scope: owner.Scope}, requires)
		if err != nil {
			http.Error(w, "required secrets unavailable", http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(values)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String() + path, func() { _ = server.Close() }, nil
}
