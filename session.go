package gimble

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Session is one agent conversation on one harness, in one workdir. It
// belongs to the scope that created it and is closed when that scope ends.
type Session struct {
	adapter HarnessAdapter
	name    string
	model   string
	effort  string
	workdir string
	id      string // the creating scope's key, then name.ordinal, as in lap.3/coder.1

	mu      sync.Mutex
	native  string // the harness's session id, made on the first turn
	turns   int
	turnID  string
	running bool
	closed  bool

	usage            Usage // the session's running total across its turns
	canonicalSession string
	eventSeq         uint64
	messageIDs       map[string]string
	activeEmit       func(AgentEvent) error
}

// NewSession creates a session in the scope the ctx is in, for the role of
// that name. The role says what the session does; what it runs on is the
// binding the run was started with. It cannot fail: the agent process
// starts on the first turn. A role the run did not bind is a programming
// error and panics, naming the role. A session created outside Run cannot
// generate turns or be forked.
func NewSession(ctx context.Context, role, workdir string) *Session {
	s := &Session{name: role, workdir: workdir}
	scope, err := current(ctx)
	if err != nil {
		return s
	}
	binding, bound := scope.run.models[role]
	if !bound {
		panic(fmt.Sprintf("gimble: the run did not bind the role %q", role))
	}
	s.adapter, s.model, s.effort = binding.Adapter, binding.Model, binding.Effort
	scope.adopt(s)
	scope.run.event(scope.key, s.id, "", SessionCreated{Name: role, Adapter: fmt.Sprintf("%T", s.adapter), Model: s.model, Effort: s.effort, Workdir: workdir})
	return s
}

// Text is the output of a prose turn: no schema is sent, and the result is
// the agent's final message.
type Text string

// Schema is empty: a prose turn sends no schema.
func (Text) Schema() json.RawMessage { return nil }

// ValidateJSON accepts the final message, a JSON string.
func (Text) ValidateJSON(raw []byte) error {
	var text string
	return json.Unmarshal(raw, &text)
}

// Generate runs one turn and blocks until it ends. Before dispatch, the
// ctx scope's rendered context is appended to prompt as
// prompt + "\n\n" + context, or nothing when the scope holds no values;
// everything downstream (the recorded prompt, a supervisor's intro, the
// re-ask on an invalid result, every harness adapter) sees that full
// prompt. T's schema is sent with the prompt, and the result is validated
// here and decoded into T. A result that does not validate is shown back to
// the model with the reason, a bounded number of times, before it is an
// error. For Text no schema is sent and the result is the final message.
// The options attach supervisors, and WithScopeTemplate renders the scope's
// context for this call in place of the runtime's own rendering.
func (s *Session) Generate[T Output](ctx context.Context, prompt string, opts ...AgentOption) (T, error) {
	ask, err := scopedPrompt(ctx, prompt, apply(opts))
	if err != nil {
		var out T
		return out, err
	}
	return dispatch[T](ctx, s, ask, opts)
}

// scopedPrompt is what Generate sends: prompt with the ctx scope's context
// appended as prompt + "\n\n" + context, rendered the runtime's way or, when
// the call gave a template, through it. A scope that holds no values, and a
// template that renders to nothing, leave prompt as it is.
func scopedPrompt(ctx context.Context, prompt string, o options) (string, error) {
	if o.scopeTemplate == "" {
		return appendScopeText(ctx, prompt), nil
	}
	shape, err := scopeTemplate(o.scopeTemplate)
	if err != nil {
		return "", err
	}
	var rendered strings.Builder
	if err := shape.Execute(&rendered, scopeData(ctx)); err != nil {
		return "", fmt.Errorf("gimble: render the scope template: %w", err)
	}
	text := strings.TrimSpace(rendered.String())
	if text == "" {
		return prompt, nil
	}
	return prompt + "\n\n" + text, nil
}

