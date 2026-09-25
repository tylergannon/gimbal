// Package claude implements Gimbal's HarnessAdapter using Claude Code.
package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/tylergannon/gimbal/harness"
)

const (
	controlTimeout = 5 * time.Second
	compactTimeout = 20 * time.Minute
)

var errInterrupted = errors.New("turn was interrupted")

type nativeConfig struct {
	sessionID       string
	model           string
	reasoningEffort string
	workdir         string
	outputSchema    any
	fresh           bool
	stderr          io.Writer
	env             map[string]string
}

type nativeSession interface {
	Send(context.Context, string) error
	Messages() iter.Seq[claudeagent.Message]
	Errors() <-chan error
	InterruptWithReceipt(context.Context) (*claudeagent.InterruptReceipt, error)
	Close() error
}

type sessionFactory func(context.Context, nativeConfig) (nativeSession, error)

// Adapter translates the neutral harness contract to Claude Code's SDK
// protocol. Each turn launches Claude Code against the session ID; native
// conversation history stays Claude Code's responsibility.
type Adapter struct {
	mu      sync.Mutex
	closed  bool
	stderr  io.Writer
	states  map[string]*sessionState
	factory sessionFactory
}

type sessionState struct {
	ops     sync.Mutex // serializes turns and compaction on this session
	mu      sync.Mutex // guards the fields below
	workdir string
	fresh   bool // no native conversation exists yet
	active  *activeTurn
}

type activeTurn struct {
	session     nativeSession
	projector   *eventProjector
	interrupted atomic.Bool
}

// New constructs an adapter that launches Claude Code through the pinned SDK.
func New() *Adapter {
	return newAdapter(openNativeSession)
}

func newAdapter(factory sessionFactory) *Adapter {
	return &Adapter{states: make(map[string]*sessionState), factory: factory}
}

// SetStderr forwards Claude Code stderr. It must be called before use.
func (a *Adapter) SetStderr(stderr io.Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stderr = stderr
}

// CreateSession mints the native ID that the first Claude Code invocation
// receives through --session-id.
func (a *Adapter) CreateSession(model, workdir string) (string, error) {
	if err := harness.ValidateCreateSessionInput(model, workdir); err != nil {
		return "", err
	}
	id, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("mint Claude session ID: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return "", errors.New("claude adapter is closed")
	}
	a.states[id] = &sessionState{workdir: workdir, fresh: true}
	return id, nil
}

