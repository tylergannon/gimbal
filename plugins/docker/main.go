// Docker environment plugin. Build with the same Go toolchain as gimbal:
// go build -buildmode=plugin -o bin/docker.so ./plugins/docker
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
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
func Start(ctx context.Context, raw json.RawMessage) (string, error) {
	c, err := parse(raw)
	if err != nil {
		return "", err
	}
	args := []string{"run", "-d", "--init", "--name", c.Name, "-p", fmt.Sprintf("127.0.0.1:%d:8081", c.Port)}
	for _, mount := range c.Mounts {
		args = append(args, "--mount", mount)
	}
	// Only explicitly named environment variables are passed; no host login state.
	for _, name := range c.Env {
		if strings.Contains(name, "=") {
			return "", fmt.Errorf("environment entries must name variables, not embed secrets")
		}
		args = append(args, "-e", name)
	}
	args = append(args, c.Image, "daemon", "--listen", "0.0.0.0:8081")
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("docker run: %w: %s", err, out)
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", c.Port)
	client := &http.Client{Timeout: time.Second}
	for range 100 {
		r, err := client.Get(endpoint + "/health")
		if err == nil {
			_ = r.Body.Close()
			if r.StatusCode == 200 {
				return endpoint, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("environment %s started but daemon is unreachable; inspect it or stop it with environment stop", c.Name)
}
func Stop(ctx context.Context, raw json.RawMessage) error {
	c, err := parse(raw)
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", c.Name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker remove: %w: %s", err, out)
	}
	return nil
}
