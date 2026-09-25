// Package codex implements Gimbal's HarnessAdapter using Codex app-server.
package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tylergannon/gimbal/harness"
	"github.com/tylergannon/gimbal/harness/codex/schema"
)

const (
	requestTimeout = 30 * time.Second
	controlTimeout = 5 * time.Second
)

var errInterrupted = errors.New("turn was interrupted")

// Adapter translates the neutral harness contract to Codex app-server's v2
// stdio protocol. Each turn runs in its own app-server process that resumes
// the durable thread; the process from CreateSession is retained until the
// first turn because a thread with no turn is not yet resumable.
type Adapter struct {
	config processConfig
	mu     sync.Mutex
	closed bool
	states map[string]*sessionState
}

type sessionState struct {
	ops     sync.Mutex // serializes turns and compaction on this session
	mu      sync.Mutex // guards the fields below
	workdir string
	fresh   *connection
	active  *activeTurn
}

type activeTurn struct {
	connection  *connection
	turnID      string
	projector   *eventProjector
	interrupted atomic.Bool
}

// New constructs an adapter that launches `codex app-server --stdio`.
func New() *Adapter {
	return newAdapter(defaultProcessConfig())
}

func newAdapter(config processConfig) *Adapter {
	return &Adapter{config: config, states: make(map[string]*sessionState)}
}

// SetStderr forwards app-server stderr. It must be called before use.
func (a *Adapter) SetStderr(stderr io.Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.stderr = stderr
}

// CreateSession starts a durable Codex thread.
func (a *Adapter) CreateSession(model, workdir string) (string, error) {
	if err := harness.ValidateCreateSessionInput(model, workdir); err != nil {
		return "", err
	}
	connection, err := a.start()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if err := connection.initialize(ctx); err != nil {
		connection.close()
		return "", err
	}
	result, err := connection.call(ctx, "thread/start", map[string]any{
		"model":              model,
		"cwd":                workdir,
		"approvalPolicy":     "never",
		"sandbox":            "danger-full-access",
		"serviceName":        "gimbal",
		"sessionStartSource": "startup",
	})
	if err != nil {
		connection.close()
		return "", err
	}
	threadID, err := decodeThreadID(result)
	if err != nil {
		connection.close()
		return "", err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		connection.close()
		return "", errors.New("codex adapter is closed")
	}
	a.states[threadID] = &sessionState{workdir: workdir, fresh: connection}
	return threadID, nil
}