// RunTurn runs one turn on the session and blocks until it ends. With an
// output schema the structured result is validated against it exactly;
// without one the assistant's final text is returned. Cancelling ctx
// interrupts the turn.
func (a *Adapter) RunTurn(ctx context.Context, input harness.RunTurnInput, onEvent harness.OnEvent) (json.RawMessage, error) {
	if err := harness.ValidateRunTurnInput(input, onEvent); err != nil {
		return nil, err
	}
	var validator *harness.ResultValidator
	var outputSchema any
	if len(input.OutputSchema) > 0 {
		var err error
		if validator, err = harness.NewResultValidator(input.OutputSchema); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(input.OutputSchema, &outputSchema); err != nil {
			return nil, fmt.Errorf("decode output schema: %w", err)
		}
	}
	state, err := a.state(input.SessionID)
	if err != nil {
		return nil, err
	}
	state.ops.Lock()
	defer state.ops.Unlock()
	fresh, err := state.bind(input.Workdir)
	if err != nil {
		return nil, err
	}

	// The native process is not tied to ctx: on cancellation the turn is
	// interrupted first so Claude Code can acknowledge, then closed.
	sessionCtx, cancelSession := context.WithCancel(context.Background())
	defer cancelSession()
	session, err := a.open(sessionCtx, nativeConfig{
		sessionID:       input.SessionID,
		model:           input.Model,
		reasoningEffort: input.ReasoningEffort,
		workdir:         input.Workdir,
		outputSchema:    outputSchema,
		fresh:           fresh,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	active := &activeTurn{session: session, projector: newEventProjector(onEvent)}
	state.setActive(active)
	defer state.clearActive(active)
	active.projector.user(input.Parts)
	if err := session.Send(sessionCtx, joinParts(input.Parts)); err != nil {
		return nil, err
	}

	result, err := waitTurn(ctx, session, state, input.SessionID, active)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if validator == nil {
		return harness.TextResult(result.Result)
	}
	if result.StructuredOutput == nil {
		return nil, errors.New("claude completed without structured output")
	}
	raw, err := json.Marshal(result.StructuredOutput)
	if err != nil {
		return nil, fmt.Errorf("encode Claude structured output: %w", err)
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
	if err := active.session.Send(ctx, joinParts(parts)); err == nil {
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
		defer cancel()
		_, _ = active.session.InterruptWithReceipt(ctx)
	}()
}

// Compact sends Claude Code's native /compact command and waits for its
// terminal compact status.
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
	if _, err := state.bind(workdir); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), compactTimeout)
	defer cancel()
	session, err := a.open(ctx, nativeConfig{sessionID: sessionID, workdir: workdir})
	if err != nil {
		return err
	}
	if err := session.Send(ctx, "/compact"); err != nil {
		return err
	}
	messages, stop := readMessages(session)
	defer stop()
	for {
		var message claudeagent.Message
		select {
		case <-ctx.Done():
			return errors.New("claude compaction timed out")
		case nativeErr := <-session.Errors():
			return nativeErr
		case next, ok := <-messages:
			if !ok {
				return errors.New("claude stream ended without a compaction result")
			}
			message = next
		}
		if err := validateMessageSession(message, sessionID); err != nil {
			return err
		}
		if status, ok := asStatus(message); ok && status.Status == nil {
			switch status.CompactResult {
			case claudeagent.CompactResultSuccess:
				return nil
			case claudeagent.CompactResultFailed:
				message := strings.TrimSpace(status.CompactError)
				if message == "" {
					message = "Claude compaction failed"
				}
				return errors.New(message)
			}
		}
		if result, ok := asResult(message); ok && result.Subtype != "success" && result.Status != "success" {
			return errors.New(resultFailure(result))
		}
	}
}

// Close interrupts live turns. Each turn otherwise owns and closes its
// native process.
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
		active := state.active
		state.mu.Unlock()
		if active != nil {
			active.interrupted.Store(true)
			ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
			_, _ = active.session.InterruptWithReceipt(ctx)
			cancel()
		}
	}
}

func (a *Adapter) open(ctx context.Context, config nativeConfig) (nativeSession, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, errors.New("claude adapter is closed")
	}
	config.stderr = a.stderr
	for _, name := range []string{"GIMBAL_RUN_DIR", "GIMBAL_SERVICE_PORT"} {
		if value, ok := os.LookupEnv(name); ok {
			if config.env == nil {
				config.env = make(map[string]string)
			}
			config.env[name] = value
		}
	}
	factory := a.factory
	a.mu.Unlock()
	return factory(ctx, config)
}

