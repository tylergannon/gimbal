package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBundledServerLoadsOnlyThisCatalogsPlugins(t *testing.T) {
	directory := t.TempDir()
	args, err := bundledServerArguments(directory)
	if err != nil || slices.Contains(args, "--plugin-dir") {
		t.Fatalf("empty catalog arguments = %v, %v", args, err)
	}
	plugins := filepath.Join(directory, "plugins")
	if err := os.Mkdir(plugins, 0o700); err != nil {
		t.Fatal(err)
	}
	args, err = bundledServerArguments(directory)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.Index(args, "--plugin-dir")
	if index < 0 || index+1 >= len(args) || args[index+1] != plugins {
		t.Fatalf("catalog plugin arguments = %v", args)
	}
	other := t.TempDir()
	args, err = bundledServerArguments(other)
	if err != nil || slices.Contains(args, "--plugin-dir") {
		t.Fatalf("another catalog inherited plugins: %v, %v", args, err)
	}
}

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

func TestInstanceDirDefaultsToDesktopAndHonorsOverride(t *testing.T) {
	t.Setenv("GIMBAL_INSTANCE_DIR", "")
	if dir := instanceDir("/Users/test"); dir != "/Users/test/.gimbal/desktop" {
		t.Fatalf("default instanceDir = %q", dir)
	}
	t.Setenv("GIMBAL_INSTANCE_DIR", "/tmp/elsewhere")
	if dir := instanceDir("/Users/test"); dir != "/tmp/elsewhere" {
		t.Fatalf("overridden instanceDir = %q", dir)
	}
}

func TestServerEnvironmentPreservesPATHAndAddsUserTools(t *testing.T) {
	t.Setenv("PATH", "/custom/bin")
	path := ""
	for _, entry := range serverEnvironment("/Users/test") {
		if after, ok := strings.CutPrefix(entry, "PATH="); ok {
			path = after
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