// RunTurn runs one turn on the session and blocks until it ends. With an
// output schema the result is validated against it exactly; without one the
// assistant's final text is returned. Cancelling ctx interrupts the turn.
func (a *Adapter) RunTurn(ctx context.Context, input harness.RunTurnInput, onEvent harness.OnEvent) (json.RawMessage, error) {
	if err := harness.ValidateRunTurnInput(input, onEvent); err != nil {
		return nil, err
	}
	var validator *harness.ResultValidator
	if len(input.OutputSchema) > 0 {
		var err error
		if validator, err = harness.NewResultValidator(input.OutputSchema); err != nil {
			return nil, err
		}
	}
	state, err := a.state(input.SessionID)
	if err != nil {
		return nil, err
	}
	state.ops.Lock()
	defer state.ops.Unlock()

	connection, err := a.open(state, input.SessionID, input.Workdir)
	if err != nil {
		return nil, err
	}
	defer connection.close()

	params, optionalProperties, err := turnStartParams(input)
	if err != nil {
		return nil, err
	}
	startCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	result, err := connection.call(startCtx, "turn/start", params)
	cancel()
	if err != nil {
		return nil, err
	}
	turnID, err := decodeTurnID(result)
	if err != nil {
		return nil, err
	}

	active := &activeTurn{connection: connection, turnID: turnID, projector: newEventProjector(onEvent)}
	state.setActive(active)
	defer state.clearActive(active)
	active.projector.user(input.Parts)

	text, err := readTurn(ctx, connection, input.SessionID, turnID, active)
	if ctx.Err() != nil {
		interruptTurn(connection, input.SessionID, turnID)
		drainCtx, drainCancel := context.WithTimeout(context.Background(), controlTimeout)
		_, _ = readTurn(drainCtx, connection, input.SessionID, turnID, active)
		drainCancel()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if validator == nil {
		return harness.TextResult(text)
	}
	raw, err := omitOptionalNulls([]byte(text), optionalProperties)
	if err != nil {
		return nil, fmt.Errorf("normalize Codex result: %w", err)
	}
	return validator.Validate(raw)
}

// Steer delivers ordered text parts to the session's live turn, if any.
func (a *Adapter) Steer(sessionID string, parts []harness.ContentPart) {
	if harness.ValidateContentParts(parts) != nil {
		return
	}
	active := a.activeTurn(sessionID)
	if active == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	_, err := active.connection.call(ctx, "turn/steer", map[string]any{
		"threadId":       sessionID,
		"expectedTurnId": active.turnID,
		"input":          nativeInput(parts),
	})
	if err == nil {
		active.projector.user(parts)
	}
}

// Interrupt asks the session's live turn to stop and returns immediately.
func (a *Adapter) Interrupt(sessionID string) {
	active := a.activeTurn(sessionID)
	if active == nil {
		return
	}
	active.interrupted.Store(true)
	go interruptTurn(active.connection, sessionID, active.turnID)
}

// Compact blocks until Codex reports completion of the context compaction.
func (a *Adapter) Compact(sessionID, workdir string) error {
	if err := harness.ValidateSessionInput(sessionID, workdir); err != nil {
		return err
	}
	state, err := a.state(sessionID)
	if err != nil {
		return err
	}
	if !state.ops.TryLock() {
		return errors.New("cannot compact a session with an active turn")
	}
	defer state.ops.Unlock()

	connection, err := a.open(state, sessionID, workdir)
	if err != nil {
		return err
	}
	defer connection.close()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := connection.call(ctx, "thread/compact/start", map[string]any{"threadId": sessionID}); err != nil {
		return err
	}
	return waitCompaction(ctx, connection, sessionID)
}

// Close releases every retained and live app-server process.
func (a *Adapter) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	states := make([]*sessionState, 0, len(a.states))
	for _, state := range a.states {
		states = append(states, state)
	}
	a.mu.Unlock()

	for _, state := range states {
		state.mu.Lock()
		fresh, active := state.fresh, state.active
		state.fresh = nil
		state.mu.Unlock()
		if fresh != nil {
			fresh.close()
		}
		if active != nil {
			active.connection.close()
		}
	}
}

func (a *Adapter) start() (*connection, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, errors.New("codex adapter is closed")
	}
	config := a.config
	config.args = append([]string(nil), config.args...)
	config.env = append([]string(nil), config.env...)
	return startConnection(config)
}

func (a *Adapter) state(sessionID string) (*sessionState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, errors.New("codex adapter is closed")
	}
	state := a.states[sessionID]
	if state == nil {
		state = &sessionState{}
		a.states[sessionID] = state
	}
	return state, nil
}

func (a *Adapter) activeTurn(sessionID string) *activeTurn {
	a.mu.Lock()
	state := a.states[sessionID]
	a.mu.Unlock()
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.active
}

// open returns a process attached to the session's thread: the one retained
// from CreateSession if it is still unused, otherwise a fresh process that
// resumes the thread. The caller closes it.
func (a *Adapter) open(state *sessionState, sessionID, workdir string) (*connection, error) {
	state.mu.Lock()
	if state.workdir != "" && state.workdir != workdir {
		state.mu.Unlock()
		return nil, errors.New("session working directory cannot change")
	}
	state.workdir = workdir
	fresh := state.fresh
	state.fresh = nil
	state.mu.Unlock()
	if fresh != nil {
		return fresh, nil
	}

	connection, err := a.start()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if err := connection.initialize(ctx); err != nil {
		connection.close()
		return nil, err
	}
	result, err := connection.call(ctx, "thread/resume", map[string]any{
		"threadId":       sessionID,
		"cwd":            workdir,
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	})
	if err != nil {
		connection.close()
		return nil, err
	}
	resumedID, err := decodeThreadID(result)
	if err != nil {
		connection.close()
		return nil, err
	}
	if resumedID != sessionID {
		connection.close()
		return nil, errors.New("thread/resume returned a different session ID")
	}
	return connection, nil
}