// scopeTemplates holds what WithScopeTemplate has parsed, by its text. A
// template's text is a constant or an embedded file, so one parse serves
// every call that passes it, however many turns a run takes.
var scopeTemplates sync.Map

// scopeTemplate parses text, or returns what an earlier call parsed.
func scopeTemplate(text string) (*template.Template, error) {
	if parsed, ok := scopeTemplates.Load(text); ok {
		return parsed.(*template.Template), nil
	}
	parsed, err := template.New("scope").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("gimble: parse the scope template: %w", err)
	}
	scopeTemplates.Store(text, parsed)
	return parsed, nil
}

// appendScopeText adds the ctx scope's rendered context to prompt, the way
// Generate hands it to the agent: prompt + "\n\n" + context, or prompt
// unchanged when the scope holds no values.
func appendScopeText(ctx context.Context, prompt string) string {
	if text := scopeText(ctx); text != "" {
		return prompt + "\n\n" + text
	}
	return prompt
}

// dispatch runs one turn for opts, without touching prompt: the internal
// callers that build a prompt at runtime (Loop's planner turn, a
// supervisor's look) call this directly instead of the exported Generate,
// so they are exempt from GIMBLE108's constant-prompt rule and are not
// given scope context a second time.
func dispatch[T Output](ctx context.Context, s *Session, prompt string, opts []AgentOption) (T, error) {
	if o := apply(opts); len(o.supervisors) > 0 {
		return supervise[T](ctx, s, prompt, o.supervisors)
	}
	var out T
	return generate[T](ctx, s, prompt, nil, fmt.Sprintf("%T", out))
}

// errInvalidResult marks a turn whose harness succeeded but whose result did
// not validate or decode. generate re-asks the model on it and returns every
// other error as it is.
var errInvalidResult = errors.New("invalid result")

// generateAttempts bounds how many times one Generate asks the model for a
// result. A result that fails validation or decoding is shown back to the
// model with the reason; only the last failure is returned.
const generateAttempts = 3

func generate[T Output](ctx context.Context, s *Session, prompt string, onEvent func(AgentEvent) error, outputType string) (T, error) {
	var out T
	var problem error
	for attempt := 1; ; attempt++ {
		ask := prompt
		if problem != nil {
			ask = prompt + fmt.Sprintf("\n\nYour previous answer was invalid and was discarded: %v. Answer again, correctly.", problem)
		}
		raw, err := s.turn(ctx, ask, out.Schema(), onEvent, outputType, out.ValidateJSON)
		if err == nil {
			if err = json.Unmarshal(raw, &out); err == nil {
				return out, nil
			}
			err = fmt.Errorf("gimble: %s: decode the result: %w: %w", s.id, errInvalidResult, err)
		} else if !errors.Is(err, errInvalidResult) {
			return out, err
		}
		if attempt == generateAttempts {
			return out, fmt.Errorf("gimble: %s: no valid result after %d attempts: %w", s.id, attempt, err)
		}
		logf("%s: the result is invalid, so the model is asked again (%d of %d): %v", s.id, attempt, generateAttempts, err)
		problem = err
	}
}

