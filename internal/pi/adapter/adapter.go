// Package adapter implements Gimbal's HarnessAdapter over the native,
// in-process Pi session port. A session owns its own Pi agent, history and
// storage; no subprocess is started.
package adapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/providers/openai"
	"github.com/tylergannon/gimbal/internal/pi/session"
)

const providerName = string(config.ProviderDiffusion)

// Config configures a native Pi adapter. The zero value uses the Diffusion
// Router endpoint and the DIFFUSION_API_KEY environment variable.
type Config struct {
	Endpoint string
	APIKey   func() string
}

// Adapter is the native, in-process Pi harness adapter.
type Adapter struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
	config   Config
}

var _ gimbal.HarnessAdapter = (*Adapter)(nil)

// New returns a Pi harness adapter.
func New() gimbal.HarnessAdapter { return newAdapter(Config{}) }

func newAdapter(cfg Config) *Adapter {
	if cfg.Endpoint == "" {
		cfg.Endpoint = config.DefaultDiffusionBaseURL
	}
	if cfg.APIKey == nil {
		cfg.APIKey = func() string { return os.Getenv("DIFFUSION_API_KEY") }
	}
	return &Adapter{sessions: map[string]*sessionState{}, config: cfg}
}

type sessionState struct {
	id, nativeID, model, workdir string
	root                         string
	session                      *session.Session

	mu      sync.Mutex
	turnMu  sync.Mutex
	steerMu sync.Mutex
	active  *turn
	closed  bool
}

// CreateSession reserves one native Pi session and starts its in-process
// agent.
func (a *Adapter) CreateSession(ctx context.Context, modelName, effort, workdir string) (string, error) {
	if effort != "" {
		return "", fmt.Errorf("pi: explicit effort %q is unsupported for this model", effort)
	}
	if strings.TrimSpace(modelName) == "" {
		return "", errors.New("pi: model is required")
	}
	key := a.config.APIKey()
	if key == "" {
		return "", errors.New("pi: DIFFUSION_API_KEY is required")
	}
	root, err := os.MkdirTemp("", "gimbal-pi-")
	if err != nil {
		return "", fmt.Errorf("pi: create session directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	agentDir := filepath.Join(root, "agent")
	sessionDir := filepath.Join(root, "sessions")
	for _, dir := range []string{agentDir, sessionDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			cleanup()
			return "", fmt.Errorf("pi: create session directory: %w", err)
		}
	}
	native, err := a.open(ctx, modelName, workdir, agentDir, sessionDir, key)
	if err != nil {
		cleanup()
		return "", err
	}
	sessionID, err := randomID()
	if err != nil {
		native.Dispose()
		cleanup()
		return "", err
	}
	s := &sessionState{
		id: sessionID, nativeID: native.SessionID(), model: modelName,
		workdir: workdir, root: root, session: native,
	}
	a.mu.Lock()
	a.sessions[sessionID] = s
	a.mu.Unlock()
	return sessionID, nil
}

// open builds one native session bound to the given directories.
func (a *Adapter) open(ctx context.Context, modelName, workdir, agentDir, sessionDir, key string) (*session.Session, error) {
	manager, err := history.Create(workdir, sessionDir, nil)
	if err != nil {
		return nil, fmt.Errorf("pi: create native session: %w", err)
	}
	result, err := session.Create(ctx, session.CreateOptions{
		Cwd: workdir, AgentDir: agentDir,
		SessionManager: manager,
		Model:          routerModel(modelID(modelName), a.config.Endpoint),
		StreamFn:       a.streamFn(key),
		APIKey:         key,
	})
	if err != nil {
		return nil, fmt.Errorf("pi: create session: %w", err)
	}
	return result.Session, nil
}

// streamFn binds the adapter's key to every provider request while leaving
// any caller-supplied options in place.
func (a *Adapter) streamFn(key string) model.StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		call := model.SimpleStreamOptions{}
		if options != nil {
			call = *options
		}
		if call.APIKey == "" {
			call.APIKey = key
		}
		return openai.StreamSimple(ctx, m, transcript, &call)
	}
}

