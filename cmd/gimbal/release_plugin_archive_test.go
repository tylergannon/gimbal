package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReleaseCLIPlugin(t *testing.T) {
	archive := os.Getenv("GIMBAL_RELEASE_ARCHIVE")
	if archive == "" {
		t.Skip("set GIMBAL_RELEASE_ARCHIVE to the native release CLI archive")
	}
	if !((runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") || (runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"))) {
		t.Fatalf("unsupported release platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join(cwd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(work, "gimbal")
	runReleaseCommand(t, work, "tar", "-xzf", archive, "-C", work, "gimbal")
	source, consumer := filepath.Join(work, "source"), filepath.Join(work, "consumer")
	runReleaseCommand(t, repo, "git", "clone", "--shared", "--no-hardlinks", "--no-checkout", repo, source)
	runReleaseCommand(t, repo, "git", "-C", source, "checkout", "--detach", "HEAD")
	tag := strings.TrimSpace(string(runReleaseCommand(t, source, "git", "describe", "--tags", "--exact-match", "HEAD")))
	if !strings.HasPrefix(tag, "v") {
		t.Fatalf("plugin source is not a release tag: %q", tag)
	}
	if err := os.MkdirAll(consumer, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(fmt.Sprintf("module example.com/gimbal-release-plugin-fixture\n\ngo 1.27.2\n\nrequire github.com/tylergannon/gimbal %s\n", tag)), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(source, "plugins", "messaging-filesystem"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, "plugins", "messaging-filesystem", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(consumer, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runReleaseCommand(t, work, "go", "work", "init", source, consumer)
	workspace, pluginDir := filepath.Join(work, "go.work"), filepath.Join(work, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildmode=plugin", "-o", filepath.Join(pluginDir, "filesystem.so"), ".")
	build.Dir = consumer
	build.Env = releasePluginEnv(workspace)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build external plugin: %v\n%s", err, output)
	}

	dataDir, route := filepath.Join(work, "messages"), filepath.Join(work, "route.json")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(route, []byte(fmt.Sprintf("{\"Name\":\"filesystem\",\"Configuration\":{\"Directory\":%q}}", dataDir)), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, cli, "message-worker", "--plugin-dir", pluginDir, "--task-route", "@"+route)
	cmd.Dir = work
	cmd.Env = []string{"HOME=" + filepath.Join(work, "home"), "TMPDIR=" + work, "DOPPLER_TOKEN=", "PATH=" + os.Getenv("PATH")}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	lines := make(chan string, 1)
	go func() {
		defer close(lines)
		for scanner := bufio.NewScanner(stdout); scanner.Scan(); {
			lines <- scanner.Text()
		}
	}()
	for {
		select {
		case line, open := <-lines:
			if !open {
				select {
				case err := <-wait:
					t.Fatalf("archived CLI exited before plugin startup: %v; stderr: %s", err, stderr.String())
				case <-ctx.Done():
					t.Fatalf("timed out waiting for plugin startup: %s", stderr.String())
				}
			}
			if line != "Messaging worker: filesystem" {
				continue
			}
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-wait:
				if err != nil {
					t.Fatalf("archived CLI failed to stop after plugin startup: %v; stderr: %s", err, stderr.String())
				}
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				<-wait
				t.Fatal("archived CLI did not stop after interrupt")
			}
			return
		case err := <-wait:
			t.Fatalf("archived CLI exited before plugin startup: %v; stderr: %s", err, stderr.String())
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-wait
			t.Fatalf("timed out waiting for plugin startup: %s", stderr.String())
		}
	}
}

func runReleaseCommand(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return output
}

func releasePluginEnv(workspace string) []string {
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOTOOLCHAIN=") || strings.HasPrefix(entry, "CGO_ENABLED=") || strings.HasPrefix(entry, "GOWORK=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOTOOLCHAIN=go1.27.2", "CGO_ENABLED=1", "GOWORK="+workspace)
}
