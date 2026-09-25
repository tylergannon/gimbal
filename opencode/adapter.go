package opencode

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
)

const controlTimeout = 5 * time.Second

type adapterConfig struct {
	stateDir string
	connect  func(context.Context, string, string) (*Client, error)
}

type adapter struct {
	mu       sync.Mutex
	sessions map[string]*adapterSession
	config   adapterConfig
	events   *eventStream
	capture  *rawCapture
}

type adapterSession struct {
	client   *Client
	model    string
	provider string
	variant  string
	workdir  string

	ops sync.Mutex
	mu  sync.Mutex

	active *activeTurn
}

type activeTurn struct {
	requestID string
	projector *projector
	pending   []*steerRequest
	accepting bool
	events    sync.Mutex
	closed    bool
}

type steerRequest struct {
	message string
	settled chan struct{}
}

type eventStream struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	ready  chan struct{}
	done   chan struct{}
	once   sync.Once
	err    error
}

// New returns Gimbal's legacy OpenCode harness. The first session starts or
// discovers the shared OpenCode server; closing sessions never stops it.
func New() gimbal.HarnessAdapter {
	return newAdapter(adapterConfig{})
}

func newAdapter(config adapterConfig) *adapter {
	if config.connect == nil {
		config.connect = Connect
	}
	return &adapter{sessions: make(map[string]*adapterSession), config: config}
}

// CreateSession creates one native OpenCode session in workdir. A single
// process-wide event stream is established before the native session is made
// so its later prompt events can be routed without per-session subscriptions.
func (a *adapter) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	provider, modelID, err := splitModel(model)
	if err != nil {
		return "", err
	}
	client, err := a.config.connect(ctx, workdir, a.config.stateDir)
	if err != nil {
		return "", fmt.Errorf("opencode: connect: %w", err)
	}
	if err := a.ensureCapture(); err != nil {
		return "", err
	}
	if err := a.ensureEvents(ctx, client); err != nil {
		return "", fmt.Errorf("opencode: connect event stream: %w", err)
	}

	input := SessionCreateInput{Model: &SessionModel{ID: modelID, ProviderID: provider}}
	if effort != "" {
		input.Model.Variant = &effort
	}
	native, err := client.CreateSession(ctx, input)
	if err != nil {
		return "", fmt.Errorf("opencode: create session: %w", err)
	}
	if strings.TrimSpace(native.Id) == "" {
		return "", errors.New("opencode: create session returned a blank id")
	}
	s := &adapterSession{
		client: client, model: modelID, provider: provider, variant: effort,
		workdir: native.Directory,
	}
	a.mu.Lock()
	a.sessions[native.Id] = s
	a.mu.Unlock()
	return native.Id, nil
}

// RunTurn posts one authoritative synchronous prompt. The shared event stream
// is diagnostic: stream or projection gaps are captured but cannot replace a
// successful POST result with failure.
func (a *adapter) RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	s.ops.Lock()
	defer s.ops.Unlock()

	_ = a.ensureEvents(ctx, s.client)
	requestID, err := captureID("req")
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	active := &activeTurn{
		requestID: requestID,
		projector: newProjector(sessionID, s.provider, s.model, onEvent),
		accepting: true,
	}
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.active == active {
			active.accepting = false
			s.active = nil
		}
		s.mu.Unlock()
		active.events.Lock()
		active.closed = true
		active.events.Unlock()
	}()

	for {
		input, err := promptInput(s, prompt, schema)
		if err != nil {
			return gimbal.TurnResult{}, err
		}
		a.captureRecord(captureEntry{
			Kind: "request", RequestID: requestID, SessionID: sessionID,
			Workdir: s.workdir, Operation: "prompt", Value: input,
		})
		response, promptErr := a.prompt(ctx, s, sessionID, input)
		a.captureRecord(captureEntry{
			Kind: "result", RequestID: requestID, SessionID: sessionID,
			Workdir: s.workdir, Operation: "prompt", Value: response, Error: errorText(promptErr),
		})
		if ctx.Err() != nil {
			return gimbal.TurnResult{}, ctx.Err()
		}

		next, err := takeSteer(ctx, s, active)
		if err != nil {
			return gimbal.TurnResult{}, err
		}
		if next != nil {
			prompt = next.message
			requestID, err = captureID("req")
			if err != nil {
				return gimbal.TurnResult{}, err
			}
			s.mu.Lock()
			active.requestID = requestID
			s.mu.Unlock()
			continue
		}
		if promptErr != nil {
			return gimbal.TurnResult{}, promptErr
		}
		active.finish(response)
		return turnResult(response, schema)
	}
}

