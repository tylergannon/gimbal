package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerExecutionConfigBuildsInstanceOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "execution.json")
	data := []byte(`{"environment":"dev","docker_image":"gimbal-browser-evaluator:local","worker_binary":"/opt/bin/gimbal-worker","temporal_address":"127.0.0.1:7233","postgres_dsn":"postgres://gimbal@127.0.0.1/gimbal","secret_files":{"OPENAI_API_KEY":"/secrets/openai-key"},"mounts":["/inputs/shared"]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := serverFlags{noWeb: true, executionConfig: path}
	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 {
		t.Fatalf("instance options = %d, want no-web and execution backend options", len(options))
	}
	if !isOrdinaryCLI([]string{"--execution-config", path}) {
		t.Fatal("execution config invocation would be routed to the vet analyzer")
	}
}

func TestServerExecutionConfigRequiresWorkerSettings(t *testing.T) {
	for name, config := range map[string]string{
		"only environment":      `{"environment":"dev"}`,
		"missing worker_binary": `{"environment":"dev","docker_image":"gimbal-browser-evaluator:local","temporal_address":"127.0.0.1:7233","postgres_dsn":"postgres://gimbal@127.0.0.1/gimbal"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "execution.json")
			if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			flags := serverFlags{executionConfig: path}
			_, err := flags.options()
			if err == nil || !strings.Contains(err.Error(), "worker_binary") {
				t.Fatalf("incomplete execution config: err = %v, want the required fields named", err)
			}
		})
	}
}
