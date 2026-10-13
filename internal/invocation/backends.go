package invocation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/localorchestrator"
	"github.com/tylergannon/gimbal/backend/secretfiles"
)

func registerResourceBackends(registry *gimbal.Registry) error {
	if err := registry.RegisterOrchestrator(gimbal.OrchestratorDefinition{Name: "local", RequiresSharedSecrets: true, ValidateConfiguration: func(raw json.RawMessage) error {
		if err := localorchestrator.ValidateConfiguration(raw); err != nil {
			return err
		}
		var config localorchestrator.Config
		if err := json.Unmarshal(raw, &config); err != nil {
			return err
		}
		for name, environment := range config.Environments {
			definition, ok := registry.Messaging(environment.Route.Name)
			if !ok {
				return fmt.Errorf("environment %q: unknown messaging backend %q", name, environment.Route.Name)
			}
			if err := definition.ValidateConfiguration(environment.Route.Configuration); err != nil {
				return fmt.Errorf("environment %q messaging configuration: %w", name, err)
			}
		}
		return nil
	}, Construct: func(_ context.Context, raw json.RawMessage, secrets backend.Selection) (backend.OrchestratorBackend, error) {
		var config localorchestrator.Config
		if err := json.Unmarshal(raw, &config); err != nil {
			return nil, err
		}
		return localorchestrator.Open(config, secrets)
	}}); err != nil {
		return err
	}
	if err := registry.RegisterSecrets(gimbal.SecretsDefinition{Name: "file", SharedAcrossProcesses: true, ValidateConfiguration: func(raw json.RawMessage) error {
		var config struct{ Directory string }
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&config); err != nil {
			return err
		}
		if !filepath.IsAbs(config.Directory) {
			return fmt.Errorf("directory must be absolute")
		}
		return nil
	}, Construct: func(_ context.Context, raw json.RawMessage) (backend.SecretsBackend, error) {
		var config struct{ Directory string }
		if err := json.Unmarshal(raw, &config); err != nil {
			return nil, err
		}
		return secretfiles.Open(config.Directory)
	}}); err != nil {
		return err
	}
	return registry.RegisterSecrets(gimbal.SecretsDefinition{Name: "memory", ValidateConfiguration: func(raw json.RawMessage) error {
		if len(raw) == 0 || string(raw) == "null" {
			return nil
		}
		var config map[string]json.RawMessage
		if err := json.Unmarshal(raw, &config); err != nil {
			return err
		}
		if config == nil || len(config) > 0 {
			return fmt.Errorf("memory secrets configuration must be an empty object")
		}
		return nil
	}, Construct: func(context.Context, json.RawMessage) (backend.SecretsBackend, error) {
		return backend.NewMemorySecrets(), nil
	}})
}
