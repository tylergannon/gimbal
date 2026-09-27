package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func RunWorker(ctx context.Context) error {
	environment := os.Getenv("GIMBAL_ENVIRONMENT")
	owner := os.Getenv("GIMBAL_WORKER_ID")
	dsn := os.Getenv("GIMBAL_POSTGRES_DSN")
	address := os.Getenv("GIMBAL_TEMPORAL_ADDRESS")
	if environment == "" || owner == "" || dsn == "" || address == "" {
		return errors.New("gimbal-worker: required environment configuration is missing")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	var row bootstrap
	var mounts []byte
	if err := db.QueryRow(ctx, `SELECT name,task_queue,owner_id,container_name,mounts FROM gimbal_environments WHERE name=$1`, environment).Scan(&row.Name, &row.Queue, &row.Owner, &row.Container, &mounts); err != nil {
		return fmt.Errorf("gimbal-worker: load bootstrap: %w", err)
	}
	if row.Owner != owner {
		return fmt.Errorf("gimbal-worker: bootstrap owner mismatch for %q", environment)
	}
	if err := json.Unmarshal(mounts, &row.Mounts); err != nil {
		return err
	}
	temporal, err := client.Dial(client.Options{HostPort: address})
	if err != nil {
		return err
	}
	defer temporal.Close()
	w := worker.New(temporal, row.Queue, worker.Options{MaxConcurrentActivityExecutionSize: 32})
	w.RegisterActivityWithOptions(func(ctx context.Context, in commandInput) (commandResult, error) {
		return runCommandActivity(ctx, db, in)
	}, activity.RegisterOptions{Name: commandActivity})
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(worker.InterruptCh()) }()
	select {
	case <-ctx.Done():
		w.Stop()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func runCommandActivity(ctx context.Context, db *pgxpool.Pool, in commandInput) (commandResult, error) {
	result := commandResult{ExitCode: -1}
	out, err := os.OpenFile(in.StdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		runErr := fmt.Errorf("open stdout capture: %w", err)
		return recordCommandEvent(db, in, result, runErr)
	}
	errOut, err := os.OpenFile(in.StderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = out.Close()
		runErr := fmt.Errorf("open stderr capture: %w", err)
		return recordCommandEvent(db, in, result, runErr)
	}
	cmd := exec.Command(in.Command, in.Args...)
	cmd.Dir = in.Workdir
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	runErr := cmd.Start()
	if runErr == nil {
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					activity.RecordHeartbeat(ctx, in.Operation)
				case <-heartbeatDone:
					return
				}
			}
		}()
		wait := make(chan error, 1)
		go func() { wait <- cmd.Wait() }()
		select {
		case runErr = <-wait:
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			select {
			case runErr = <-wait:
			case <-time.After(time.Second):
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				runErr = <-wait
			}
		}
		close(heartbeatDone)
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
		}
		if ctx.Err() != nil {
			runErr = ctx.Err()
		} else {
			if _, ok := errors.AsType[*exec.ExitError](runErr); ok {
				runErr = nil
			}
		}
	}
	if closeErr := errors.Join(out.Close(), errOut.Close()); runErr == nil && closeErr != nil {
		runErr = closeErr
	}
	return recordCommandEvent(db, in, result, runErr)
}

func recordCommandEvent(db *pgxpool.Pool, in commandInput, result commandResult, runErr error) (commandResult, error) {
	// Recording is intentionally best effort and cannot change the process result.
	recordCtx, cancelRecord := context.WithTimeout(context.Background(), 2*time.Second)
	_, recordErr := db.Exec(recordCtx, `INSERT INTO gimbal_command_events(environment_name,operation_id,session_id,role,command,args,workdir,exit_code,error_text) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, in.Environment, in.Operation, in.Session, in.Role, in.Command, mustJSON(in.Args), in.Workdir, result.ExitCode, errorText(runErr))
	cancelRecord()
	if recordErr != nil {
		result.RecordingError = recordErr.Error()
		log.Printf("gimbal-worker: command event recording degraded for %s: %v", in.Operation, recordErr)
	}
	return result, runErr
}

func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