// turn runs one agent turn and records its outcome. validate, if any, is the
// typed output's check: the turn is recorded as failed when the result does
// not validate, because that is what the turn produced.
func (s *Session) turn(ctx context.Context, prompt string, schema json.RawMessage, onEvent func(AgentEvent) error, outputType string, validate func([]byte) error) (json.RawMessage, error) {
	s.mu.Lock()
	if err := s.usable(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.running = true
	s.turns++
	turnID := fmt.Sprintf("%s/turn.%d", s.id, s.turns)
	s.turnID = turnID
	native := s.native
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.turnID = ""
		s.activeEmit = nil
		s.mu.Unlock()
	}()

	if native == "" {
		id, err := s.adapter.CreateSession(ctx, s.model, s.effort, s.workdir)
		if err != nil {
			return nil, fmt.Errorf("gimble: %s: %w", s.id, err)
		}
		s.mu.Lock()
		s.native, native = id, id
		s.mu.Unlock()
	}
	if onEvent == nil {
		onEvent = func(AgentEvent) error { return nil }
	}
	logf("%s: turn started (%s)", s.id, s.model)
	start := time.Now()
	scope, _ := current(ctx)
	if scope != nil {
		scope.run.event(scope.key, s.id, turnID, TurnStarted{Prompt: prompt, OutputType: outputType})
	}
	var turnUsage Usage
	var wrapped func(AgentEvent) error
	wrapped = func(e AgentEvent) error {
		stamped, err := s.stampAgentEvent(e)
		if err != nil {
			return err
		}
		if scope != nil {
			if err := scope.run.sessionEvent(scope.key, s.id, turnID, stamped); err != nil {
				return err
			}
		}
		if err := onEvent(stamped); err != nil {
			return err
		}
		if step, carried := stepUsage(stamped); carried {
			turnUsage.add(step)
			return s.recordUsage(step, native, turnID, wrapped)
		}
		return nil
	}
	s.mu.Lock()
	s.activeEmit = wrapped
	s.mu.Unlock()
	inboxKey := turnID + "/prompt"
	if err := wrapped(nativeEvent("session.inbox.enqueued", map[string]any{
		"sessionID": native, "inboxID": inboxKey,
		"item": map[string]any{"type": "user", "payload": map[string]any{"text": prompt}, "delivery": "queue"},
	}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID, "messageID": inboxKey})); err != nil {
		return nil, fmt.Errorf("gimble: %s: record prompt: %w", s.id, err)
	}
	if err := wrapped(nativeEvent("session.inbox.delivered", map[string]any{"sessionID": native, "inboxID": inboxKey}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID, "messageID": inboxKey})); err != nil {
		return nil, fmt.Errorf("gimble: %s: deliver prompt: %w", s.id, err)
	}
	if err := wrapped(nativeEvent("session.execution.started", map[string]any{"sessionID": native}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID})); err != nil {
		return nil, fmt.Errorf("gimble: %s: start execution: %w", s.id, err)
	}
	// The turn has its own ctx, reachable by turn id through the run's table
	// while RunTurn runs, so an operator can end this one turn and leave the
	// scope running. Cancelling it is the interrupt: the adapter stops the
	// native turn and returns. The cause is read after the turn leaves the
	// table: nil while the turn's ctx is live, Killed for a kill by id, and
	// the parent's own cause (Canceled, DeadlineExceeded, or an inherited
	// Killed) when the scope or run ended around it.
	turnCtx, cancelTurn := context.WithCancelCause(ctx)
	if scope != nil {
		scope.run.addTurn(turnID, cancelTurn)
	}
	result, err := s.adapter.RunTurn(turnCtx, native, prompt, schema, wrapped)
	if scope != nil {
		scope.run.removeTurn(turnID)
	}
	stopped := context.Cause(turnCtx)
	if stopped != nil && stopped != turnCtx.Err() {
		// Keep errors.Is(err, context.Canceled) true for callers that only
		// ask whether the turn was cancelled, and errors.As for the cause.
		stopped = fmt.Errorf("%w: %w", turnCtx.Err(), stopped)
	}
	cancelTurn(nil)
	if stopped != nil {
		err = stopped
	} else if len(result.Usage) > 0 {
		// The steps already accounted for the turn's tokens. The harness's
		// own report adds only the cost it stated, so nothing is counted
		// twice, and the session republishes its total.
		var stated Usage
		for _, model := range result.Usage {
			stated.Cost += model.Cost
		}
		if usageErr := s.recordUsage(stated, native, turnID, wrapped); usageErr != nil {
			err = errors.Join(err, usageErr)
		}
	}
	// The turn's usage is what the harness reported, and otherwise what its
	// steps spent, under the model the session runs.
	report := modelUsage(map[string]Usage{s.model: turnUsage})
	if result.Usage != nil {
		report = modelUsage(result.Usage)
	}
	logf("%s: turn ended after %s: %v", s.id, time.Since(start).Round(time.Second), orNone(err))
	if stopped != nil {
		reason := "shutdown"
		var killed Killed
		if steerSource(ctx) != "" || errors.As(stopped, &killed) {
			reason = "user"
		}
		terminalErr := wrapped(nativeEvent("session.execution.interrupted", map[string]any{"sessionID": native, "reason": reason}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID}))
		err = errors.Join(stopped, terminalErr)
		if scope != nil {
			scope.run.event(scope.key, s.id, turnID, TurnEnded{Error: stopped.Error(), Usage: report, Duration: time.Since(start), Interrupted: true})
		}
		return nil, err
	}
	if err != nil {
		terminalErr := wrapped(nativeEvent("session.execution.failed", map[string]any{"sessionID": native, "error": map[string]any{"type": "provider", "message": err.Error()}}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID}))
		err = errors.Join(err, terminalErr)
		if scope != nil {
			scope.run.event(scope.key, s.id, turnID, TurnEnded{Error: err.Error(), Usage: report, Duration: time.Since(start)})
		}
		return nil, fmt.Errorf("gimble: %s: %w", s.id, err)
	}
	if terminalErr := wrapped(nativeEvent("session.execution.succeeded", map[string]any{"sessionID": native}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID})); terminalErr != nil {
		return nil, fmt.Errorf("gimble: %s: complete execution: %w", s.id, terminalErr)
	}
	// The harness succeeded, so its native events stand; only the turn's own
	// recorded outcome carries the validation failure.
	if validate != nil {
		if err := validate(result.Output); err != nil {
			if scope != nil {
				scope.run.event(scope.key, s.id, turnID, TurnEnded{Result: JSONText(result.Output), Error: err.Error(), Usage: report, Duration: time.Since(start)})
			}
			return nil, fmt.Errorf("gimble: %s: the result does not validate: %w: %w", s.id, errInvalidResult, err)
		}
	}
	if scope != nil {
		scope.run.event(scope.key, s.id, turnID, TurnEnded{Result: JSONText(result.Output), Usage: report, Duration: time.Since(start)})
	}
	return result.Output, nil
}

