package pi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
)

const provider = "diffusion"

type Adapter struct {
	mu       sync.Mutex
	sessions map[string]*session
	config   adapterConfig
}

var _ gimbal.HarnessAdapter = (*Adapter)(nil)

type adapterConfig struct {
	command  string
	endpoint string
	apiKey   func() string
}

type session struct {
	id, nativeID, model, workdir string
	dir, sessionDir              string
	mu                           sync.Mutex
	turnMu                       sync.Mutex
	steerMu                      sync.Mutex
	proc                         *process
	active                       *turn
	closed                       bool
}

// New returns a Pi RPC adapter. Each Gimbal session owns its Pi process,
// configuration directory, and native session directory.
func New() gimbal.HarnessAdapter { return newAdapter(adapterConfig{}) }

func newAdapter(config adapterConfig) *Adapter {
	if config.command == "" {
		config.command = "pi"
	}
	if config.endpoint == "" {
		config.endpoint = "https://router.diffusion.io/v1"
	}
	if config.apiKey == nil {
		config.apiKey = func() string { return os.Getenv("DIFFUSION_API_KEY") }
	}
	return &Adapter{sessions: make(map[string]*session), config: config}
}

func (a *Adapter) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	if effort != "" {
		return "", fmt.Errorf("pi: explicit effort %q is unsupported for this model", effort)
	}
	if strings.TrimSpace(model) == "" {
		return "", errors.New("pi: model is required")
	}
	key := a.config.apiKey()
	if key == "" {
		return "", errors.New("pi: DIFFUSION_API_KEY is required")
	}
	root, err := os.MkdirTemp("", "gimbal-pi-")
	if err != nil {
		return "", fmt.Errorf("pi: create session directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0700); err != nil {
		cleanup()
		return "", fmt.Errorf("pi: create agent directory: %w", err)
	}
	sessionID, err := randomID()
	if err != nil {
		cleanup()
		return "", err
	}
	if err := writeConfig(filepath.Join(root, "agent", "models.json"), a.config.endpoint, modelID(model)); err != nil {
		cleanup()
		return "", err
	}
	s := &session{id: sessionID, nativeID: sessionID, model: model, workdir: workdir,
		dir: filepath.Join(root, "agent"), sessionDir: filepath.Join(root, "sessions")}
	if err := os.MkdirAll(s.sessionDir, 0700); err != nil {
		cleanup()
		return "", fmt.Errorf("pi: create native session directory: %w", err)
	}
	if err := s.start(ctx, a.config.command, key, false); err != nil {
		cleanup()
		return "", err
	}
	a.mu.Lock()
	a.sessions[sessionID] = s
	a.mu.Unlock()
	return sessionID, nil
}