func (a *adapter) prompt(ctx context.Context, s *adapterSession, sessionID string, input PromptInput) (PromptResponse, error) {
	type outcome struct {
		response PromptResponse
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		response, err := s.client.Prompt(ctx, sessionID, input)
		done <- outcome{response: response, err: err}
	}()
	select {
	case result := <-done:
		return result.response, result.err
	case <-ctx.Done():
		abortCtx, cancel := context.WithTimeout(context.Background(), controlTimeout)
		_, _ = s.client.Abort(abortCtx, sessionID)
		cancel()
		select {
		case <-done:
		case <-time.After(controlTimeout):
		}
		return PromptResponse{}, ctx.Err()
	}
}

// Steer interrupts the native prompt and queues message for immediate
// continuation inside the same Gimbal turn. With no active turn it is dropped.
func (a *adapter) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return false, err
	}
	request := &steerRequest{message: message, settled: make(chan struct{})}
	s.mu.Lock()
	active := s.active
	if active != nil && active.accepting {
		active.pending = append(active.pending, request)
	} else {
		active = nil
	}
	s.mu.Unlock()
	if active == nil {
		return false, nil
	}

	aborted, abortErr := s.client.Abort(ctx, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != active {
		removeSteer(active, request)
		close(request.settled)
		return false, nil
	}
	if abortErr != nil {
		removeSteer(active, request)
		close(request.settled)
		return false, fmt.Errorf("opencode: steer: %w", abortErr)
	}
	close(request.settled)
	// false can mean the prompt crossed its native completion boundary while
	// the abort was in flight. It is still locally active and will claim the
	// queued continuation before RunTurn returns.
	_ = aborted
	return true, nil
}

// Fork copies the native conversation and retains the explicit model routing
// Gimbal applies on every subsequent prompt.
func (a *adapter) Fork(ctx context.Context, sessionID string) (string, error) {
	parent, err := a.session(sessionID)
	if err != nil {
		return "", err
	}
	parent.ops.Lock()
	defer parent.ops.Unlock()
	requestID, err := captureID("req")
	if err != nil {
		return "", err
	}
	input := ForkInput{}
	a.captureRecord(captureEntry{
		Kind: "request", RequestID: requestID, SessionID: sessionID,
		Workdir: parent.workdir, Operation: "fork", Value: input,
	})
	native, err := parent.client.Fork(ctx, sessionID, input)
	a.captureRecord(captureEntry{
		Kind: "result", RequestID: requestID, SessionID: sessionID,
		Workdir: parent.workdir, Operation: "fork", Value: native, Error: errorText(err),
	})
	if err != nil {
		return "", fmt.Errorf("opencode: fork session %s: %w", sessionID, err)
	}
	if strings.TrimSpace(native.Id) == "" {
		return "", errors.New("opencode: fork returned a blank id")
	}
	forked := &adapterSession{
		client: parent.client, model: parent.model, provider: parent.provider,
		variant: parent.variant, workdir: native.Directory,
	}
	a.mu.Lock()
	a.sessions[native.Id] = forked
	a.mu.Unlock()
	return native.Id, nil
}

// Close forgets sessionID and interrupts only its active native work. It does
// not delete native history, dispose a project, or stop the shared server.
func (a *adapter) Close(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	s := a.sessions[sessionID]
	delete(a.sessions, sessionID)
	empty := len(a.sessions) == 0
	stream := a.events
	if empty {
		a.events = nil
	}
	a.mu.Unlock()
	if s == nil {
		return nil
	}

	s.mu.Lock()
	active := s.active
	if active != nil {
		active.accepting = false
	}
	s.active = nil
	s.mu.Unlock()
	if active != nil {
		active.events.Lock()
		active.closed = true
		active.events.Unlock()
	}
	var closeErr error
	if active != nil {
		abortCtx := ctx
		var cancel context.CancelFunc
		if _, ok := ctx.Deadline(); !ok {
			abortCtx, cancel = context.WithTimeout(ctx, controlTimeout)
			defer cancel()
		}
		if _, err := s.client.Abort(abortCtx, sessionID); err != nil && ctx.Err() == nil {
			closeErr = fmt.Errorf("opencode: close session %s: %w", sessionID, err)
		}
	}
	if empty && stream != nil {
		stream.cancel()
		select {
		case <-stream.done:
		case <-time.After(controlTimeout):
		}
	}
	if empty {
		a.closeCapture()
	}
	return closeErr
}

