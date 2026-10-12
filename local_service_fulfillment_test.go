//go:build darwin || linux

package gimbal

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

func TestDefaultLocalServicePublishesAndReleases(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	workdir := t.TempDir()
	if err = os.WriteFile(filepath.Join(workdir, "value.txt"), []byte("local-service-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	var address string
	err = Run(Project(t.Context(), t.TempDir()), "local-service", nil, func(ctx context.Context) error {
		return Scope(ctx, "http", func(ctx context.Context) error {

			command := fmt.Sprintf(`exec python3 -c 'import http.server,os,threading,signal
server=http.server.ThreadingHTTPServer(("127.0.0.1",%d),http.server.SimpleHTTPRequestHandler)
threading.Thread(target=server.serve_forever,daemon=True).start()
with open(os.environ["ENV_FILE"]+".tmp","w") as f:f.write("SERVICE_URL=http://127.0.0.1:%%d\n"%%server.server_port)
os.replace(os.environ["ENV_FILE"]+".tmp",os.environ["ENV_FILE"])
signal.signal(signal.SIGINT,lambda *_:exit(0))
signal.pause()'`, port)
			if err := Service(WithYields(ctx, DefaultStore, "SERVICE_URL"), "server", workdir, command); err != nil {
				return err
			}
			values, err := effects.Connection(ctx).Secrets.Acquire(ctx, backend.Scope{Run: effectOwner(mustScope(ctx), "").Run, Scope: mustScope(ctx).key}, backend.Requirements{{Names: []string{"SERVICE_URL"}}})
			if err != nil {
				return err
			}
			address = values["SERVICE_URL"]
			response, err := http.Get(address + "/value.txt")
			if err != nil {
				return err
			}
			defer func() { _ = response.Body.Close() }()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if string(raw) != "local-service-bytes" {
				return fmt.Errorf("service returned %q", raw)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if response, err := http.Get(address + "/value.txt"); err == nil {
		_ = response.Body.Close()
		t.Fatal("scope-owned service remained reachable")
	}
}

func mustScope(ctx context.Context) *scope {
	s, err := current(ctx)
	if err != nil {
		panic(err)
	}
	return s
}
