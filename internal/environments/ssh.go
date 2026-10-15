package environments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type tunnel struct {
	cmd      *exec.Cmd
	endpoint string
	done     chan struct{}
	stderr   bytes.Buffer
}

func (t *tunnel) close() error {
	select {
	case <-t.done:
		return nil
	default:
	}
	e := t.cmd.Process.Kill()
	select {
	case <-t.done:
	case <-time.After(2 * time.Second):
		return fmt.Errorf("SSH tunnel did not exit")
	}
	return e
}
func sshArgs(m Machine) []string {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "ExitOnForwardFailure=yes"}
	if m.SSH.Port != 0 {
		args = append(args, "-p", strconv.Itoa(m.SSH.Port))
	}
	return args
}
func shellQuote(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
func (s *Service) sshEndpoint(ctx context.Context, m Machine, bootstrap bool) (string, error) {
	s.sshMu.Lock()
	defer s.sshMu.Unlock()
	if m.SSH == nil {
		return "", fmt.Errorf("SSH host required")
	}
	raw, _ := json.Marshal(m)
	key := string(raw)
	s.mu.Lock()
	existing := s.tunnels[key]
	s.mu.Unlock()
	if existing != nil {
		select {
		case <-existing.done:
		default:
			if bootstrap && health(ctx, existing.endpoint) != nil {
				return s.bootstrapSSH(ctx, m, existing.endpoint)
			}
			return existing.endpoint, nil
		}
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return "", e
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	daemonPort := m.SSH.DaemonPort
	if daemonPort == 0 {
		daemonPort = 8081
	}
	args := sshArgs(m)
	args = append(args, "-N", "-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", port, daemonPort), "--", m.SSH.Host)
	t := &tunnel{endpoint: fmt.Sprintf("http://127.0.0.1:%d", port), done: make(chan struct{})}
	t.cmd = exec.CommandContext(s.ctx, "ssh", args...)
	t.cmd.Stderr = &t.stderr
	if e = t.cmd.Start(); e != nil {
		return "", fmt.Errorf("connect with ssh %s in Terminal: %w", m.SSH.Host, e)
	}
	go func() { _ = t.cmd.Wait(); close(t.done) }()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = t.close()
		return "", fmt.Errorf("environment service closed")
	}
	s.tunnels[key] = t
	s.mu.Unlock()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(75 * time.Millisecond)
	defer tick.Stop()
	for {
		if health(ctx, t.endpoint) == nil {
			return t.endpoint, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-t.done:
			return "", fmt.Errorf("SSH connection failed; run ssh %s in Terminal", m.SSH.Host)
		case <-deadline.C:
			if !bootstrap {
				return "", fmt.Errorf("daemon unreachable; reconnect or run ssh %s", m.SSH.Host)
			}
			return s.bootstrapSSH(ctx, m, t.endpoint)
		case <-tick.C:
		}
	}
}
func (s *Service) bootstrapSSH(ctx context.Context, m Machine, endpoint string) (string, error) {
	executable := m.Executable
	if executable == "" {
		executable = "gimbal"
	}
	port := m.SSH.DaemonPort
	if port == 0 {
		port = 8081
	}
	command := "nohup " + shellQuote(executable) + " daemon --listen " + shellQuote("127.0.0.1:"+strconv.Itoa(port))
	if m.SSH.WorkspaceRoot != "" {
		command += " --workspace-root " + shellQuote(m.SSH.WorkspaceRoot)
	}
	command += " </dev/null >/dev/null 2>&1 &"
	args := append(sshArgs(m), "--", m.SSH.Host, command)
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if e := exec.CommandContext(runCtx, "ssh", args...).Run(); e != nil {
		return "", fmt.Errorf("could not start installed Gimbal; run ssh %s in Terminal: %w", m.SSH.Host, e)
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if health(runCtx, endpoint) == nil {
			return endpoint, nil
		}
		select {
		case <-runCtx.Done():
			return "", fmt.Errorf("remote Gimbal did not become ready; run ssh %s and check gimbal is installed: %w", m.SSH.Host, runCtx.Err())
		case <-tick.C:
		}
	}
}
func health(ctx context.Context, endpoint string) error {
	c, cancel := context.WithTimeout(ctx, 600*time.Millisecond)
	defer cancel()
	req, e := http.NewRequestWithContext(c, "GET", endpoint+"/health", nil)
	if e != nil {
		return e
	}
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		return e
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	if response.StatusCode != 200 {
		return fmt.Errorf("health HTTP %d", response.StatusCode)
	}
	return nil
}
func (s *Service) probe(ctx context.Context, m Machine, bootstrap bool) error {
	if m.ID == Local {
		return nil
	}
	started := time.Now()
	var e error
	if m.Kind == KindRoute {
		if !bootstrap {
			return nil
		}
		_, e = s.operation(ctx, Target{Machine: m}, HelperRequest{Operation: "inspect"}, "")
		if e != nil {
			s.setMachineError(m, e)
		}
		return e
	}
	endpoint := m.Address
	if m.Kind == KindSSH {
		endpoint, e = s.sshEndpoint(ctx, m, bootstrap)
	}
	if e == nil {
		e = health(ctx, strings.TrimRight(endpoint, "/"))
	}
	s.mu.Lock()
	previous := s.states[m.ID]
	state := previous
	state.ID = m.ID
	state.Name = m.Name
	state.Kind = m.Kind
	state.Detail = "Connected"
	state.LatencyMS = time.Since(started).Milliseconds()
	state.State = StateConnected
	if e != nil {
		state.State = StateUnreachable
		state.Detail = e.Error()
	}
	if previous.State != state.State {
		state.Since = time.Now().UnixMilli()
	}
	s.states[m.ID] = state
	s.mu.Unlock()
	if e == nil && bootstrap {
		_, e = s.operation(ctx, Target{Machine: m}, HelperRequest{Operation: "inspect"}, "")
		if e != nil {
			s.setMachineError(m, e)
		}
	}
	return e
}
func (s *Service) setMachineError(m Machine, e error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[m.ID]
	st.ID = m.ID
	st.Name = m.Name
	st.Kind = m.Kind
	if st.State != StateError {
		st.Since = time.Now().UnixMilli()
	}
	st.State = StateError
	st.Detail = e.Error()
	s.states[m.ID] = st
}