func (s *sessionState) setActive(active *activeTurn) {
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
}

func (s *sessionState) clearActive(active *activeTurn) {
	s.mu.Lock()
	if s.active == active {
		s.active = nil
	}
	s.mu.Unlock()
}

func interruptTurn(connection *connection, threadID, turnID string) {
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	_, _ = connection.call(ctx, "turn/interrupt", map[string]any{"threadId": threadID, "turnId": turnID})
}

func turnStartParams(input harness.RunTurnInput) (schema.TurnStartParams, map[string]struct{}, error) {
	var outputSchema any
	var optionalProperties map[string]struct{}
	if len(input.OutputSchema) > 0 {
		var err error
		outputSchema, optionalProperties, err = codexCompatibleOutputSchema(input.OutputSchema)
		if err != nil {
			return schema.TurnStartParams{}, nil, err
		}
	}
	model := input.Model
	workdir := input.Workdir
	return schema.TurnStartParams{
		ThreadID:       input.SessionID,
		Input:          nativeInput(input.Parts),
		Model:          schema.TurnStartParamsModel(&model),
		Cwd:            schema.TurnStartParamsCwd(&workdir),
		Effort:         schema.ReasoningEffort(input.ReasoningEffort),
		OutputSchema:   outputSchema,
		ApprovalPolicy: "never",
		SandboxPolicy:  map[string]any{"type": "dangerFullAccess"},
	}, optionalProperties, nil
}

func nativeInput(parts []harness.ContentPart) []schema.UserInput {
	result := make([]schema.UserInput, 0, len(parts))
	for _, part := range parts {
		result = append(result, map[string]any{
			"type":          "text",
			"text":          part.Text,
			"text_elements": []any{},
		})
	}
	return result
}

// readTurn consumes notifications until the turn completes and returns the
// assistant's final text.
func readTurn(ctx context.Context, connection *connection, threadID, turnID string, active *activeTurn) (string, error) {
	var assistantText string
	for {
		message, err := connection.next(ctx)
		if err != nil {
			return "", err
		}
		if len(message.ID) > 0 && message.Method != "" {
			return "", refuseServerRequest(connection, message)
		}
		if !messageMatches(message.Params, threadID, turnID) {
			continue
		}
		switch message.Method {
		case "item/started":
			active.projector.itemStarted(message.Params)
		case "item/completed":
			if text, ok := active.projector.itemCompleted(message.Params); ok {
				assistantText = text
			}
		case "thread/tokenUsage/updated":
			active.projector.usage(message.Params)
		case "turn/completed":
			status, failure, ok := decodeCompletedTurn(message.Params, threadID, turnID)
			if !ok {
				continue
			}
			switch status {
			case "completed":
				if active.interrupted.Load() {
					return "", errInterrupted
				}
				if strings.TrimSpace(assistantText) == "" {
					return "", errors.New("codex completed without an assistant result")
				}
				return assistantText, nil
			case "interrupted":
				return "", errInterrupted
			case "failed":
				if failure == "" {
					failure = "Codex turn failed"
				}
				return "", errors.New(failure)
			default:
				return "", fmt.Errorf("codex turn ended with status %q", status)
			}
		case "error":
			return "", decodeErrorNotification(message.Params)
		}
	}
}

