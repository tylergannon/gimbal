package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// This starts the actual CLI and River plugin, with only the disposable DSN
// supplied by the operator. The authority lock is deliberately held elsewhere
// after the first worker loses it, while its executor-directory lease remains.
func TestMessageWorkerStateLeaseSurvivesLostDatabaseAuthority(t *testing.T) {
	dsnFile := os.Getenv("GIMBAL_RIVER_TEST_DSN_FILE")
	if dsnFile == "" {
		t.Skip("set GIMBAL_RIVER_TEST_DSN_FILE to the disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plugins := filepath.Join(dir, "plugins")
	if err := os.Mkdir(plugins, 0700); err != nil {
		t.Fatal(err)
	}
	binary, plugin := filepath.Join(dir, "gimbal"), filepath.Join(plugins, "river.so")
	for _, args := range [][]string{{"build", "-o", binary, "./cmd/gimbal"}, {"build", "-buildmode=plugin", "-o", plugin, "./plugins/messaging-river"}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build worker: %v: %s", err, out)
		}
	}
	queue := fmt.Sprintf("lease_test_%d", time.Now().UnixNano())
	worker := "worker"
	config, _ := json.Marshal(map[string]any{"Name": "river", "Configuration": map[string]any{"DSNFile": dsnFile, "Queue": queue, "WorkerID": worker, "Environments": map[string]string{"selected": worker}, "ReconnectSeconds": 15}})
	route := filepath.Join(dir, "route.json")
	if err := os.WriteFile(route, config, 0600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	start := func(label string) (*exec.Cmd, <-chan error, string, *bytes.Buffer) {
		t.Helper()
		ready := filepath.Join(dir, label+".ready")
		var output bytes.Buffer
		cmd := exec.CommandContext(ctx, binary, "message-worker", "--plugin-dir", plugins, "--task-route", "@"+route, "--executor-state-dir", state, "--ready-file", ready)
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		return cmd, done, ready, &output
	}
	waitReady := func(ready string, done <-chan error) {
		t.Helper()
		for {
			if _, err := os.Stat(ready); err == nil {
				return
			}
			select {
			case err := <-done:
				t.Fatalf("worker exited before ready: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	first, firstDone, ready, _ := start("first")
	firstStopped := false
	defer func() {
		if !firstStopped {
			_ = first.Process.Kill()
			select {
			case <-firstDone:
			case <-time.After(time.Second):
			}
		}
	}()
	waitReady(ready, firstDone)
	dsn, err := os.ReadFile(dsnFile)
	if err != nil {
		t.Fatal("cannot read disposable DSN file")
	}
	admin, err := pgx.Connect(ctx, strings.TrimSpace(string(dsn)))
	if err != nil {
		t.Fatal("cannot connect to disposable database")
	}
	defer func() { _ = admin.Close(context.Background()) }()
	defer func() {
		_, _ = admin.Exec(context.Background(), "DELETE FROM gimbal_message_bindings WHERE namespace=$1", queue)
	}()
	blocker, err := pgx.Connect(ctx, strings.TrimSpace(string(dsn)))
	if err != nil {
		t.Fatal("cannot connect blocker")
	}
	blockerClosed := false
	defer func() {
		if !blockerClosed {
			_ = blocker.Close(context.Background())
		}
	}()
	key := fmt.Sprintf("gimbal-worker:%d:%s%s", len(queue), queue, worker)
	var pid int32
	err = admin.QueryRow(ctx, `SELECT pid FROM pg_locks WHERE locktype='advisory' AND granted AND objsubid=1
 AND classid::bigint=((hashtextextended($1,0) >> 32) & 4294967295)
 AND objid::bigint=(hashtextextended($1,0) & 4294967295)`, key).Scan(&pid)
	if err != nil {
		t.Fatal("cannot locate isolated worker authority")
	}
	blockerPID := blocker.PgConn().PID()
	acquired := make(chan error, 1)
	go func() {
		_, err := blocker.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", key)
		acquired <- err
	}()
	for {
		var waiting bool
		if err := admin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)", blockerPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	var killed bool
	if err := admin.QueryRow(ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&killed); err != nil || !killed {
		t.Fatal("cannot release isolated worker authority")
	}
	if err := <-acquired; err != nil {
		t.Fatal(err)
	}
	if err := first.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatal("first worker exited before lease check")
	}
	second := exec.CommandContext(ctx, binary, "message-worker", "--plugin-dir", plugins, "--task-route", "@"+route, "--executor-state-dir", state)
	out, err := second.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "executor state directory already owned") {
		t.Fatalf("second worker did not reject the held state directory: %v", err)
	}
	_ = first.Process.Signal(syscall.SIGTERM)
	<-firstDone
	firstStopped = true
	if err := blocker.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	blockerClosed = true
	third, thirdDone, ready3, _ := start("third")
	defer func() {
		_ = third.Process.Kill()
		select {
		case <-thirdDone:
		case <-time.After(time.Second):
		}
	}()
	waitReady(ready3, thirdDone)
}
