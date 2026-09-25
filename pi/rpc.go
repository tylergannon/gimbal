package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

type process struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	writeMu   sync.Mutex
	mu        sync.Mutex
	pending   map[string]chan response
	turn      *turn
	done      chan struct{}
	readDone  chan struct{}
	waitErr   error
	seq       atomic.Uint64
	closeOnce sync.Once
}

type response struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

type wireRecord struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	Command  string          `json:"command"`
	Success  bool            `json:"success"`
	Error    string          `json:"error"`
	Data     json.RawMessage `json:"data"`
	Message  json.RawMessage `json:"message"`
	Usage    json.RawMessage `json:"usage"`
	Event    json.RawMessage `json:"assistantMessageEvent"`
	ToolID   string          `json:"toolCallId"`
	ToolName string          `json:"toolName"`
	Args     json.RawMessage `json:"args"`
	Result   json.RawMessage `json:"result"`
	IsError  bool            `json:"isError"`
}

func modelID(model string) string {
	if strings.Contains(model, "/") {
		parts := strings.SplitN(model, "/", 2)
		if parts[0] == provider {
			return parts[1]
		}
		return model
	}
	return model
}

func replaceEnv(env []string, name, value string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

func (p *process) command(ctx context.Context, value any) (response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id := fmt.Sprintf("gimbal-%d", p.seq.Add(1))
	command, _ := value.(map[string]any)
	command["id"] = id
	wait := make(chan response, 1)
	p.mu.Lock()
	select {
	case <-p.done:
		p.mu.Unlock()
		return response{}, fmt.Errorf("child exited: %w", p.waitError())
	default:
	}
	p.pending[id] = wait
	p.mu.Unlock()
	data, err := json.Marshal(value)
	if err != nil {
		p.dropPending(id)
		return response{}, err
	}
	p.writeMu.Lock()
	_, err = p.stdin.Write(append(data, '\n'))
	p.writeMu.Unlock()
	if err != nil {
		p.dropPending(id)
		return response{}, fmt.Errorf("write RPC command: %w", err)
	}
	select {
	case answer := <-wait:
		return answer, nil
	case <-ctx.Done():
		p.dropPending(id)
		return response{}, ctx.Err()
	case <-p.done:
		p.dropPending(id)
		return response{}, fmt.Errorf("child exited: %w", p.waitError())
	}
}

func (p *process) dropPending(id string) {
	p.mu.Lock()
	delete(p.pending, id)
	p.mu.Unlock()
}

func (p *process) read(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var record wireRecord
		if json.Unmarshal(line, &record) != nil {
			continue // diagnostic protocol damage must not hide a later settled result.
		}
		if record.Type == "response" {
			p.mu.Lock()
			wait := p.pending[record.ID]
			delete(p.pending, record.ID)
			p.mu.Unlock()
			if wait != nil {
				wait <- response{ID: record.ID, Type: record.Type, Command: record.Command, Success: record.Success, Error: record.Error, Data: record.Data}
			}
			continue
		}
		p.mu.Lock()
		active := p.turn
		p.mu.Unlock()
		if active != nil {
			active.record(record)
		}
	}
}

func (p *process) setTurn(turn *turn) {
	p.mu.Lock()
	p.turn = turn
	p.mu.Unlock()
}

func (p *process) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *process) waitError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waitErr == nil {
		return errors.New("process stopped")
	}
	return p.waitErr
}

func (p *process) close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	})
}

func (s *session) start(ctx context.Context, command, key string, resume bool) error {
	args := []string{"--mode", "rpc", "--provider", provider, "--model", modelID(s.model), "--session-dir", s.sessionDir, "--session-id", s.nativeID}
	cmd := exec.Command(command, args...)
	cmd.Dir = s.workdir
	cmd.Env = replaceEnv(os.Environ(), "PI_CODING_AGENT_DIR", s.dir)
	cmd.Env = replaceEnv(cmd.Env, "DIFFUSION_API_KEY", key)
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("pi: open RPC stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pi: open RPC stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pi: start RPC process: %w", err)
	}
	p := &process{cmd: cmd, stdin: stdin, pending: make(map[string]chan response), done: make(chan struct{}), readDone: make(chan struct{})}
	s.proc = p
	go func() {
		p.read(stdout)
		close(p.readDone)
	}()
	go func() {
		<-p.readDone
		err := cmd.Wait()
		p.mu.Lock()
		p.waitErr = err
		p.mu.Unlock()
		close(p.done)
	}()
	if resume {
		if _, err := p.command(ctx, map[string]any{"type": "get_state"}); err != nil {
			p.close()
			return fmt.Errorf("pi: resume native session: %w", err)
		}
	}
	return nil
}
