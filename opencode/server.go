package opencode

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	stateEnv       = "GIMBLE_OPENCODE_DIR"
	stateFileName  = "server.json"
	lockFileName   = "server.lock"
	serverLogName  = "server.log"
	startupTimeout = 15 * time.Second
	stopTimeout    = 5 * time.Second
)

// ServerInfo is the non-secret discovery information reported by lifecycle
// commands. Started is false when an existing healthy server was reused.
type ServerInfo struct {
	URL     string
	PID     int
	Started bool
}

type serverSecret struct {
	Username string
	Password string
}

type serverState struct {
	PID          int    `json:"pid"`
	ProcessToken string `json:"process_token"`
	URL          string `json:"url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

// DefaultStateDir returns the shared OpenCode state directory. The
// GIMBLE_OPENCODE_DIR environment variable overrides ~/.gimble/opencode.
func DefaultStateDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(stateEnv)); configured != "" {
		return filepath.Abs(configured)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".gimble", "opencode"), nil
}

// StartServer starts the shared server or returns the healthy server already
// recorded in stateDir. An empty stateDir uses DefaultStateDir.
func StartServer(ctx context.Context, stateDir string) (ServerInfo, error) {
	info, _, started, err := ensureServer(ctx, stateDir)
	info.Started = started
	return info, err
}

// StopServer terminates the server owned by stateDir. It returns false when no
// owned server exists; it never starts one as part of cleanup.
func StopServer(ctx context.Context, stateDir string) (bool, error) {
	dir, err := resolveStateDir(stateDir)
	if err != nil {
		return false, err
	}
	if err := prepareStateDir(dir); err != nil {
		return false, err
	}
	lock, err := acquireFileLock(ctx, filepath.Join(dir, lockFileName))
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Close() }()

	state, err := readServerState(dir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	current, alive, err := processIdentity(state.PID)
	if err != nil {
		return false, fmt.Errorf("inspect OpenCode process %d: %w", state.PID, err)
	}
	if !alive {
		if err := removeServerState(dir); err != nil {
			return false, err
		}
		return false, nil
	}
	if current != state.ProcessToken {
		return false, fmt.Errorf("refusing to stop pid %d: OpenCode state is stale and the pid now belongs to another process", state.PID)
	}
	if err := signalProcessGroup(state.PID, syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return false, fmt.Errorf("stop OpenCode pid %d: %w", state.PID, err)
	}
	if !waitForProcessExit(ctx, state.PID, state.ProcessToken, stopTimeout) {
		if err := signalProcessGroup(state.PID, syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return false, fmt.Errorf("kill OpenCode pid %d: %w", state.PID, err)
		}
		if !waitForProcessExit(ctx, state.PID, state.ProcessToken, stopTimeout) {
			return false, fmt.Errorf("OpenCode pid %d did not exit", state.PID)
		}
	}
	if err := removeServerState(dir); err != nil {
		return false, err
	}
	return true, nil
}

func ensureServer(ctx context.Context, stateDir string) (ServerInfo, serverSecret, bool, error) {
	dir, err := resolveStateDir(stateDir)
	if err != nil {
		return ServerInfo{}, serverSecret{}, false, err
	}
	if err := prepareStateDir(dir); err != nil {
		return ServerInfo{}, serverSecret{}, false, err
	}
	lock, err := acquireFileLock(ctx, filepath.Join(dir, lockFileName))
	if err != nil {
		return ServerInfo{}, serverSecret{}, false, err
	}
	defer func() { _ = lock.Close() }()

	if state, err := readServerState(dir); err == nil {
		current, alive, identityErr := processIdentity(state.PID)
		if identityErr != nil {
			return ServerInfo{}, serverSecret{}, false, fmt.Errorf("inspect OpenCode process %d: %w", state.PID, identityErr)
		}
		if alive && current != state.ProcessToken {
			return ServerInfo{}, serverSecret{}, false, fmt.Errorf("OpenCode state is stale: pid %d now belongs to another process", state.PID)
		}
		if alive {
			if err := waitForHealth(ctx, state, startupTimeout); err != nil {
				return ServerInfo{}, serverSecret{}, false, fmt.Errorf("discover existing OpenCode server: %w", err)
			}
			return publicInfo(state), secretInfo(state), false, nil
		}
		if err := removeServerState(dir); err != nil {
			return ServerInfo{}, serverSecret{}, false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ServerInfo{}, serverSecret{}, false, err
	}

	state, cmd, err := launchServer(dir)
	if err != nil {
		return ServerInfo{}, serverSecret{}, false, err
	}
	if err := writeServerState(dir, state); err != nil {
		_ = signalProcessGroup(state.PID, syscall.SIGTERM)
		return ServerInfo{}, serverSecret{}, false, err
	}
	go func() { _ = cmd.Wait() }()
	if err := waitForHealth(ctx, state, startupTimeout); err != nil {
		_ = signalProcessGroup(state.PID, syscall.SIGTERM)
		if !waitForProcessExit(context.Background(), state.PID, state.ProcessToken, stopTimeout) {
			_ = signalProcessGroup(state.PID, syscall.SIGKILL)
		}
		_ = removeServerState(dir)
		return ServerInfo{}, serverSecret{}, false, err
	}
	return publicInfo(state), secretInfo(state), true, nil
}

func launchServer(stateDir string) (serverState, *exec.Cmd, error) {
	executable, err := exec.LookPath("opencode")
	if err != nil {
		return serverState{}, nil, fmt.Errorf("find opencode executable: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return serverState{}, nil, fmt.Errorf("reserve OpenCode port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return serverState{}, nil, fmt.Errorf("release OpenCode port: %w", err)
	}
	password, err := randomPassword()
	if err != nil {
		return serverState{}, nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(stateDir, serverLogName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return serverState{}, nil, fmt.Errorf("open OpenCode server log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(executable, "serve", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port), "--no-mdns")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = environmentWith(os.Environ(), map[string]string{
		"OPENCODE_SERVER_USERNAME": "gimble",
		"OPENCODE_SERVER_PASSWORD": password,
	})
	prepareServerProcess(cmd)
	if err := cmd.Start(); err != nil {
		return serverState{}, nil, fmt.Errorf("start OpenCode server: %w", err)
	}
	token, alive, err := processIdentity(cmd.Process.Pid)
	if err != nil || !alive {
		_ = cmd.Process.Kill()
		if err != nil {
			return serverState{}, nil, fmt.Errorf("identify OpenCode process: %w", err)
		}
		return serverState{}, nil, errors.New("OpenCode process exited during startup")
	}
	return serverState{
		PID:          cmd.Process.Pid,
		ProcessToken: token,
		URL:          fmt.Sprintf("http://127.0.0.1:%d", port),
		Username:     "gimble",
		Password:     password,
	}, cmd, nil
}

func waitForHealth(ctx context.Context, state serverState, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := health(ctx, state); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("OpenCode server did not become healthy: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func health(ctx context.Context, state serverState) error {
	requestCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, state.URL+"/global/health", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(state.Username, state.Password)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %s", response.Status)
	}
	var result struct {
		Healthy bool   `json:"healthy"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Healthy {
		return errors.New("health returned unhealthy")
	}
	return nil
}

