// Docker environment plugin. Build with the same Go toolchain as gimbal:
// go build -buildmode=plugin -o bin/docker.so ./plugins/docker
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

type config struct {
	Name, Image string
	Port        int
	Mounts, Env []string
}

func parse(raw json.RawMessage) (config, error) {
	var c config
	err := json.Unmarshal(raw, &c)
	if err == nil && (c.Name == "" || c.Image == "" || c.Port < 1) {
		err = fmt.Errorf("docker environment needs Name, Image and a host Port")
	}
	return c, err
}

// Start provisions only the environment. Task ownership and observation are
// implemented by the gimbal daemon inside it, never by this plugin.
func Start(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	c, err := parse(raw)
	if err != nil {
		return nil, err
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	label := hex.EncodeToString(token)
	args := []string{"run", "--label", "gimbal.owner=" + label, "-d", "--init", "--name", c.Name, "-p", fmt.Sprintf("127.0.0.1:%d:8081", c.Port)}
	for _, mount := range c.Mounts {
		args = append(args, "--mount", mount)
	}
	// Only explicitly named environment variables are passed; no host login state.
	for _, name := range c.Env {
		if strings.Contains(name, "=") {
			return nil, fmt.Errorf("environment entries must name variables, not embed secrets")
		}
		args = append(args, "-e", name)
	}
	args = append(args, c.Image, "daemon", "--listen", "0.0.0.0:8081")
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker run: %w: %s", err, out)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return nil, fmt.Errorf("docker run returned no container ID")
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", c.Port)
	receipt, _ := json.Marshal(ownership{ID: id, Label: label, Endpoint: endpoint})
	client := &http.Client{Timeout: time.Second}
	for range 100 {
		r, err := client.Get(endpoint + "/health")
		if err == nil {
			_ = r.Body.Close()
			if r.StatusCode == 200 {
				return receipt, nil
			}
		}
		select {
		case <-ctx.Done():
			return receipt, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return receipt, fmt.Errorf("environment %s started but daemon is unreachable; inspect it or stop it with environment stop", c.Name)
}

type ownership struct{ ID, Label, Endpoint string }

// Stop releases exactly the container described by Start's ownership receipt.
func Stop(ctx context.Context, raw json.RawMessage) error {
	var receipt ownership
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return err
	}
	if receipt.ID == "" || receipt.Label == "" {
		return fmt.Errorf("docker stop needs the ownership receipt returned by start")
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.Labels}}", receipt.ID).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "No such object") || strings.Contains(string(out), "No such container") {
			return nil
		}
		return fmt.Errorf("docker inspect: %w: %s", err, out)
	}
	var labels map[string]string
	if err := json.Unmarshal(out, &labels); err != nil {
		return fmt.Errorf("docker ownership: %w", err)
	}
	if labels["gimbal.owner"] != receipt.Label {
		return fmt.Errorf("docker ownership mismatch for %s", receipt.ID)
	}
	out, err = exec.CommandContext(ctx, "docker", "rm", "-f", receipt.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker remove: %w: %s", err, out)
	}
	return nil
}

// Register exposes the owned daemon receipt as a backend route after Start.
func Register(registry *gimbal.Registry) error {
	validate := func(raw json.RawMessage) error {
		var receipt ownership
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return err
		}
		u, err := url.Parse(receipt.Endpoint)
		if err != nil || u.Host == "" || u.Scheme != "http" || receipt.ID == "" || receipt.Label == "" {
			return fmt.Errorf("docker backend requires Start's ownership receipt")
		}
		return nil
	}
	return registry.RegisterBackend(gimbal.BackendDefinition{Name: "docker", ConfigurationSchema: json.RawMessage(`{"type":"object","properties":{"ID":{"type":"string"},"Label":{"type":"string"},"Endpoint":{"type":"string"}},"required":["ID","Label","Endpoint"],"additionalProperties":false}`), ValidateConfiguration: validate, Connect: func(ctx context.Context, raw json.RawMessage) (backend.Connection, error) {
		if err := validate(raw); err != nil {
			return backend.Connection{}, err
		}
		var receipt ownership
		_ = json.Unmarshal(raw, &receipt)
		http := backend.HTTP{Endpoint: receipt.Endpoint}
		return backend.Connection{Execution: http, Events: http, Observations: http}, nil
	}})
}
