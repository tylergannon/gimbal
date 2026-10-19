package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func TestExecutionRoutesProvideScopedSecrets(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	stop, err := serveEnvironmentOptions(t.Context(), address, "", nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	configuration, _ := json.Marshal(map[string]string{"endpoint": "http://" + address})
	for _, route := range []backend.Route{{Name: "local"}, {Name: "http", Configuration: configuration}} {
		t.Run(route.Name, func(t *testing.T) {
			var connection backend.Connection
			if route.Name == "local" {
				daemon := taskdaemon.New(t.TempDir())
				daemon.Secrets = backend.NewMemorySecrets()
				connection = daemon.Connection()
			} else {
				connection, err = gimbal.DefaultRegistry.Connect(t.Context(), route)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			scope := backend.Scope{Run: route.Name, Scope: "parent"}
			t.Cleanup(func() {
				if err := connection.Execution.Cleanup(context.Background(), scope); err != nil {
					t.Error(err)
				}
			})
			complete := func(process backend.Process) {
				t.Helper()
				if err := connection.Execution.Process(ctx, process); err != nil {
					t.Fatal(err)
				}
				for {
					result, err := connection.Execution.Result(ctx, backend.Target{Run: process.Run, ID: process.Task})
					if err != nil || result.Error != "" {
						t.Fatalf("%s result: %+v, %v", process.Task, result, err)
					}
					if result.Done {
						var outcome backend.ProcessResult
						if err := json.Unmarshal(result.Result, &outcome); err != nil || outcome.Error != "" || outcome.ExitCode != 0 {
							t.Fatalf("%s outcome: %+v, %v", process.Task, outcome, err)
						}
						return
					}
					select {
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					case <-time.After(10 * time.Millisecond):
					}
				}
			}
			complete(backend.Process{Run: scope.Run, Scope: scope.Scope, Task: "producer", Command: "sh", Args: []string{"-c", `printf 'TOKEN=published\nEXTRA=whole-store\n' > "$ENV_FILE"`}, Produces: true, Store: "private", Yields: []string{"TOKEN", "EXTRA"}})
			consumer := backend.Process{Run: scope.Run, Scope: "parent/child", Task: "consumer", Command: "sh", Args: []string{"-c", `test "$TOKEN" = published && test "$EXTRA" = whole-store`}, Requires: backend.Requirements{{Store: "private", Names: []string{"TOKEN"}}}}
			complete(consumer)
			marker := filepath.Join(t.TempDir(), "must-not-exist")
			consumer.Task = "missing-requirement"
			consumer.Args = []string{"-c", `touch "$1"`, "sh", marker}
			consumer.Requires[0].Names = []string{"MISSING"}
			if err := connection.Execution.Process(ctx, consumer); err == nil {
				t.Fatal("missing requirement admitted")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("native work ran before requirements were satisfied: %v", err)
			}
		})
	}
}