func (a *adapter) session(id string) (*adapterSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.sessions[id]; s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("opencode: no session %q", id)
}

func promptInput(s *adapterSession, prompt string, schema json.RawMessage) (PromptInput, error) {
	input := PromptInput{
		Model: &PromptModel{ProviderID: s.provider, ModelID: s.model},
		Parts: []TextPartInput{{Type: TextPartInputTypeText, Text: prompt}},
	}
	if s.variant != "" {
		input.Variant = &s.variant
	}
	if len(schema) > 0 {
		var value JSONSchema
		if err := json.Unmarshal(schema, &value); err != nil {
			return PromptInput{}, fmt.Errorf("opencode: decode output schema: %w", err)
		}
		var format OutputFormat
		if err := format.FromOutputFormatJsonSchema(OutputFormatJsonSchema{Type: JsonSchema, Schema: value}); err != nil {
			return PromptInput{}, fmt.Errorf("opencode: encode output schema: %w", err)
		}
		input.Format = &format
	}
	return input, nil
}

func turnResult(response PromptResponse, schema json.RawMessage) (gimbal.TurnResult, error) {
	if response.Info.Error != nil {
		raw, _ := json.Marshal(response.Info.Error)
		return gimbal.TurnResult{}, fmt.Errorf("opencode: turn failed: %s", raw)
	}
	if len(schema) > 0 {
		if response.Info.Structured == nil {
			return gimbal.TurnResult{}, errors.New("opencode: turn ended without structured output")
		}
		output, err := json.Marshal(response.Info.Structured)
		return gimbal.TurnResult{Output: output}, err
	}
	var text strings.Builder
	for _, part := range response.Parts {
		raw, err := json.Marshal(part)
		if err != nil {
			continue
		}
		var kind struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &kind) != nil || kind.Type != "text" {
			continue
		}
		value, err := part.AsTextPart()
		if err != nil || value.Ignored != nil && *value.Ignored {
			continue
		}
		text.WriteString(value.Text)
	}
	output, err := json.Marshal(text.String())
	return gimbal.TurnResult{Output: output}, err
}

func splitModel(value string) (provider, model string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", errors.New("opencode: model is blank")
	}
	provider, model, found := strings.Cut(value, "/")
	if !found {
		return "opencode", value, nil
	}
	if provider == "" || model == "" {
		return "", "", fmt.Errorf("opencode: invalid provider/model %q", value)
	}
	return provider, model, nil
}

func takeSteer(ctx context.Context, s *adapterSession, active *activeTurn) (*steerRequest, error) {
	for {
		s.mu.Lock()
		if s.active != active || len(active.pending) == 0 {
			if s.active == active {
				active.accepting = false
			}
			s.mu.Unlock()
			return nil, nil
		}
		request := active.pending[0]
		s.mu.Unlock()

		select {
		case <-request.settled:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		s.mu.Lock()
		if s.active != active {
			s.mu.Unlock()
			return nil, nil
		}
		if len(active.pending) > 0 && active.pending[0] == request {
			active.pending = active.pending[1:]
			s.mu.Unlock()
			return request, nil
		}
		s.mu.Unlock()
		// Steer removed a request whose Abort failed or whose turn ended while
		// the request was in flight. Recheck any later queued request.
	}
}

func removeSteer(active *activeTurn, request *steerRequest) {
	for index, candidate := range active.pending {
		if candidate == request {
			active.pending = append(active.pending[:index], active.pending[index+1:]...)
			return
		}
	}
}

func captureID(prefix string) (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("opencode: mint %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(value[:]), nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (active *activeTurn) observe(eventType, eventID string, payload json.RawMessage) error {
	active.events.Lock()
	defer active.events.Unlock()
	if active.closed {
		return nil
	}
	return active.projector.event(eventType, eventID, payload)
}

func (active *activeTurn) finish(response PromptResponse) {
	active.events.Lock()
	defer active.events.Unlock()
	if !active.closed {
		active.projector.finish(response)
	}
}

var _ gimbal.HarnessAdapter = (*adapter)(nil)
