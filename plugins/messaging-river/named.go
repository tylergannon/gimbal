package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/tylergannon/gimbal/backend"
)

// A worker's queue is derived from the namespace and stable worker identity,
// not from the logical name, so a rebind cannot redirect already queued work.
func workerQueue(namespace, worker string, control bool) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s%s", len(namespace), namespace, worker)))
	suffix := "a"
	if control {
		suffix = "c"
	}
	return fmt.Sprintf("gimbal_%x_%s", digest[:16], suffix)
}
func authorityKey(namespace, worker string) string {
	return fmt.Sprintf("gimbal-worker:%d:%s%s", len(namespace), namespace, worker)
}

type authority struct {
	mu                sync.Mutex
	conn              *pgx.Conn
	lost              chan error
	namespace, worker string
}

func registerWorker(ctx context.Context, t *transport, worker string) (*authority, error) {
	conn, err := pgx.ConnectConfig(ctx, t.pool.Config().ConnConfig)
	if err != nil {
		return nil, err
	}
	var acquired bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", authorityKey(t.config.Queue, worker)).Scan(&acquired)
	if err != nil || !acquired {
		_ = conn.Close(context.Background())
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("river worker %q is already registered", worker)
	}
	return &authority{conn: conn, lost: make(chan error, 1), namespace: t.config.Queue, worker: worker}, nil
}

// The assignment is committed by the connection that owns registration.
func (a *authority) admit(ctx context.Context, r backend.MessageRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.Created = time.Time{}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	var worker string
	var saved []byte
	err = a.conn.QueryRow(ctx, `INSERT INTO gimbal_message_acceptances(namespace,request_id,worker,digest)
 VALUES ($1,$2,$3,$4) ON CONFLICT (namespace,request_id) DO UPDATE SET request_id=EXCLUDED.request_id
 RETURNING worker,digest`, a.namespace, r.ID, a.worker, digest[:]).Scan(&worker, &saved)
	if err != nil {
		select {
		case a.lost <- err:
		default:
		}
		return err
	}
	if worker != a.worker || !bytes.Equal(saved, digest[:]) {
		return &backend.ContractRejected{Message: "request identity conflicts with retained assignment"}
	}
	return nil
}

// Confirm on the same connection that owns the database lock. Failure closes
// admission immediately; River snoozes the delivery rather than consuming it.
func (a *authority) confirm(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := a.conn.Exec(ctx, "SELECT 1")
	if err != nil {
		select {
		case a.lost <- err:
		default:
		}
	}
	return err
}
func (a *authority) close() { a.mu.Lock(); defer a.mu.Unlock(); _ = a.conn.Close(context.Background()) }

