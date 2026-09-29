package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const dopplerSecretsURL = "https://api.doppler.com/v3/configs/config/secrets/download?format=json"

func needsSecrets(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return false
		}
	}
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "runs", "watch", "steer", "count-tokens":
		return false
	case "audit-index":
		return false
	case "opencode":
		return len(args) > 1 && args[1] == "start"
	default:
		return true
	}
}

func loadCommandSecrets(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory for Gimbal config: %w", err)
	}
	filename := filepath.Join(home, ".gimbal", "config.json")
	client := &http.Client{Timeout: 15 * time.Second}
	secrets, err := resolveSecrets(ctx, filename, os.LookupEnv, client, dopplerSecretsURL)
	if err != nil {
		return err
	}
	for name, value := range secrets {
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("invalid Gimbal secret name %q: %w", name, err)
		}
	}
	return nil
}

func resolveSecrets(ctx context.Context, filename string, lookup func(string) (string, bool), client *http.Client, endpoint string) (map[string]string, error) {
	local := map[string]string{}
	data, err := os.ReadFile(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read Gimbal config: %w", err)
	}
	if err == nil {
		var config struct {
			Secrets map[string]string `json:"secrets"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("parse Gimbal config: %w", err)
		}
		maps.Copy(local, config.Secrets)
	}

	token := local["DOPPLER_TOKEN"]
	if value, exists := lookup("DOPPLER_TOKEN"); exists {
		token = value
	}
	merged := map[string]string{}
	if token != "" {
		fetched, err := fetchDopplerSecrets(ctx, client, endpoint, token)
		if err != nil {
			return nil, err
		}
		maps.Copy(merged, fetched)
	}
	maps.Copy(merged, local)
	return merged, nil
}

func fetchDopplerSecrets(ctx context.Context, client *http.Client, endpoint, token string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build Doppler secrets request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Doppler secrets: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, &httpStatusError{Code: response.StatusCode}
	}
	var secrets map[string]string
	if err := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(&secrets); err != nil {
		return nil, fmt.Errorf("decode Doppler secrets: %w", err)
	}
	if secrets == nil {
		return nil, fmt.Errorf("decode Doppler secrets: expected an object")
	}
	return secrets, nil
}

type httpStatusError struct{ Code int }

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("fetch Doppler secrets: HTTP %d", e.Code)
}
