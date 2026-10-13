package main

import (
	"strings"
	"testing"
)

func TestExplicitServerURLDoesNotLaunchOrManageBundledServer(t *testing.T) {
	t.Setenv("GIMBAL_SERVER_URL", "http://127.0.0.1:9187")
	if target, err := serverTarget(); err != nil || target != "http://127.0.0.1:9187" {
		t.Fatalf("serverTarget() = %q, %v", target, err)
	}
	bundledServer.Lock()
	defer bundledServer.Unlock()
	if bundledServer.cmd != nil || bundledServer.stopped != nil {
		t.Fatal("explicit server URL started a bundled child")
	}
}

func TestServerEnvironmentPreservesPATHAndAddsUserTools(t *testing.T) {
	t.Setenv("PATH", "/custom/bin")
	path := ""
	for _, entry := range serverEnvironment("/Users/test") {
		if strings.HasPrefix(entry, "PATH=") {
			path = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	for _, want := range []string{
		"/custom/bin:/Users/test/go/bin",
		"/Users/test/.local/bin",
		"/Users/test/.local/share/mise/shims",
		"/opt/homebrew/bin",
		"/usr/local/bin",
	} {
		if !strings.Contains(path, want) {
			t.Fatalf("server PATH is missing %q", want)
		}
	}
	if strings.HasPrefix(path, ":") || strings.HasSuffix(path, ":") || strings.Contains(path, "::") {
		t.Fatal("server PATH contains an empty/current-directory entry")
	}
}