// Reacquisition holds the admission gate until the same lock is confirmed.
func (a *authority) reacquire(ctx context.Context, t *transport, worker string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.conn.Close(context.Background())
	deadline, cancel := context.WithTimeout(ctx, time.Duration(t.config.ReconnectSeconds)*time.Second)
	defer cancel()
	for {
		attempt, stop := context.WithTimeout(deadline, time.Second)
		conn, err := pgx.ConnectConfig(attempt, t.pool.Config().ConnConfig)
		if err == nil {
			var acquired bool
			err = conn.QueryRow(attempt, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", authorityKey(t.config.Queue, worker)).Scan(&acquired)
			if err == nil && acquired {
				stop()
				a.conn = conn
				for {
					select {
					case <-a.lost:
					default:
						return nil
					}
				}
			}
			_ = conn.Close(context.Background())
		}
		stop()
		select {
		case <-deadline.Done():
			return fmt.Errorf("river worker %q could not reacquire registration: %w", worker, deadline.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (t *transport) publishNamed(ctx context.Context, r backend.MessageRequest) error {
	if r.Kind == "cleanup" {
		return &backend.ContractRejected{Message: "named scope cleanup across retained owners is not implemented"}
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return storeError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	worker := t.config.Environments[r.Environment]
	ownerKind, ownerID := "", ""
	switch r.Kind {
	case "identified-session":
		ownerKind, ownerID = "session", r.SessionID
	case "identified-process":
		ownerKind, ownerID = "task", r.Task
	case "identified-turn", "identified-close", "steer":
		ownerKind, ownerID = "session", r.SessionID
	case "interrupt":
		if r.Target != nil {
			ownerKind, ownerID = "task", r.Target.ID
		}
	}
	if ownerKind != "" && ownerID != "" && r.Run != "" {
		if r.Kind == "identified-session" || r.Kind == "identified-process" {
			if worker == "" {
				return &backend.ContractRejected{Message: fmt.Sprintf("river environment %q is not configured", r.Environment)}
			}
			_, err = tx.Exec(ctx, `INSERT INTO gimbal_message_owners (namespace,run_id,kind,identity,worker,environment,scope)
 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, t.config.Queue, r.Run, ownerKind, ownerID, worker, r.Environment, r.Scope)
			if err != nil {
				return storeError(err)
			}
		}
		var environment, ownerScope string
		err = tx.QueryRow(ctx, `SELECT worker,environment,scope FROM gimbal_message_owners
 WHERE namespace=$1 AND run_id=$2 AND kind=$3 AND identity=$4`, t.config.Queue, r.Run, ownerKind, ownerID).Scan(&worker, &environment, &ownerScope)
		if errors.Is(err, pgx.ErrNoRows) {
			return &backend.ContractRejected{Message: "river retained owner is unknown"}
		}
		if err != nil {
			return storeError(err)
		}
		if r.Environment != "" && r.Environment != environment {
			return &backend.ContractRejected{Message: "environment conflicts with retained owner"}
		}
		if (r.Kind == "identified-session" || r.Kind == "identified-process") && ownerScope != r.Scope {
			return &backend.ContractRejected{Message: "scope conflicts with retained owner"}
		}
		r.Environment = environment
		if r.Kind == "identified-turn" && r.Task != "" {
			_, err = tx.Exec(ctx, `INSERT INTO gimbal_message_owners (namespace,run_id,kind,identity,worker,environment,scope)
 VALUES ($1,$2,'task',$3,$4,$5,$6) ON CONFLICT DO NOTHING`, t.config.Queue, r.Run, r.Task, worker, environment, r.Scope)
			if err != nil {
				return storeError(err)
			}
			var taskWorker, taskEnvironment, taskScope string
			if err = tx.QueryRow(ctx, `SELECT worker,environment,scope FROM gimbal_message_owners WHERE namespace=$1 AND run_id=$2 AND kind='task' AND identity=$3`, t.config.Queue, r.Run, r.Task).Scan(&taskWorker, &taskEnvironment, &taskScope); err != nil {
				return storeError(err)
			}
			if taskWorker != worker || taskEnvironment != environment || taskScope != r.Scope {
				return &backend.ContractRejected{Message: "task conflicts with retained owner"}
			}
		}
	}
	if worker == "" {
		return &backend.ContractRejected{Message: fmt.Sprintf("river environment %q is not configured", r.Environment)}
	}
	if r.WorkerID != "" && r.WorkerID != worker {
		return &backend.ContractRejected{Message: "worker conflicts with retained owner"}
	}
	r.WorkerID = worker
	var vacant bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", authorityKey(t.config.Queue, worker)).Scan(&vacant); err != nil {
		return storeError(err)
	}
	if vacant {
		return &backend.ContractRejected{Message: fmt.Sprintf("river worker %q is not registered", worker)}
	}
	control := r.Kind == "interrupt" || r.Kind == "identified-close" || r.Kind == "cleanup"
	_, err = t.client.InsertTx(ctx, tx, actionArgs{Request: r}, &river.InsertOpts{Queue: workerQueue(t.config.Queue, worker, control), MaxAttempts: 1})
	if err != nil {
		return storeError(err)
	}
	return storeError(tx.Commit(ctx))
}

func (a *authority) watch(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping, cancel := context.WithTimeout(ctx, 2*time.Second)
			_ = a.confirm(ping)
			cancel()
		}
	}
}

// CleanupScope sends one stable cleanup request to each retained native owner.
// Each completion is awaited before reporting that the scope was released.
func (t *transport) CleanupScope(ctx context.Context, request backend.MessageRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	rows, err := t.pool.Query(ctx, `SELECT worker,min(environment) FROM gimbal_message_owners
 WHERE namespace=$1 AND run_id=$2 AND ($3='' OR scope=$3 OR left(scope,length($3)+1)=$3||'/')
 GROUP BY worker ORDER BY worker`, t.config.Queue, request.Run, request.Scope)
	if err != nil {
		return storeError(err)
	}
	type destination struct{ worker, environment string }
	var destinations []destination
	for rows.Next() {
		var d destination
		if err = rows.Scan(&d.worker, &d.environment); err != nil {
			break
		}
		destinations = append(destinations, d)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return storeError(err)
	}
	for _, d := range destinations {
		child := request
		sum := sha256.Sum256([]byte(request.ID + "\x00" + d.worker))
		child.ID = fmt.Sprintf("%x", sum[:])
		child.WorkerID, child.Environment = d.worker, d.environment
		tx, err := t.pool.Begin(ctx)
		if err != nil {
			return &backend.DeliveryUncertain{RequestID: request.ID, Err: err}
		}
		var vacant bool
		err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", authorityKey(t.config.Queue, d.worker)).Scan(&vacant)
		if err == nil && vacant {
			err = fmt.Errorf("retained worker %q is not registered", d.worker)
		}
		if err == nil {
			_, err = t.client.InsertTx(ctx, tx, actionArgs{Request: child}, &river.InsertOpts{Queue: workerQueue(t.config.Queue, d.worker, true), MaxAttempts: 1})
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(context.Background())
		}
		if err != nil {
			return &backend.DeliveryUncertain{RequestID: request.ID, Err: fmt.Errorf("cleanup at %s unconfirmed: %w", d.worker, err)}
		}
		completed := errors.New("cleanup completion received")
		var outcome *backend.MessageResult
		err = t.ListenEvents(ctx, backend.MessageSelection{RequestID: child.ID}, time.Time{}, func(e backend.MessageEvent) error {
			if e.Kind == "completed" {
				outcome = e.Result
				return completed
			}
			return nil
		})
		if !errors.Is(err, completed) || outcome == nil {
			return &backend.DeliveryUncertain{RequestID: request.ID, Err: fmt.Errorf("cleanup outcome at %s unavailable: %v", d.worker, err)}
		}
		if outcome.Unresolved || outcome.Error != "" {
			return &backend.DeliveryUncertain{RequestID: request.ID, Err: fmt.Errorf("cleanup at %s: %s", d.worker, outcome.Error)}
		}
	}
	return nil
}
