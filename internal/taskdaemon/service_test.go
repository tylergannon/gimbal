package taskdaemon

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

func TestUsefulServiceAddressAndScopeRelease(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	d := New()
	d.PublicHost = "127.0.0.1"
	owner := backend.Owner{Run: "run", Scope: "scope", Task: "service"}
	address, e := d.service(backend.Service{Owner: owner, Workdir: t.TempDir(), Command: "python3", Args: []string{"-m", "http.server", strconv.Itoa(port), "--bind", "127.0.0.1"}, Port: port, Consumer: "workflow-A"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	defer func() { _ = d.cleanup(ctx, backend.Scope{Run: "run"}) }()
	for {
		response, err := http.Get(address.URL)
		if err == nil {
			raw, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if len(raw) == 0 {
				t.Fatal("empty useful HTTP response")
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	if e = d.cleanup(ctx, backend.Scope{Run: "run"}); e != nil {
		t.Fatal(e)
	}
	if _, e = http.Get(address.URL); e == nil {
		t.Fatal("service remained reachable after release")
	}
}