func writeConfig(path, endpoint, id string) error {
	entry := map[string]any{"id": id}
	// These limits come from the Router's model list. New IDs use Pi's
	// conservative defaults until their limits have been checked.
	switch id {
	case "deepseek-4.1-flash", "deepseek-4.1-flash-background":
		entry["contextWindow"], entry["maxTokens"] = 1048576, 262144
	case "glm-5.3-flash", "glm-5.3-flash-background":
		entry["contextWindow"], entry["maxTokens"] = 524288, 163840
	case "glm-5.2-vision", "glm-5.2-vision-background", "glm-5.2-vision-flex",
		"glm-5.3", "glm-5.3-background", "glm-5.3-vision", "glm-5.3-vision-background":
		entry["contextWindow"], entry["maxTokens"] = 524288, 131072
	}
	if _, known := entry["contextWindow"]; known {
		entry["input"] = []string{"text", "image"}
	}
	data, err := json.Marshal(struct {
		Providers map[string]any `json:"providers"`
	}{map[string]any{provider: map[string]any{
		"baseUrl": endpoint, "api": "openai-completions", "apiKey": "$DIFFUSION_API_KEY",
		"models": []any{entry},
	}}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("pi: write models config: %w", err)
	}
	return nil
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("pi: create session id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (a *Adapter) session(id string) (*session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[id]
	if s == nil {
		return nil, fmt.Errorf("pi: no session %q", id)
	}
	return s, nil
}

func (a *Adapter) RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	s.turnMu.Lock()
	defer s.turnMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return gimbal.TurnResult{}, fmt.Errorf("pi: session %q is closed", sessionID)
	}
	proc := s.proc
	if proc == nil || proc.exited() {
		if proc != nil {
			proc.close()
		}
		if err := s.start(ctx, a.config.command, a.config.apiKey(), true); err != nil {
			s.mu.Unlock()
			return gimbal.TurnResult{}, err
		}
		proc = s.proc
	}
	s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return gimbal.TurnResult{}, err
	}
	message := prompt
	if len(schema) > 0 {
		var parsed any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			return gimbal.TurnResult{}, fmt.Errorf("pi: decode output schema: %w", err)
		}
		raw, _ := json.MarshalIndent(parsed, "", "  ")
		message += "\n\nReturn one JSON value matching this JSON Schema. Do not wrap it in Markdown fences.\n" + string(raw)
	}
	run := newTurn(sessionID, s.model, onEvent)
	s.mu.Lock()
	s.active = run
	s.mu.Unlock()
	proc.setTurn(run)
	response, err := proc.command(ctx, map[string]any{"type": "prompt", "message": message})
	if err != nil {
		proc.setTurn(nil)
		if ctx.Err() != nil {
			a.cancelTurn(s, proc, run)
			return gimbal.TurnResult{}, ctx.Err()
		}
		s.clearActive(run)
		return gimbal.TurnResult{}, fmt.Errorf("pi: prompt: %w", err)
	}
	if !response.Success {
		proc.setTurn(nil)
		s.clearActive(run)
		return gimbal.TurnResult{}, fmt.Errorf("pi: prompt rejected: %s", response.Error)
	}
	select {
	case <-run.settled:
	case <-ctx.Done():
		a.cancelTurn(s, proc, run)
		return gimbal.TurnResult{}, ctx.Err()
	case <-proc.done:
		select {
		case <-run.settled:
			break
		default:
			s.clearActive(run)
			return gimbal.TurnResult{}, fmt.Errorf("pi: child exited before agent_settled: %w", proc.waitError())
		}
	}
	proc.setTurn(nil)
	s.finishTurn(proc, run)
	if run.err != nil {
		return gimbal.TurnResult{}, run.err
	}
	if run.final == nil {
		return gimbal.TurnResult{}, errors.New("pi: agent_settled without a final assistant message")
	}
	if run.final.Error != "" || run.final.StopReason == "error" {
		return gimbal.TurnResult{}, fmt.Errorf("pi: provider error: %s", run.final.Error)
	}
	text := assistantText(run.final.Content)
	if text == "" {
		return gimbal.TurnResult{}, errors.New("pi: final assistant message has no text output")
	}
	var output json.RawMessage
	if len(schema) == 0 {
		output, _ = json.Marshal(text)
	} else {
		output = json.RawMessage(extractJSON(text))
		if !json.Valid(output) {
			return gimbal.TurnResult{}, errors.New("pi: final assistant output does not contain valid JSON")
		}
	}
	return gimbal.TurnResult{Output: output, Usage: run.usage}, nil
}

func assistantText(content []contentBlock) string {
	var b strings.Builder
	for _, block := range content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	start, end := strings.IndexAny(text, "{["), -1
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '}' || text[i] == ']' {
			end = i
			break
		}
	}
	if start >= 0 && end >= start {
		return text[start : end+1]
	}
	return text
}

func (s *session) clearActive(run *turn) {
	s.steerMu.Lock()
	s.mu.Lock()
	if s.active == run {
		s.active = nil
	}
	s.mu.Unlock()
	s.steerMu.Unlock()
}

func (s *session) finishTurn(proc *process, run *turn) {
	s.steerMu.Lock()
	// Pi can settle before a concurrently accepted steer is drained. Clear the
	// native queue at this boundary so no message can spill into the next turn.
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	answer, err := proc.command(cleanup, map[string]any{"type": "clear_queue"})
	cancel()
	if err != nil || !answer.Success {
		s.discardProcess(proc)
	}
	s.mu.Lock()
	if s.active == run {
		s.active = nil
	}
	s.mu.Unlock()
	s.steerMu.Unlock()
}

func (a *Adapter) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return false, err
	}
	s.steerMu.Lock()
	defer s.steerMu.Unlock()
	s.mu.Lock()
	run, proc := s.active, s.proc
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return false, fmt.Errorf("pi: session %q is closed", sessionID)
	}
	if run == nil || proc == nil || proc.exited() {
		return false, nil
	}
	select {
	case <-run.settled:
		return false, nil
	default:
	}
	answer, err := proc.command(ctx, map[string]any{"type": "steer", "message": message})
	if err != nil {
		select {
		case <-run.settled:
			return false, nil
		default:
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, fmt.Errorf("pi: steer: %w", err)
	}
	if !answer.Success {
		return false, nil
	}
	select {
	case <-run.settled:
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		answer, clearErr := proc.command(cleanup, map[string]any{"type": "clear_queue"})
		cancel()
		if clearErr != nil || !answer.Success {
			s.discardProcess(proc)
		}
		return false, nil
	default:
		return true, nil
	}
}

func (s *session) discardProcess(proc *process) {
	proc.close()
	s.mu.Lock()
	if s.proc == proc {
		s.proc = nil
	}
	s.mu.Unlock()
}