func waitCompaction(ctx context.Context, connection *connection, threadID string) error {
	var compactTurnID string
	for {
		message, err := connection.next(ctx)
		if err != nil {
			return err
		}
		if len(message.ID) > 0 && message.Method != "" {
			return refuseServerRequest(connection, message)
		}
		if !messageMatchesThread(message.Params, threadID) {
			continue
		}
		switch message.Method {
		case "turn/started":
			compactTurnID = notificationTurnID(message.Params)
		case "item/completed":
			item, ok := decodeItem(message.Params)
			if ok && item.Type == "contextCompaction" {
				turnID := notificationTurnID(message.Params)
				if compactTurnID == "" || turnID == compactTurnID {
					return nil
				}
			}
		case "turn/completed":
			status, failure, ok := decodeCompletedTurn(message.Params, threadID, compactTurnID)
			if ok && status != "completed" {
				if failure == "" {
					failure = "Codex compaction failed"
				}
				return errors.New(failure)
			}
		case "error":
			return decodeErrorNotification(message.Params)
		}
	}
}

// refuseServerRequest declines any interactive request; Gimbal turns run
// with approvals disabled and never answer prompts.
func refuseServerRequest(connection *connection, message rpcMessage) error {
	result := map[string]any{"decision": "decline"}
	switch message.Method {
	case "execCommandApproval", "applyPatchApproval":
		result["decision"] = "denied"
	case "item/tool/requestUserInput":
		result = map[string]any{"answers": map[string]any{}}
	case "mcpServer/elicitation/request":
		result = map[string]any{"action": "decline"}
	case "item/permissions/requestApproval":
		result = map[string]any{"permissions": map[string]any{}, "scope": "turn"}
	}
	if err := connection.respond(message, result, nil); err != nil {
		return err
	}
	return fmt.Errorf("unexpected interactive Codex request %q", message.Method)
}

func decodeThreadID(raw json.RawMessage) (string, error) {
	var response struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode Codex thread response: %w", err)
	}
	if response.Thread.ID == "" {
		return "", errors.New("codex thread response omitted thread.id")
	}
	return response.Thread.ID, nil
}

func decodeTurnID(raw json.RawMessage) (string, error) {
	var response struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode Codex turn response: %w", err)
	}
	if response.Turn.ID == "" {
		return "", errors.New("codex turn response omitted turn.id")
	}
	return response.Turn.ID, nil
}

func decodeCompletedTurn(raw json.RawMessage, threadID, turnID string) (string, string, bool) {
	var notification struct {
		ThreadID string `json:"threadId"`
		Turn     struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &notification) != nil || notification.ThreadID != threadID || notification.Turn.ID != turnID {
		return "", "", false
	}
	failure := ""
	if notification.Turn.Error != nil {
		failure = notification.Turn.Error.Message
	}
	return notification.Turn.Status, failure, true
}

func decodeErrorNotification(raw json.RawMessage) error {
	var notification struct {
		Message string `json:"message"`
		Error   *struct {
			Message           string `json:"message"`
			AdditionalDetails string `json:"additionalDetails"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &notification) != nil {
		return fmt.Errorf("codex error notification: %s", strings.TrimSpace(string(raw)))
	}
	if notification.Message != "" {
		return errors.New(notification.Message)
	}
	if notification.Error != nil && notification.Error.Message != "" {
		if notification.Error.AdditionalDetails != "" {
			return fmt.Errorf("%s: %s", notification.Error.Message, notification.Error.AdditionalDetails)
		}
		return errors.New(notification.Error.Message)
	}
	return errors.New("codex emitted an empty error notification")
}

func messageMatches(raw json.RawMessage, threadID, turnID string) bool {
	var envelope struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Turn     struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.ThreadID != threadID {
		return false
	}
	if envelope.TurnID != "" {
		return envelope.TurnID == turnID
	}
	return envelope.Turn.ID == turnID
}

func messageMatchesThread(raw json.RawMessage, threadID string) bool {
	var envelope struct {
		ThreadID string `json:"threadId"`
	}
	return json.Unmarshal(raw, &envelope) == nil && envelope.ThreadID == threadID
}

func notificationTurnID(raw json.RawMessage) string {
	var envelope struct {
		TurnID string `json:"turnId"`
		Turn   struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(raw, &envelope)
	if envelope.TurnID != "" {
		return envelope.TurnID
	}
	return envelope.Turn.ID
}

var _ harness.HarnessAdapter = (*Adapter)(nil)