func resolveStateDir(stateDir string) (string, error) {
	if strings.TrimSpace(stateDir) == "" {
		return DefaultStateDir()
	}
	absolute, err := filepath.Abs(stateDir)
	if err != nil {
		return "", fmt.Errorf("resolve OpenCode state directory: %w", err)
	}
	return absolute, nil
}

func prepareStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create OpenCode state directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("protect OpenCode state directory: %w", err)
	}
	return nil
}

func readServerState(dir string) (serverState, error) {
	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return serverState{}, err
	}
	var state serverState
	if err := json.Unmarshal(data, &state); err != nil {
		return serverState{}, fmt.Errorf("read OpenCode state: %w", err)
	}
	if state.PID <= 0 || state.ProcessToken == "" || state.URL == "" || state.Username == "" || state.Password == "" {
		return serverState{}, errors.New("OpenCode state is incomplete")
	}
	return state, nil
}

func writeServerState(dir string, state serverState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode OpenCode state: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".server-*.json")
	if err != nil {
		return fmt.Errorf("create OpenCode state: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("protect OpenCode state: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write OpenCode state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync OpenCode state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close OpenCode state: %w", err)
	}
	if err := os.Rename(tempName, filepath.Join(dir, stateFileName)); err != nil {
		return fmt.Errorf("publish OpenCode state: %w", err)
	}
	return nil
}

func removeServerState(dir string) error {
	err := os.Remove(filepath.Join(dir, stateFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove OpenCode state: %w", err)
	}
	return nil
}

func randomPassword() (string, error) {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", fmt.Errorf("generate OpenCode server password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func publicInfo(state serverState) ServerInfo {
	return ServerInfo{URL: state.URL, PID: state.PID}
}

func secretInfo(state serverState) serverSecret {
	return serverSecret{Username: state.Username, Password: state.Password}
}

func environmentWith(base []string, values map[string]string) []string {
	result := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if _, replace := values[name]; !replace {
			result = append(result, entry)
		}
	}
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}