// stepUsage reads the usage a step event carries. A step that ended always
// accounts for itself; a step that failed does so only when it reached the
// model, which is when its data carries both cost and tokens.
func stepUsage(event AgentEvent) (Usage, bool) {
	ended := event.Type == "session.step.ended"
	if !ended && event.Type != "session.step.failed" {
		return Usage{}, false
	}
	var data struct {
		Cost   *float64 `json:"cost"`
		Tokens *Tokens  `json:"tokens"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return Usage{}, false
	}
	if !ended && (data.Cost == nil || data.Tokens == nil) {
		return Usage{}, false
	}
	var usage Usage
	if data.Cost != nil {
		usage.Cost = *data.Cost
	}
	if data.Tokens != nil {
		usage.Tokens = *data.Tokens
	}
	return usage, true
}

// recordUsage adds one usage to the session's running total and emits that
// total as session.usage.updated, through the same path the harness's own
// events take: the session's total is republished after every step and
// after every harness turn report.
func (s *Session) recordUsage(add Usage, native, turnID string, emit func(AgentEvent) error) error {
	s.mu.Lock()
	s.usage.add(add)
	total := s.usage
	s.mu.Unlock()
	return emit(nativeEvent("session.usage.updated",
		map[string]any{"sessionID": native, "cost": total.Cost, "tokens": total.Tokens},
		map[string]any{"provider": "gimble", "sessionID": native, "turnID": turnID}))
}

func nativeEvent(eventType string, data any, nativeRef any) AgentEvent {
	event := AgentEvent{Type: eventType, Data: mustJSON(data)}
	if nativeRef != nil {
		event.NativeRef = mustJSON(nativeRef)
	}
	return event
}

func (s *Session) stampAgentEvent(event AgentEvent) (AgentEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var data map[string]any
	if err := json.Unmarshal(event.Data, &data); err != nil || data == nil {
		return AgentEvent{}, fmt.Errorf("gimble: %s data is not an object", event.Type)
	}
	if s.canonicalSession == "" {
		s.canonicalSession = "ses_" + base64.RawURLEncoding.EncodeToString([]byte(s.id))
	}
	nativeSession, _ := data["sessionID"].(string)
	if nativeSession == "" {
		return AgentEvent{}, fmt.Errorf("gimble: %s has no native sessionID", event.Type)
	}
	data["sessionID"] = s.canonicalSession
	ref := make(map[string]any)
	if len(event.NativeRef) > 0 {
		if err := json.Unmarshal(event.NativeRef, &ref); err != nil {
			return AgentEvent{}, fmt.Errorf("gimble: %s NativeRef is not a JSON object: %w", event.Type, err)
		}
	}
	if ref == nil {
		return AgentEvent{}, fmt.Errorf("gimble: %s NativeRef is not a JSON object", event.Type)
	}
	allowedRef := map[string]bool{
		"provider": true, "sessionID": true, "turnID": true, "messageID": true,
		"responseID": true, "itemID": true, "parentToolUseID": true,
		"normalizedMessageID": true, "normalizedSessionID": true,
	}
	for key := range ref {
		if !allowedRef[key] {
			return AgentEvent{}, fmt.Errorf("gimble: %s NativeRef has unsupported field %q", event.Type, key)
		}
	}
	if provider, _ := ref["provider"].(string); provider == "" {
		return AgentEvent{}, fmt.Errorf("gimble: %s NativeRef has no provider", event.Type)
	}
	if _, exists := ref["sessionID"]; !exists {
		ref["sessionID"] = nativeSession
	} else if ref["sessionID"] != nativeSession {
		return AgentEvent{}, fmt.Errorf("gimble: %s NativeRef sessionID does not match data", event.Type)
	}
	nextEventSeq := s.eventSeq + 1
	for _, key := range []string{"assistantMessageID", "inboxID"} {
		if nativeID, present := data[key].(string); present {
			if nativeID == "" {
				return AgentEvent{}, fmt.Errorf("gimble: %s has an empty %s", event.Type, key)
			}
			canonical := s.messageIDLocked(nativeID, nextEventSeq)
			data[key] = canonical
			if err := bindMessageRef(event.Type, ref, nativeID, canonical); err != nil {
				return AgentEvent{}, err
			}
		}
	}
	if event.Type == "session.step.started" {
		data["agent"] = s.name
	}
	event.ID = fmt.Sprintf("evt_%s_%020d", base64.RawURLEncoding.EncodeToString([]byte(s.id)), nextEventSeq)
	event.Created = time.Now().UnixMilli()
	event.Data = mustJSON(data)
	event.NativeRef = mustJSON(ref)
	s.eventSeq = nextEventSeq
	return event, nil
}

func (s *Session) messageIDLocked(native string, eventSeq uint64) string {
	if s.messageIDs == nil {
		s.messageIDs = make(map[string]string)
	}
	if id := s.messageIDs[native]; id != "" {
		return id
	}
	id := fmt.Sprintf("msg_%s_%020d", base64.RawURLEncoding.EncodeToString([]byte(s.id)), eventSeq)
	s.messageIDs[native] = id
	return id
}

func bindMessageRef(eventType string, ref map[string]any, native, canonical string) error {
	if _, exists := ref["messageID"]; !exists {
		ref["messageID"] = native
	} else if ref["messageID"] != native {
		return fmt.Errorf("gimble: %s NativeRef messageID does not match data", eventType)
	}
	ref["normalizedMessageID"] = canonical
	return nil
}

// usable reports why s cannot start a turn. The caller holds s.mu.
func (s *Session) usable() error {
	switch {
	case s.id == "":
		return fmt.Errorf("gimble: session %q was not created in a run", s.name)
	case s.closed:
		return fmt.Errorf("gimble: session %s was used after its scope ended", s.id)
	case s.running:
		return fmt.Errorf("gimble: session %s is already running a turn", s.id)
	}
	return nil
}

// Steer injects a message into the turn that is running on this session.
// It is called from another goroutine while Generate blocks. The message
// lands at the worker's next model call, and landed reports that it did.
// A steer with no turn to receive it is dropped, not failed: landed is
// false and the error is nil, whether no turn was running here or the
// turn ended inside the harness while the steer was on its way. The Steer
// lifecycle record carries the same outcome. The error is for a harness
// that could not be reached.
func (s *Session) Steer(ctx context.Context, message string) (landed bool, err error) {
	s.mu.Lock()
	native, running, turn, emit := s.native, s.running, s.turnID, s.activeEmit
	s.mu.Unlock()
	scope, _ := current(ctx)
	record := func(landed bool) {
		if scope != nil {
			scope.run.event(scope.key, s.id, turn, Steer{Target: s.id, Source: steerSource(ctx), Message: message, Landed: landed})
		}
	}
	if !running || native == "" {
		logf("%s: steer dropped, no turn is running: %s", s.id, oneLine(message))
		record(false)
		return false, nil
	}
	inboxKey := turn + fmt.Sprintf("/steer.%d", time.Now().UnixNano())
	if emit != nil {
		if err := emit(nativeEvent("session.inbox.enqueued", map[string]any{
			"sessionID": native, "inboxID": inboxKey,
			"item": map[string]any{"type": "user", "payload": map[string]any{"text": message}, "delivery": "steer"},
		}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turn, "messageID": inboxKey})); err != nil {
			return false, err
		}
	}
	landed, err = s.adapter.Steer(ctx, native, message)
	switch {
	case err != nil:
		landed = false
		logf("%s: steer failed: %v: %s", s.id, err, oneLine(message))
	case landed:
		logf("%s: steer landed: %s", s.id, oneLine(message))
	default:
		logf("%s: steer dropped, the turn ended first: %s", s.id, oneLine(message))
	}
	record(landed)
	if emit != nil {
		eventType := "session.inbox.delivered"
		if !landed {
			eventType = "session.inbox.cancelled"
		}
		emitErr := emit(nativeEvent(eventType, map[string]any{"sessionID": native, "inboxID": inboxKey}, map[string]any{"provider": "gimble", "sessionID": native, "turnID": turn, "messageID": inboxKey}))
		err = errors.Join(err, emitErr)
	}
	return landed, err
}

type steerSourceKey struct{}

func withSteerSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, steerSourceKey{}, source)
}
func steerSource(ctx context.Context) string {
	source, _ := ctx.Value(steerSourceKey{}).(string)
	return source
}

// Fork returns a new session, named for the role it takes on, in the same
// workdir, with the same conversation so far. A fork continues its parent's
// conversation, so it runs on its parent's binding and the run binds
// nothing for it. The two sessions are independent after that.
func (s *Session) Fork(ctx context.Context, name string) (*Session, error) {
	scope, err := current(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	err = s.usable()
	native := s.native
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	fork := &Session{adapter: s.adapter, name: name, model: s.model, effort: s.effort, workdir: s.workdir}
	if native != "" {
		if fork.native, err = s.adapter.Fork(ctx, native); err != nil {
			return nil, fmt.Errorf("gimble: fork %s: %w", s.id, err)
		}
	}
	scope.adopt(fork)
	scope.run.event(scope.key, fork.id, "", SessionCreated{Name: name, Adapter: fmt.Sprintf("%T", fork.adapter), Model: fork.model, Workdir: fork.workdir, Parent: s.id})
	logf("%s: forked from %s", fork.id, s.id)
	return fork, nil
}

func oneLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}
