package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestNativePluginRepeatedAdmission(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skip("native Go plugins are unavailable on this platform")
	}
	pluginDir := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	args := []string{"build", "-buildmode=plugin", "-o", filepath.Join(pluginDir, "workflow.so")}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "-race" && setting.Value == "true" {
				args = append(args, "-race")
			}
		}
	}
	build := exec.CommandContext(ctx, "go", append(args, "./testdata/nativeplugin")...)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native plugin: %v\n%s", err, output)
	}
	dirs := []string{pluginDir}
	t.Run("startup then same-process admission", func(t *testing.T) {
		if err := invocation.LoadPlugins(dirs); err != nil {
			t.Fatalf("startup: %v", err)
		}
		project := t.TempDir()
		response, err := Spawn(invocation.Request{
			Workflow: "native-admission-test", PluginDirs: dirs, Input: json.RawMessage(`{}`),
			Project: project, WorkDir: project,
		})
		if err != nil || response.RunID == "" {
			t.Fatalf("same-process admission: %+v %v", response, err)
		}
		deadline := time.After(5 * time.Second)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			data, err := os.ReadFile(filepath.Join(project, "executed"))
			if err == nil && string(data) == "native workflow ran" {
				// Wait for the runner to finish using the temporary project.
				_, err = os.Stat(filepath.Join(project, ".gimbal", "invocations", response.RunID+".json"))
				_, controllerErr := Connect(filepath.Join(project, ".gimbal"), response.RunID)
				if os.IsNotExist(err) && controllerErr != nil {
					break
				}
			}
			select {
			case <-deadline:
				t.Fatal("admitted native workflow did not finish executing")
			case <-ticker.C:
			}
		}
		snapshot, err := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(response.RunID)
		if err != nil || snapshot.Run.Status != observation.StatusCompleted {
			t.Fatalf("native workflow completion: %+v %v", snapshot.Run, err)
		}
	})
	t.Run("separate registry and concurrent repeated loads", func(t *testing.T) {
		registry := &gimbal.Registry{}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if err := registry.LoadPlugins([]string{pluginDir, pluginDir}); err != nil {
					t.Errorf("load: %v", err)
				}
			})
		}
		wg.Wait()
		if _, ok := registry.Workflow("native-admission-test"); !ok || len(registry.Catalog()) != 1 {
			t.Fatal("plugin was not registered independently")
		}
		alias := t.TempDir()
		if err := os.Symlink(filepath.Join(pluginDir, "workflow.so"), filepath.Join(alias, "alias.so")); err != nil {
			t.Fatal(err)
		}
		if err := registry.LoadPlugins([]string{alias}); err != nil {
			t.Fatalf("same plugin through symlink: %v", err)
		}
	})
	t.Run("conflicting definition still rejected on retry", func(t *testing.T) {
		registry := &gimbal.Registry{}
		definition, ok := gimbal.DefaultRegistry.Workflow("native-admission-test")
		if !ok {
			t.Fatal("startup did not register plugin")
		}
		definition.Summary = "conflicting definition"
		if err := registry.RegisterWorkflow(definition); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			err := registry.LoadPlugins(dirs)
			if err == nil || !strings.Contains(err.Error(), `duplicate workflow "native-admission-test"`) {
				t.Fatalf("conflict: %v", err)
			}
		}
		got, _ := registry.Workflow(definition.Name)
		if got.Summary != definition.Summary {
			t.Fatal("plugin replaced conflicting definition")
		}
	})
}
