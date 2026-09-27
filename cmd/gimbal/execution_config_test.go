package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerExecutionConfigBuildsInstanceOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "execution.json")
	data := []byte(`{"environment":"dev","docker_image":"gimbal-worker:local","temporal_address":"127.0.0.1:7233","postgres_dsn":"postgres://gimbal@127.0.0.1/gimbal","secret_files":{"OPENAI_API_KEY":"/secrets/openai-key"},"mounts":["/inputs/shared"]}`)
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
	path := filepath.Join(t.TempDir(), "execution.json")
	if err := os.WriteFile(path, []byte(`{"environment":"dev"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := serverFlags{executionConfig: path}
	if _, err := flags.options(); err == nil {
		t.Fatal("incomplete execution config was accepted")
	}
}