// routerModel is the Pi model entry for one Diffusion Router id. The limits
// match the ones Pi wrote into models.json before the native port: new ids
// keep Pi's conservative defaults until their limits have been checked.
func routerModel(id, endpoint string) *model.Model {
	m := &model.Model{
		Type:     model.ModelTypeChat,
		ID:       id,
		Name:     id,
		Api:      model.APIOpenAICompletions,
		Provider: config.ProviderDiffusion,
		BaseURL:  endpoint,
	}
	switch id {
	case "deepseek-4.1-flash", "deepseek-4.1-flash-background":
		m.ContextWindow, m.MaxTokens = 1048576, 262144
	case "glm-5.3-flash", "glm-5.3-flash-background":
		m.ContextWindow, m.MaxTokens = 524288, 163840
	case "glm-5.2-vision", "glm-5.2-vision-background", "glm-5.2-vision-flex",
		"glm-5.3", "glm-5.3-background", "glm-5.3-vision", "glm-5.3-vision-background":
		m.ContextWindow, m.MaxTokens = 524288, 131072
	}
	if m.ContextWindow > 0 {
		m.Input = []string{"text", "image"}
	}
	return m
}

func modelID(modelName string) string {
	if before, after, found := strings.Cut(modelName, "/"); found {
		if before == providerName {
			return after
		}
		return modelName
	}
	return modelName
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("pi: create session id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (a *Adapter) session(id string) (*sessionState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[id]
	if s == nil {
		return nil, fmt.Errorf("pi: no session %q", id)
	}
	return s, nil
}

// RunTurn runs one turn on the native session and blocks until it settles.
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
	native := s.session
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

	run := newTurn(sessionID, modelID(s.model), onEvent)
	s.mu.Lock()
	s.active = run
	s.mu.Unlock()
	unsubscribe := native.Subscribe(run.record)
	promptErr := native.Prompt(ctx, message, nil)
	unsubscribe()
	// A steer accepted at the settlement boundary must not spill into the
	// next turn; the native session drains its queues here.
	native.ClearQueue()
	s.mu.Lock()
	if s.active == run {
		s.active = nil
	}
	s.mu.Unlock()

	if ctx.Err() != nil {
		return gimbal.TurnResult{}, ctx.Err()
	}
	if promptErr != nil {
		return gimbal.TurnResult{}, fmt.Errorf("pi: prompt: %w", promptErr)
	}
	if run.err != nil {
		return gimbal.TurnResult{}, run.err
	}
	final := run.finalMessage()
	if final == nil {
		return gimbal.TurnResult{}, errors.New("pi: agent_settled without a final assistant message")
	}
	if final.ErrorMessage != "" || final.StopReason == model.StopError {
		return gimbal.TurnResult{}, fmt.Errorf("pi: provider error: %s", final.ErrorMessage)
	}
	text := strings.TrimSpace(model.ContentText(final.Content))
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
	return gimbal.TurnResult{Output: output, Usage: run.usageReport()}, nil
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

// Steer sends a message into the session's running turn and reports whether
// it was consumed by that turn.
func (a *Adapter) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return false, err
	}
	s.steerMu.Lock()
	defer s.steerMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false, fmt.Errorf("pi: session %q is closed", sessionID)
	}
	run, native := s.active, s.session
	s.mu.Unlock()
	if run == nil || !native.IsStreaming() {
		return false, nil
	}
	waiter := &steerWaiter{text: message, delivered: make(chan struct{})}
	run.addWaiter(waiter)
	defer run.removeWaiter(waiter)
	if err := native.Steer(message); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, fmt.Errorf("pi: steer: %w", err)
	}
	select {
	case <-waiter.delivered:
		return true, nil
	case <-run.settled:
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// Fork returns a new native session with an independent deep snapshot of the
// conversation so far.
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
	native := parent.session
	parent.mu.Unlock()

	child, err := native.Fork(ctx)
	if err != nil {
		return "", fmt.Errorf("pi: fork session: %w", err)
	}
	childID, err := randomID()
	if err != nil {
		child.Dispose()
		return "", err
	}
	s := &sessionState{
		id: childID, nativeID: child.SessionID(), model: parent.model,
		workdir: parent.workdir, session: child,
	}
	a.mu.Lock()
	a.sessions[childID] = s
	a.mu.Unlock()
	return childID, nil
}

// Close releases the session. It is idempotent.
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
	native := s.session
	s.mu.Unlock()
	if native != nil {
		native.Dispose()
	}
	if s.root != "" {
		if err := os.RemoveAll(s.root); err != nil && ctx.Err() == nil {
			return fmt.Errorf("pi: remove owned session directory: %w", err)
		}
	}
	return nil
}