// SSHHosts returns literal aliases only. Wildcards and Match blocks are not
// suggested; typed aliases still use system SSH's complete configuration.
func SSHHosts(ctx context.Context) ([]string, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return nil, e
	}
	return sshHosts(ctx, filepath.Join(home, ".ssh", "config"))
}
func sshHosts(ctx context.Context, path string) ([]string, error) {
	seen := map[string]bool{}
	hosts := map[string]bool{}
	var read func(string, int) error
	read = func(path string, depth int) error {
		if depth > 8 || seen[path] {
			return nil
		}
		seen[path] = true
		if e := ctx.Err(); e != nil {
			return e
		}
		b, e := os.ReadFile(path)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil {
			return e
		}
		if len(b) > 1<<20 {
			return fmt.Errorf("SSH config is too large")
		}
		for _, line := range strings.Split(string(b), "\n") {
			line, _, _ = strings.Cut(line, "#")
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			switch strings.ToLower(fields[0]) {
			case "host":
				for _, name := range fields[1:] {
					if !strings.ContainsAny(name, "*?!") && !strings.HasPrefix(name, "-") {
						hosts[name] = true
					}
				}
			case "include":
				for _, pattern := range fields[1:] {
					if strings.HasPrefix(pattern, "~/") {
						home, _ := os.UserHomeDir()
						pattern = filepath.Join(home, strings.TrimPrefix(pattern, "~/"))
					} else if !filepath.IsAbs(pattern) {
						pattern = filepath.Join(filepath.Dir(path), pattern)
					}
					paths, _ := filepath.Glob(pattern)
					for _, p := range paths {
						if e := read(p, depth+1); e != nil {
							return e
						}
					}
				}
			}
		}
		return nil
	}
	if e := read(path, 0); e != nil {
		return nil, e
	}
	out := []string{}
	for h := range hosts {
		out = append(out, h)
	}
	sort.Strings(out)
	return out, nil
}
