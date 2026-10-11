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
	"time"
)

func TestDefaultLocalServiceFulfillmentAndScopeRelease(t *testing.T) {
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
			var err error
			address, err = FulfillService(ctx, "server", workdir, fmt.Sprintf("exec python3 -m http.server %d --bind 127.0.0.1", port), port, "workflow")
			if err != nil {
				return err
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				response, err := http.Get(address + "/value.txt")
				if err == nil {
					raw, readErr := io.ReadAll(response.Body)
					closeErr := response.Body.Close()
					if readErr != nil {
						return readErr
					}
					if closeErr != nil {
						return closeErr
					}
					if string(raw) != "local-service-bytes" {
						return fmt.Errorf("service returned %q", raw)
					}
					return nil
				}
				if time.Now().After(deadline) {
					return err
				}
				time.Sleep(20 * time.Millisecond)
			}
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