func (a *Adapter) Fork(ctx context.Context, sessionID string) (string, error) {
	parent, err := a.session(sessionID)
	if err != nil {
		return "", err
	}
	parent.turnMu.Lock()
	defer parent.turnMu.Unlock()
	parent.mu.Lock()
	if parent.closed {
		parent.mu.Unlock()
		return "", fmt.Errorf("pi: session %q is closed", sessionID)
	}
	proc := parent.proc
	parent.mu.Unlock()
	if proc == nil || proc.exited() {
		return "", errors.New("pi: cannot fork an unavailable session")
	}
	state, err := proc.command(ctx, map[string]any{"type": "get_state"})
	if err != nil || !state.Success {
		if err == nil {
			err = fmt.Errorf("get_state rejected: %s", state.Error)
		}
		return "", fmt.Errorf("pi: read parent state: %w", err)
	}
	var native struct {
		SessionFile string `json:"sessionFile"`
		SessionID   string `json:"sessionId"`
	}
	if err := json.Unmarshal(state.Data, &native); err != nil || native.SessionFile == "" || native.SessionID == "" {
		return "", errors.New("pi: parent state did not include native session identity")
	}
	childID, err := randomID()
	if err != nil {
		return "", err
	}
	root, err := os.MkdirTemp("", "gimbal-pi-fork-")
	if err != nil {
		return "", fmt.Errorf("pi: create fork directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	dir := filepath.Join(root, "agent")
	childSessionDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		cleanup()
		return "", fmt.Errorf("pi: create fork agent directory: %w", err)
	}
	if err := os.MkdirAll(childSessionDir, 0700); err != nil {
		cleanup()
		return "", fmt.Errorf("pi: create fork session directory: %w", err)
	}
	if err := writeConfig(filepath.Join(dir, "models.json"), a.config.endpoint, modelID(parent.model)); err != nil {
		cleanup()
		return "", err
	}
	copyPath := filepath.Join(childSessionDir, filepath.Base(native.SessionFile))
	if err := copyFile(native.SessionFile, copyPath); err != nil {
		cleanup()
		return "", fmt.Errorf("pi: copy native session for fork: %w", err)
	}
	child := &session{id: childID, nativeID: native.SessionID, model: parent.model, workdir: parent.workdir, dir: dir, sessionDir: childSessionDir}
	if err := child.start(ctx, a.config.command, a.config.apiKey(), true); err != nil {
		cleanup()
		return "", err
	}
	cloned, err := child.proc.command(ctx, map[string]any{"type": "clone"})
	if err != nil || !cloned.Success {
		child.proc.close()
		cleanup()
		if err == nil {
			err = fmt.Errorf("clone rejected: %s", cloned.Error)
		}
		return "", fmt.Errorf("pi: clone parent conversation: %w", err)
	}
	if len(cloned.Data) > 0 {
		var result struct {
			Cancelled bool `json:"cancelled"`
		}
		_ = json.Unmarshal(cloned.Data, &result)
		if result.Cancelled {
			child.proc.close()
			cleanup()
			return "", errors.New("pi: clone was cancelled")
		}
	}
	clonedState, err := child.proc.command(ctx, map[string]any{"type": "get_state"})
	if err != nil || !clonedState.Success {
		child.proc.close()
		cleanup()
		if err == nil {
			err = fmt.Errorf("get_state rejected: %s", clonedState.Error)
		}
		return "", fmt.Errorf("pi: inspect cloned session: %w", err)
	}
	var childNative struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(clonedState.Data, &childNative); err != nil || childNative.SessionID == "" {
		child.proc.close()
		cleanup()
		return "", errors.New("pi: cloned state did not include native session identity")
	}
	child.nativeID = childNative.SessionID
	a.mu.Lock()
	a.sessions[childID] = child
	a.mu.Unlock()
	return childID, nil
}

func copyFile(from, to string) error {
	input, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func (a *Adapter) cancelTurn(s *session, proc *process, run *turn) {
	s.steerMu.Lock()
	defer s.steerMu.Unlock()
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	clear, clearErr := proc.command(cleanup, map[string]any{"type": "clear_queue"})
	abort, abortErr := proc.command(cleanup, map[string]any{"type": "abort"})
	settled := false
	if clearErr == nil && clear.Success && abortErr == nil && abort.Success {
		select {
		case <-run.settled:
			settled = true
		case <-cleanup.Done():
		}
	}
	cancel()
	proc.setTurn(nil)
	if clearErr != nil || !clear.Success || abortErr != nil || !abort.Success || !settled {
		s.discardProcess(proc)
	}
	s.mu.Lock()
	if s.active == run {
		s.active = nil
	}
	s.mu.Unlock()
}

func (a *Adapter) Close(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	s := a.sessions[sessionID]
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.closed = true
	proc := s.proc
	s.mu.Unlock()
	if proc != nil {
		proc.close()
	}
	if err := os.RemoveAll(filepath.Dir(s.dir)); err != nil && ctx.Err() == nil {
		return fmt.Errorf("pi: remove owned session directory: %w", err)
	}
	return nil
}
