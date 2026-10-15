package environments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiteralSSHHostSuggestionsRespectIncludes(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "other"), []byte("Host studio\n HostName studio.internal\nHost !ignored *.wild\n"), 0600); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "config")
	if e := os.WriteFile(path, []byte("Host build-box *.example\nInclude other\nHost build-box\n"), 0600); e != nil {
		t.Fatal(e)
	}
	hosts, e := sshHosts(context.Background(), path)
	if e != nil || strings.Join(hosts, ",") != "build-box,studio" {
		t.Fatal(hosts, e)
	}
}
func TestCachedSSHTunnelCanRetryFailedBootstrap(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ready")
	allow := filepath.Join(dir, "allow")
	script := "#!/bin/sh\nif test -f '" + allow + "'; then touch '" + marker + "'; exit 0; fi\nexit 1\n"
	if e := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, e := os.Stat(marker); e != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	s := testService(t)
	m := Machine{ID: "box", Name: "Box", Kind: KindSSH, SSH: &SSH{Host: "box"}}
	key, _ := json.Marshal(m)
	s.tunnels[string(key)] = &tunnel{endpoint: server.URL, done: make(chan struct{})}
	defer delete(s.tunnels, string(key))
	if _, e := s.sshEndpoint(context.Background(), m, true); e == nil {
		t.Fatal("first missing daemon bootstrap unexpectedly passed")
	}
	if e := os.WriteFile(allow, nil, 0600); e != nil {
		t.Fatal(e)
	}
	endpoint, e := s.sshEndpoint(context.Background(), m, true)
	if e != nil || endpoint != server.URL {
		t.Fatalf("cached tunnel prevented bootstrap retry: %s %v", endpoint, e)
	}
}
