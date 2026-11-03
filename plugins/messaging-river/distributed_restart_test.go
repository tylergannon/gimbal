package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/web"
)

// TestDistributedRestartOwnedParticipant requires the independently operated app
// and workers. Set GIMBAL_DISTRIBUTED_RESTART_TEST=1 and absolute paths in
// GIMBAL_DISTRIBUTED_INSTANCE, GIMBAL_DISTRIBUTED_PROJECT,
// GIMBAL_DISTRIBUTED_ROUTE, GIMBAL_DISTRIBUTED_REMOTE_DIR and
// GIMBAL_DISTRIBUTED_RESTART_CALLBACK; the route must configure the private DSN.
// The database user must create/drop a trigger and function, inspect locks and
// acquire advisory locks. The callback receives (inspect-stop|restart, run ID,
// request ID, remote marker path). inspect-stop reads the matching worker-b
// retained disposition, stops only that worker even if publication is blocked,
// waits for exit and database disconnection, then prints the exact JSON object
// containing Request, Result, EventID and Sequence without credentials. It must
// not alter the marker or state. restart starts the same worker configuration
// and state directory and waits for readiness; it must not submit work or edit
// state. Both phases exit nonzero on failure. No callback program is committed.
func TestDistributedRestartOwnedParticipant(t *testing.T) {
	if os.Getenv("GIMBAL_DISTRIBUTED_RESTART_TEST") != "1" {
		t.Skip("set GIMBAL_DISTRIBUTED_RESTART_TEST=1 with owned participants")
	}
	required := func(key string) string {
		t.Helper()
		v := os.Getenv(key)
		if !filepath.IsAbs(v) {
			t.Fatalf("%s must be an absolute path", key)
		}
		return v
	}
	instance, project := required("GIMBAL_DISTRIBUTED_INSTANCE"), required("GIMBAL_DISTRIBUTED_PROJECT")
	routePath, remoteDir := required("GIMBAL_DISTRIBUTED_ROUTE"), required("GIMBAL_DISTRIBUTED_REMOTE_DIR")
	callback := required("GIMBAL_DISTRIBUTED_RESTART_CALLBACK")
	raw, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatal(err)
	}
	var route backend.Route
	if err := json.Unmarshal(raw, &route); err != nil {
		t.Fatal(err)
	}
	if route.Name != "river" || route.Environment != "" {
		t.Fatal("route must select River without physical environment")
	}
	cfg, err := parse(route.Configuration)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environments["task-remote"] != "worker-b" {
		t.Fatal("task-remote must bind worker-b")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	bus, err := open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close River: %v", err)
		}
	}()
	lock, err := pgx.ConnectConfig(ctx, bus.pool.Config().ConnConfig)
	if err != nil {
		t.Fatal("connect lock session")
	}
	runID, token := ulid.Make().String(), ulid.Make().String()
	marker := path.Join(remoteDir, ".gimbal-d7-"+runID)
	// The function and trigger names are unique, and only this queue/run and a
	// worker-b native completion can wait. A request-keyed advisory lock lets us
	// match the blocked INSERT to the separately observed acceptance.
	name := "gimbal_d7_" + strings.ToLower(runID)
	ident := pgx.Identifier{name}.Sanitize()
	literal := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	installed := false
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		if _, err := lock.Exec(cleanup, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, runID); err != nil {
			t.Errorf("release advisory lock: %v", err)
		}
		if err := lock.Close(cleanup); err != nil {
			t.Errorf("close advisory lock session: %v", err)
		}
		if installed {
			if _, err := bus.pool.Exec(cleanup, "DROP TRIGGER IF EXISTS "+ident+" ON gimbal_message_events"); err != nil {
				t.Errorf("drop trigger: %v", err)
			}
			if _, err := bus.pool.Exec(cleanup, "DROP FUNCTION IF EXISTS "+ident+"()"); err != nil {
				t.Errorf("drop function: %v", err)
			}
		}
	}()
	var lockPID int32
	if err := lock.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&lockPID); err != nil {
		t.Fatalf("get advisory lock backend PID: %v", err)
	}
	if _, err := lock.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, runID); err != nil {
		t.Fatal(err)
	}
	sql := `CREATE FUNCTION ` + ident + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.queue = ` + literal(cfg.Queue) + ` AND NEW.run_id = ` + literal(runID) + ` AND convert_from(NEW.event,'UTF8')::jsonb->>'WorkerID' = 'worker-b' AND convert_from(NEW.event,'UTF8')::jsonb->>'Kind' = 'completed' AND convert_from(NEW.event,'UTF8')::jsonb->'Result'->'Process' IS NOT NULL THEN
 PERFORM pg_advisory_xact_lock(713746, hashtext(NEW.request_id));
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.run_id, 0)); END IF; RETURN NEW; END $$`
	if _, err := bus.pool.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	installed = true
	if _, err := bus.pool.Exec(ctx, `CREATE TRIGGER `+ident+` BEFORE INSERT ON gimbal_message_events FOR EACH ROW EXECUTE FUNCTION `+ident+`() `); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]string{"remote_dir": remoteDir, "marker": marker, "token": token})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := web.Start(ctx, instance, invocation.Request{RunID: runID, Workflow: "distributed-restart-test", Input: input, Project: project, WorkDir: project, TaskRoute: route})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.RunID != runID {
		t.Fatalf("run identity: %s", accepted.RunID)
	}
	found := errors.New("accepted")
	var admission backend.MessageEvent
	err = bus.ListenEvents(ctx, backend.MessageSelection{Run: runID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Kind == "accepted" && e.WorkerID == "worker-b" {
			admission = e
			return found
		}
		return nil
	})
	if !errors.Is(err, found) {
		t.Fatalf("acceptance: %v", err)
	}
	// Confirm that this exact admission owns a granted request lock and is waiting
	// on the test session. No other run or queue can enter this trigger branch.
	for {
		var waiting bool
		err = bus.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE l.locktype='advisory' AND l.granted AND l.classid=713746::oid AND l.objid=hashtext($1)::oid AND $2 = ANY(pg_blocking_pids(l.pid)))`, admission.RequestID, lockPID).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("completion did not reach interruption point")
		case <-time.After(100 * time.Millisecond):
		}
	}
	jobCount := func() int {
		t.Helper()
		var n int
		if err := bus.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE queue=$1 AND kind='gimbal_action' AND args->'request'->>'ID'=$2`, workerQueue(cfg.Queue, "worker-b", false), admission.RequestID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := jobCount(); n != 1 {
		t.Fatalf("original request has %d River jobs", n)
	}
	var count int
	if err := bus.pool.QueryRow(ctx, `SELECT count(*) FROM gimbal_message_events WHERE queue=$1 AND run_id=$2 AND request_id=$3 AND convert_from(event,'UTF8')::jsonb->>'Kind'='completed'`, cfg.Queue, runID, admission.RequestID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("published completions while blocked: %d, %v", count, err)
	}
	type disposition struct {
		Request  backend.MessageRequest
		Result   *backend.MessageResult
		EventID  string
		Sequence uint64
	}
	call := func(phase string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, callback, phase, runID, admission.RequestID, marker)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("callback %s failed: %v", phase, err)
		}
		return output
	}
	var saved disposition
	if err := json.Unmarshal(call("inspect-stop"), &saved); err != nil {
		t.Fatalf("callback JSON: %v", err)
	}
	if saved.Request.ID != admission.RequestID || saved.Request.Run != runID || saved.Request.WorkerID != "worker-b" || saved.Request.Environment != "task-remote" || saved.Request.Owner != admission.Owner || saved.Request.Kind != "identified-process" || saved.Request.Process == nil || saved.Request.Task == "" || saved.EventID == "" || saved.Sequence == 0 || saved.Result == nil || saved.Result.Process == nil || saved.Result.Error != "" || saved.Result.Unresolved || saved.Result.Process.ExitCode != 0 || saved.Result.Process.Error != "" || saved.Result.Process.Interrupted || saved.Request.ExecutionIdentity.Validate("process") != nil || saved.Request.Process.ExecutionIdentity != saved.Request.ExecutionIdentity || saved.Request.Process.Owner != saved.Request.Owner || saved.Request.Process.Workdir != remoteDir {
		t.Fatalf("invalid retained disposition: %+v", saved.Request)
	}
	lines := strings.Split(strings.TrimSpace(string(saved.Result.Stdout)), "\n")
	if len(lines) != 3 || lines[0] == "" || lines[1] != token || lines[2] != "1" {
		t.Fatalf("native result: %q", saved.Result.Stdout)
	}
	if _, err := bus.pool.Exec(ctx, "DROP TRIGGER "+ident+" ON gimbal_message_events"); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.pool.Exec(ctx, "DROP FUNCTION "+ident+"()"); err != nil {
		t.Fatal(err)
	}
	installed = false
	if _, err := lock.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, runID); err != nil {
		t.Fatal(err)
	}
	call("restart")
	recovered := errors.New("recovered")
	err = bus.ListenEvents(ctx, backend.MessageSelection{Run: runID, RequestID: admission.RequestID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Kind != "completed" {
			return nil
		}
		if e.ID != saved.EventID || e.Sequence != saved.Sequence || e.Source != saved.Request.ID || e.Owner != saved.Request.Owner || e.WorkerID != saved.Request.WorkerID || e.Environment != saved.Request.Environment || e.Result == nil {
			return fmt.Errorf("recovered event identity mismatch")
		}
		a, err := json.Marshal(saved.Result)
		if err != nil {
			return err
		}
		b, err := json.Marshal(e.Result)
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			return errors.New("recovered result differs from persisted result")
		}
		return recovered
	})
	if !errors.Is(err, recovered) {
		t.Fatalf("recovery: %v", err)
	}
	if n := jobCount(); n != 1 {
		t.Fatalf("request was resubmitted: %d River jobs", n)
	}
	row, err := web.Follow(ctx, instance, project, runID)
	if err != nil || row.Status != "completed" {
		t.Fatalf("workflow: status=%s error=%s: %v", row.Status, row.Error, err)
	}
	t.Logf("run=%s request=%s event=%s recovered; marker verified and removed", runID, admission.RequestID, saved.EventID)
}
