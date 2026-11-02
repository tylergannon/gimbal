// Package main is the runtime-loaded River/Postgres messaging plugin.
// Build with go build -buildmode=plugin -o river.so ./plugins/messaging-river.
// Configuration names a private DSNFile and explicitly selected action/control
// queues. Start one local worker for those queues; events last until database
// removal. Native execution is not resumed after worker loss.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	DSNFile          string
	Queue            string
	ControlQueue     string
	Workers          int
	Environments     map[string]string
	WorkerID         string
	ReconnectSeconds int
}

func parse(raw json.RawMessage) (configuration, error) {
	var c configuration
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c, errors.New("river configuration must contain one JSON object")
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
	if c.ReconnectSeconds == 0 {
		c.ReconnectSeconds = 30
	}
	if c.ReconnectSeconds < 0 {
		return c, errors.New("river ReconnectSeconds must be positive")
	}
	for name, worker := range c.Environments {
		if strings.TrimSpace(name) != name || name == "" || strings.TrimSpace(worker) != worker || worker == "" {
			return c, errors.New("river environment names and worker identities must be nonempty and have no surrounding whitespace")
		}
	}
	if strings.TrimSpace(c.WorkerID) != c.WorkerID {
		return c, errors.New("river WorkerID must have no surrounding whitespace")
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

func (t *transport) NamedEnvironments() bool { return len(t.config.Environments) != 0 }

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
	if _, err = t.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gimbal_message_owners (
 namespace text NOT NULL, run_id text NOT NULL, kind text NOT NULL, identity text NOT NULL,
 worker text NOT NULL, environment text NOT NULL, scope text NOT NULL DEFAULT '',
 PRIMARY KEY (namespace, run_id, kind, identity))`); err != nil {
		return err
	}
	if _, err = t.pool.Exec(ctx, `ALTER TABLE gimbal_message_owners ADD COLUMN IF NOT EXISTS scope text NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err = t.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gimbal_message_acceptances (
 namespace text NOT NULL, request_id text NOT NULL, worker text NOT NULL, digest bytea NOT NULL,
 PRIMARY KEY (namespace, request_id))`); err != nil {
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
	if len(t.config.Environments) != 0 {
		return t.publishNamed(ctx, r)
	}
	queue := t.config.Queue
	switch r.Kind {
	case "interrupt", "identified-close", "cleanup":
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
	budget := time.Duration(t.config.ReconnectSeconds) * time.Second
	if budget <= 0 {
		budget = 30 * time.Second
	}
	var disconnected time.Time
	for {
		started := time.Now()
		remaining := budget
		if !disconnected.IsZero() {
			remaining -= started.Sub(disconnected)
		}
		if remaining <= 0 {
			return fmt.Errorf("river event reconnect budget exhausted")
		}
		queryCtx, cancel := context.WithTimeout(ctx, remaining)
		rows, err := t.pool.Query(queryCtx, `SELECT event FROM gimbal_message_events
 WHERE queue=$1 AND created >= $2 AND ($3='' OR request_id=$3)
 AND ($4='' OR run_id=$4) AND ($5='' OR session_id=$5) ORDER BY created,id`, t.config.Queue, since, s.RequestID, s.Run, s.SessionID)
		var events []backend.MessageEvent
		if err == nil {
			for rows.Next() {
				var b []byte
				if err = rows.Scan(&b); err != nil {
					break
				}
				var event backend.MessageEvent
				if err = json.Unmarshal(b, &event); err != nil {
					rows.Close()
					cancel()
					return fmt.Errorf("invalid retained event: %w", err)
				}
				if !seen[event.ID] {
					events = append(events, event)
				}
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
		}
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && !strings.HasPrefix(pgErr.Code, "08") && pgErr.Code != "57P01" && pgErr.Code != "57P02" && pgErr.Code != "57P03" {
				return storeError(err)
			}
			if disconnected.IsZero() {
				disconnected = started
			}
			if time.Since(disconnected) >= budget {
				return fmt.Errorf("river event reconnect budget exhausted: %w", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		disconnected = time.Time{}
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
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type actionWorker struct {
	river.WorkerDefaults[actionArgs]
	ctx       context.Context
	handler   backend.MessageHandler
	mu        sync.Mutex
	seen      map[string]bool
	authority *authority
	workerID  string
}

func (w *actionWorker) Work(_ context.Context, job *river.Job[actionArgs]) error {
	if w.authority != nil {
		if job.Args.Request.WorkerID != w.workerID {
			return river.JobCancel(fmt.Errorf("request addressed to a different worker"))
		}
		check, cancel := context.WithTimeout(w.ctx, 2*time.Second)
		err := w.authority.admit(check, job.Args.Request)
		cancel()
		if err != nil {
			var rejected *backend.ContractRejected
			if errors.As(err, &rejected) {
				return river.JobCancel(err)
			}
			return river.JobSnooze(time.Second)
		}
	}
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

func serve(ctx context.Context, raw json.RawMessage, resources backend.Dependencies, handler backend.MessageHandler) error {
	c, err := parse(raw)
	if err != nil {
		return err
	}
	if len(c.Environments) != 0 && c.WorkerID == "" {
		return errors.New("river named worker requires WorkerID")
	}
	t, err := open(ctx, c)
	if err != nil {
		return err
	}
	defer t.pool.Close()
	if err := t.initialize(ctx); err != nil {
		return err
	}
	queues := map[string]river.QueueConfig{c.Queue: {MaxWorkers: c.Workers}, c.ControlQueue: {MaxWorkers: 2}}
	var owner *authority
	if len(c.Environments) != 0 {
		owner, err = registerWorker(ctx, t, c.WorkerID)
		if err != nil {
			return err
		}
		defer owner.close()
		queues = map[string]river.QueueConfig{workerQueue(c.Queue, c.WorkerID, false): {MaxWorkers: c.Workers}, workerQueue(c.Queue, c.WorkerID, true): {MaxWorkers: 2}}
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &actionWorker{ctx: ctx, handler: handler, seen: map[string]bool{}, authority: owner, workerID: c.WorkerID})
	client, err := river.NewClient(riverpgxv5.New(t.pool), &river.Config{
		Queues:  queues,
		Workers: workers,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	if resources.Ready != nil {
		if err := resources.Ready(); err != nil {
			_ = client.Stop(context.Background())
			return err
		}
	}
	var lost error
	if owner != nil {
		watchCtx, stopWatch := context.WithCancel(ctx)
		defer stopWatch()
		go owner.watch(watchCtx)
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case lost = <-owner.lost:
				if err := owner.reacquire(ctx, t, c.WorkerID); err != nil {
					lost = errors.Join(lost, err)
					break loop
				}
				lost = nil
			}
		}
	} else {
		<-ctx.Done()
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		return err
	}
	if lost != nil {
		return fmt.Errorf("river worker authority lost: %w", lost)
	}
	return ctx.Err()
}

func Register(registry *gimbal.Registry) error {
	return registry.RegisterMessaging(gimbal.MessagingDefinition{ExecutionIdentity: true,
		Name: "river",
		ValidateEnvironment: func(raw json.RawMessage, name string) error {
			c, err := parse(raw)
			if err != nil {
				return err
			}
			if c.Environments[name] == "" {
				return fmt.Errorf("river environment %q is not configured", name)
			}
			return nil
		},
		ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err },
		Connect: func(ctx context.Context, raw json.RawMessage, _ backend.Dependencies) (backend.Messaging, error) {
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
