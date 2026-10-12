// Package main is the runtime-loaded River/Postgres messaging plugin.
// Build with go build -buildmode=plugin -o river.so ./plugins/messaging-river.
// Configuration names a private DSNFile and explicitly selected action/control
// queues. Start one local worker for those queues; events last until database
// removal. Native execution is not resumed after worker loss.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

type configuration struct {
	DSNFile      string
	Queue        string
	ControlQueue string
	Workers      int
}

func parse(raw json.RawMessage) (configuration, error) {
	var c configuration
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.DSNFile == "" {
		return c, errors.New("river messaging requires DSNFile pointing to a private PostgreSQL connection string")
	}
	if c.Queue == "" {
		c.Queue = "gimbal"
	}
	if c.ControlQueue == "" {
		c.ControlQueue = c.Queue + "_control"
	}
	if c.Queue == c.ControlQueue {
		return c, errors.New("river action and control queues must differ")
	}
	if c.Workers == 0 {
		c.Workers = 4
	}
	if c.Workers < 1 {
		return c, errors.New("river Workers must be positive")
	}
	return c, nil
}

type actionArgs struct {
	Request backend.MessageRequest `json:"request"`
}

func (actionArgs) Kind() string { return "gimbal_action" }

type transport struct {
	config configuration
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
}

func open(ctx context.Context, c configuration) (*transport, error) {
	dsn, err := os.ReadFile(c.DSNFile)
	if err != nil {
		return nil, fmt.Errorf("river DSN file: %w", err)
	}
	// pgx parse errors can contain the complete input, including a password.
	pool, err := pgxpool.New(ctx, strings.TrimSpace(string(dsn)))
	if err != nil {
		return nil, errors.New("river DSN file contains an invalid PostgreSQL connection string")
	}
	ok := false
	defer func() {
		if !ok {
			pool.Close()
		}
	}()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("river PostgreSQL connection unavailable: %w", err)
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return nil, err
	}
	ok = true
	return &transport{config: c, pool: pool, client: client}, nil
}

func (t *transport) initialize(ctx context.Context) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(t.pool), &rivermigrate.Config{Logger: slog.Default()})
	if err != nil {
		return err
	}
	lock, err := pgx.ConnectConfig(ctx, t.pool.Config().ConnConfig)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close(context.Background()) }()
	// Only explicitly started workers initialize schema; callers never take this lock.
	if _, err = lock.Exec(ctx, "SELECT pg_advisory_lock(745289004)"); err != nil {
		return err
	}
	if _, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	if _, err = t.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gimbal_message_events (
 queue text NOT NULL, id text NOT NULL, created timestamptz NOT NULL,
 request_id text NOT NULL, run_id text NOT NULL, session_id text NOT NULL,
 event bytea NOT NULL, PRIMARY KEY (queue, id))`); err != nil {
		return err
	}
	return nil
}

// Close releases this connection's pool. Request and listener contexts control
// operations; cancelling the Connect establishment budget does not close it.
func (t *transport) Close() error { t.pool.Close(); return nil }

func storeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("river messaging schema unavailable; start the configured message-worker to initialize PostgreSQL: %w", err)
	}
	return err
}

func (t *transport) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	queue := t.config.Queue
	switch r.Kind {
	case "interrupt", "close", "cleanup":
		queue = t.config.ControlQueue
	}
	// Native work is admission-only. Neither normal errors nor crash rescue may
	// replay it: a crash leaves the native outcome unknown.
	_, err := t.client.Insert(ctx, actionArgs{Request: r}, &river.InsertOpts{Queue: queue, MaxAttempts: 1})
	return storeError(err)
}

func (t *transport) PublishEvent(ctx context.Context, e backend.MessageEvent) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = t.pool.Exec(ctx, `INSERT INTO gimbal_message_events
 (queue,id,created,request_id,run_id,session_id,event) VALUES ($1,$2,$3,$4,$5,$6,$7)
 ON CONFLICT (queue,id) DO NOTHING`, t.config.Queue, e.ID, e.Created, e.RequestID, e.Run, e.SessionID, b)
	return storeError(err)
}

func (t *transport) ListenEvents(ctx context.Context, s backend.MessageSelection, since time.Time, receive func(backend.MessageEvent) error) error {
	seen := map[string]bool{}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		rows, err := t.pool.Query(ctx, `SELECT id FROM gimbal_message_events
            WHERE queue=$1 AND created >= $2 AND ($3='' OR request_id=$3)
            AND ($4='' OR run_id=$4) AND ($5='' OR session_id=$5)`,
			t.config.Queue, since, s.RequestID, s.Run, s.SessionID)
		if err != nil {
			return storeError(err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			if !seen[id] {
				ids = append(ids, id)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return storeError(err)
		}
		var events []backend.MessageEvent
		if len(ids) > 0 {
			rows, err = t.pool.Query(ctx, `SELECT event FROM gimbal_message_events
                WHERE queue=$1 AND id=ANY($2) ORDER BY created,id`, t.config.Queue, ids)
			if err != nil {
				return storeError(err)
			}
			for rows.Next() {
				var b []byte
				if err = rows.Scan(&b); err != nil {
					break
				}
				var event backend.MessageEvent
				if err = json.Unmarshal(b, &event); err != nil {
					break
				}
				events = append(events, event)
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return storeError(err)
			}
		}
		// Release connections before callbacks so slow listeners cannot hold
		// the database resources needed by publishers or other listeners.
		for _, event := range events {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := receive(event); err != nil {
				return err
			}
			seen[event.ID] = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type actionWorker struct {
	river.WorkerDefaults[actionArgs]
	ctx     context.Context
	handler backend.MessageHandler
	mu      sync.Mutex
	seen    map[string]bool
}

func (w *actionWorker) Work(_ context.Context, job *river.Job[actionArgs]) error {
	w.mu.Lock()
	duplicate := w.seen[job.Args.Request.ID]
	w.seen[job.Args.Request.ID] = true
	w.mu.Unlock()
	if duplicate {
		return nil
	}
	if err := w.handler(w.ctx, job.Args.Request); err != nil {
		// Do not wrap the underlying context error: River's shutdown path checks
		// errors.Is(context.Canceled) before cancellation and requeues that job.
		return river.JobCancel(fmt.Errorf("admission %s unconfirmed: %v", job.Args.Request.ID, err))
	}
	return nil
}

func serve(ctx context.Context, raw json.RawMessage, handler backend.MessageHandler) error {
	c, err := parse(raw)
	if err != nil {
		return err
	}
	t, err := open(ctx, c)
	if err != nil {
		return err
	}
	defer t.pool.Close()
	if err := t.initialize(ctx); err != nil {
		return err
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &actionWorker{ctx: ctx, handler: handler, seen: map[string]bool{}})
	client, err := river.NewClient(riverpgxv5.New(t.pool), &river.Config{
		Queues:  map[string]river.QueueConfig{c.Queue: {MaxWorkers: c.Workers}, c.ControlQueue: {MaxWorkers: 2}},
		Workers: workers,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		return err
	}
	return ctx.Err()
}

func Register(registry *gimbal.Registry) error {
	return registry.RegisterMessaging(gimbal.MessagingDefinition{
		Name:                  "river",
		ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err },
		Connect: func(ctx context.Context, raw json.RawMessage) (backend.Messaging, error) {
			c, err := parse(raw)
			if err != nil {
				return nil, err
			}
			t, err := open(ctx, c)
			if err != nil {
				return nil, err
			}
			return t, nil
		},
		Worker: serve,
	})
}