func (a *Adapter) state(sessionID string) (*sessionState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, errors.New("claude adapter is closed")
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

// bind pins the session to workdir and reports whether no native
// conversation exists yet.
func (s *sessionState) bind(workdir string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workdir != "" && s.workdir != workdir {
		return false, errors.New("session working directory cannot change")
	}
	s.workdir = workdir
	return s.fresh, nil
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

// waitTurn reads the turn to its result. When ctx ends first it interrupts
// the native turn, waits briefly for Claude Code to wind down, and returns.
func waitTurn(ctx context.Context, session nativeSession, state *sessionState, sessionID string, active *activeTurn) (claudeagent.ResultMessage, error) {
	messages, stop := readMessages(session)
	defer stop()

	ctxDone := ctx.Done()
	var grace <-chan time.Time
	for {
		select {
		case nativeErr := <-session.Errors():
			return claudeagent.ResultMessage{}, nativeErr
		case <-ctxDone:
			ctxDone = nil
			active.interrupted.Store(true)
			interruptCtx, cancel := context.WithTimeout(context.Background(), controlTimeout)
			_, _ = session.InterruptWithReceipt(interruptCtx)
			cancel()
			grace = time.After(controlTimeout)
		case <-grace:
			return claudeagent.ResultMessage{}, ctx.Err()
		case message, ok := <-messages:
			if !ok {
				if active.interrupted.Load() {
					return claudeagent.ResultMessage{}, errInterrupted
				}
				return claudeagent.ResultMessage{}, errors.New("claude stream ended without a result")
			}
			if err := validateMessageSession(message, sessionID); err != nil {
				return claudeagent.ResultMessage{}, err
			}
			promoteMaterialized(state, message, sessionID)
			if err := assistantFailure(message); err != nil {
				return claudeagent.ResultMessage{}, err
			}
			active.projector.message(message)
			if result, ok := asResult(message); ok {
				if active.interrupted.Load() {
					return claudeagent.ResultMessage{}, errInterrupted
				}
				if result.Subtype != "success" && result.Status != "success" {
					return claudeagent.ResultMessage{}, errors.New(resultFailure(result))
				}
				return result, nil
			}
		}
	}
}

func assistantFailure(message claudeagent.Message) error {
	var code claudeagent.AssistantMessageError
	switch assistant := message.(type) {
	case claudeagent.AssistantMessage:
		code = assistant.Error
	case *claudeagent.AssistantMessage:
		code = assistant.Error
	}
	if code == "" {
		return nil
	}
	return errors.New("Claude assistant error: " + string(code))
}

func readMessages(session nativeSession) (<-chan claudeagent.Message, func()) {
	messages := make(chan claudeagent.Message)
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		defer close(messages)
		for message := range session.Messages() {
			select {
			case messages <- message:
			case <-stop:
				return
			}
		}
	}()
	return messages, func() {
		close(stop)
		_ = session.Close()
		<-done
	}
}

// promoteMaterialized marks the session as having a native conversation once
// any post-init message for it arrives.
func promoteMaterialized(state *sessionState, message claudeagent.Message, sessionID string) {
	envelope, err := nativeEnvelope(message)
	if err != nil || envelope.SessionID != sessionID || (envelope.Type == "system" && envelope.Subtype == "init") {
		return
	}
	state.mu.Lock()
	state.fresh = false
	state.mu.Unlock()
}

func validateMessageSession(message claudeagent.Message, sessionID string) error {
	envelope, err := nativeEnvelope(message)
	if err != nil {
		return fmt.Errorf("decode Claude message: %w", err)
	}
	if envelope.SessionID != "" && envelope.SessionID != sessionID {
		return errors.New("claude returned a different session ID")
	}
	if envelope.Type == "system" && envelope.Subtype == "init" && envelope.SessionID == "" {
		return errors.New("claude init omitted its session ID")
	}
	return nil
}

type messageEnvelope struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
}

func nativeEnvelope(message claudeagent.Message) (messageEnvelope, error) {
	raw, err := json.Marshal(message)
	if err != nil {
		return messageEnvelope{}, err
	}
	var envelope messageEnvelope
	err = json.Unmarshal(raw, &envelope)
	return envelope, err
}

func joinParts(parts []harness.ContentPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		texts = append(texts, part.Text)
	}
	return strings.Join(texts, "\n\n")
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	var encoded [32]byte
	hex.Encode(encoded[:], value[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

func resultFailure(result claudeagent.ResultMessage) string {
	parts := append([]string(nil), result.Errors...)
	if result.TerminalReason != nil {
		parts = append(parts, string(*result.TerminalReason))
	}
	if len(parts) == 0 {
		parts = append(parts, result.Subtype)
	}
	return "Claude turn failed: " + strings.Join(parts, "; ")
}

var _ harness.HarnessAdapter = (*Adapter)(nil)
