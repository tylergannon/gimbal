package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

// A remote task daemon runs the same maintained directory helper as the CLI.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "desktop-environment" {
		var in environments.HelperRequest
		if err := json.Unmarshal([]byte(os.Args[2]), &in); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		out, err := environments.RunHelper(context.Background(), in)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestProspectiveRemoteDirectoryOperationsIgnoreInvalidConfigSiblings(t *testing.T) {
	owner := host.New(t.Context(), t.TempDir())
	defer owner.Close()
	p, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := owner.Environments()
	if err != nil {
		t.Fatal(err)
	}
	daemon := taskdaemon.New()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	machine := map[string]any{"id": "valid", "name": "Valid remote", "kind": "address", "address": server.URL, "executable": os.Args[0], "future": map[string]any{"keep": []any{1, "opaque"}}}
	raw, err := json.Marshal(map[string]any{"version": 1, "machines": []any{machine, map[string]any{"id": 5, "kind": "address", "address": "http://invalid:8081"}}, "environments": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	document, err := service.SaveConfig(t.Context(), environments.ConfigEdit{Text: string(raw)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot(t.Context(), p.ID())
	if err != nil || len(snapshot.Issues) == 0 {
		t.Fatalf("invalid sibling should remain visible: %+v %v", snapshot, err)
	}
	directory := t.TempDir()
	child := filepath.Join(directory, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	ctx := host.WithOwner(t.Context(), owner)
	location := EnvironmentLocation{Project: p.ID(), Machine: "valid", Directory: directory}
	facts, err := inspectEnvironment(ctx, location)
	if err != nil {
		t.Fatal(err)
	}
	realDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if !facts.Exists || facts.Path != realDirectory {
		t.Fatalf("wrong directory inspected: %+v", facts)
	}
	listing, err := listEnvironmentDirectories(ctx, location)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "child" {
		t.Fatalf("wrong directory listing: %+v", listing)
	}

	source := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	git("init", "-q", "-b", "main")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "initial")
	destination := filepath.Join(directory, "cloned")
	cloned, err := cloneEnvironment(ctx, app.RequestEvent[params.Params]{}, CloneEnvironment{Project: p.ID(), Machine: "valid", Directory: destination, Origin: source, Operation: "isolated-sibling-clone"})
	if err != nil {
		t.Fatal(err)
	}
	if !cloned.Exists || !cloned.Git || cloned.Path != filepath.Join(realDirectory, "cloned") {
		t.Fatalf("wrong clone: %+v", cloned)
	}
	after, err := service.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Text != document.Text || after.Revision != document.Revision {
		t.Fatal("directory operations changed opaque configuration or invalid siblings")
	}
}
