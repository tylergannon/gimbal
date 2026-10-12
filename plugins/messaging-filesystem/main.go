// Build with go build -buildmode=plugin -o filesystem.so ./plugins/messaging-filesystem.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
)

type config struct{ Directory string }
type transport struct {
	*messagefiles.Store
	requests string
}

func parse(raw json.RawMessage) (config, error) {
	var c config
	err := json.Unmarshal(raw, &c)
	if err == nil && !filepath.IsAbs(c.Directory) {
		err = fmt.Errorf("directory must be absolute")
	}
	return c, err
}
func connect(ctx context.Context, raw json.RawMessage, _ backend.Dependencies) (backend.Messaging, error) {
	c, err := parse(raw)
	if err != nil {
		return nil, err
	}
	store, err := messagefiles.Open(filepath.Join(c.Directory, "events"))
	if err != nil {
		return nil, err
	}
	requests := filepath.Join(c.Directory, "requests")
	if err = os.MkdirAll(requests, 0700); err != nil {
		return nil, err
	}
	return &transport{store, requests}, nil
}
func (t *transport) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := ulid.ParseStrict(r.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(t.requests, ".request-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	err = os.Link(f.Name(), filepath.Join(t.requests, r.ID+".json"))
	if os.IsExist(err) {
		return nil
	}
	return err
}
func worker(ctx context.Context, raw json.RawMessage, resources backend.Dependencies, handle backend.MessageHandler) error {
	bus, err := connect(ctx, raw, resources)
	if err != nil {
		return err
	}
	t := bus.(*transport)
	// One explicitly owned process per directory. No automatic crash replay.
	lock := filepath.Join(t.requests, "worker.lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("worker directory already owned (remove stale worker.lock only after checking stopped worker): %w", err)
	}
	_ = f.Close()
	defer func() { _ = os.Remove(lock) }()
	if resources.Ready != nil {
		if err := resources.Ready(); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for {
		entries, err := os.ReadDir(t.requests)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || seen[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true
			raw, err := os.ReadFile(filepath.Join(t.requests, entry.Name()))
			if err != nil {
				log.Printf("request file %s unreadable; retained without retry: %v", entry.Name(), err)
				continue
			}
			var r backend.MessageRequest
			if err = json.Unmarshal(raw, &r); err != nil {
				log.Printf("request file %s invalid; retained without retry: %v", entry.Name(), err)
				continue
			}
			tasks.Go(func() {
				if err := handle(ctx, r); err != nil {
					// Do not invent a terminal execution failure: native work
					// may already exist when its event publication fails.
					log.Printf("request %s in %s handling or event delivery failed; inspect retained request, execution may be unresolved: %v", r.ID, entry.Name(), err)
				}
			})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func Register(r *gimbal.Registry) error {
	return r.RegisterMessaging(gimbal.MessagingDefinition{Name: "filesystem", ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err }, Connect: connect, Worker: worker})
}
