package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tylergannon/gimbal/backend"
)

func init() {
	definition := BackendDefinition{Name: "http", ConfigurationSchema: json.RawMessage(`{"type":"object","properties":{"endpoint":{"type":"string"}},"required":["endpoint"],"additionalProperties":false}`), ValidateConfiguration: func(raw json.RawMessage) error {
		var config struct {
			Endpoint string `json:"endpoint"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&config); err != nil {
			return err
		}
		u, err := url.Parse(config.Endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("backend requires an HTTP endpoint")
		}
		return nil
	}}
	definition.Connect = func(ctx context.Context, raw json.RawMessage) (backend.Connection, error) {
		if err := definition.ValidateConfiguration(raw); err != nil {
			return backend.Connection{}, err
		}
		var config struct {
			Endpoint string `json:"endpoint"`
		}
		_ = json.Unmarshal(raw, &config)
		h := backend.HTTP{Endpoint: config.Endpoint}
		return backend.Connection{Execution: h, Events: h, Observations: h}, nil
	}
	if err := DefaultRegistry.RegisterBackend(definition); err != nil {
		panic(err)
	}
}
