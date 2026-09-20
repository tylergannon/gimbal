// Package claude is Gimble's HarnessAdapter for Claude Code, through the
// Claude Agent SDK. Each turn launches Claude Code against the session id;
// the conversation is Claude Code's to keep.
package claude

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"strings"
	"sync"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/tylergannon/gimble"
)

const controlTimeout = 5 * time.Second

// autoCompactWindow sets Claude Code's auto-compact window so long turns
// don't get cut off mid-task. It is a token count with no suffix: the CLI
// does not parse "256k" and falls back to a 100k window, which with its 33k
// compaction buffer and a 43k base context left a turn about 25k tokens of
// room and made compaction thrash until the turn failed.
const autoCompactWindow = "256000"

// adapter runs Claude Code sessions.
type adapter struct {
	userConfiguration bool

	mu       sync.Mutex
	sessions map[string]*session
}

// Option configures the harness.
type Option func(*adapter)

// WithUserConfiguration lets a workflow's sessions load the user's own
// Claude Code configuration: the MCP servers, plugins and settings under
// ~/.claude. Ask for it only when the workflow's agents use those tools.
// Every one of them is paid for on every request of every session: the
// desktop integrations on the machine this was written on cost about 43k
// tokens of tool definitions before a coder had read a line of the
// repository.
func WithUserConfiguration() Option {
	return func(a *adapter) { a.userConfiguration = true }
}

type session struct {
	model   string
	effort  string
	workdir string

	mu     sync.Mutex
	fresh  bool   // no native conversation yet
	parent string // for a fork, the session to fork from on the first turn
	active *activeTurn
}

type activeTurn struct {
	stream *claudeagent.Stream
	emit   *projector
}

// New returns Gimble's Claude Code harness. It launches Claude Code when a
// session first needs it. Its sessions start on the CLI's own tools and the
// repository's configuration, not the user's, unless a workflow asks for the
// user's with WithUserConfiguration.
func New(options ...Option) gimble.HarnessAdapter {
	a := &adapter{sessions: make(map[string]*session)}
	for _, option := range options {
		option(a)
	}
	return a
}

// settingSources are the settings a session loads when it is not asking for
// the user's own configuration: the repository's, and the checkout's own.
// The user source, ~/.claude, is left out, so a coder does not inherit the
// plugins and MCP servers of whoever started the run.
const settingSources = "project,local"

// isolate keeps a session on the CLI's own tools and the repository's
// configuration. Without it a session inherits every MCP server and plugin
// the user has installed and pays for their tool definitions on every
// request, none of which a coder in a repository uses.
func isolate(extra map[string]*string) []claudeagent.Option {
	sources := settingSources
	extra["setting-sources"] = &sources
	return []claudeagent.Option{claudeagent.WithStrictMCPConfig(true)}
}

// CreateSession mints the id the first turn passes as --session-id.
func (a *adapter) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	return a.add(&session{model: model, effort: effort, workdir: workdir, fresh: true})
}

// Fork mints an id for a new session that forks from sessionID on its
// first turn, with --resume and --fork-session.
func (a *adapter) Fork(ctx context.Context, sessionID string) (string, error) {
	parent, err := a.session(sessionID)
	if err != nil {
		return "", err
	}
	return a.add(&session{model: parent.model, effort: parent.effort, workdir: parent.workdir, parent: sessionID})
}

func (a *adapter) add(s *session) (string, error) {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return "", fmt.Errorf("claude: mint a session id: %w", err)
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
	a.mu.Lock()
	a.sessions[id] = s
	a.mu.Unlock()
	return id, nil
}

// RunTurn runs one turn and blocks until it ends.
func (a *adapter) RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return gimble.TurnResult{}, err
	}
	s.mu.Lock()
	fresh, parent := s.fresh, s.parent
	s.mu.Unlock()

	nativeErrors := make(chan error, 1)
	project := newProjector(sessionID, s.model, onEvent)
	options := []claudeagent.Option{
		claudeagent.WithCwd(s.workdir),
		claudeagent.WithModel(s.model),
		claudeagent.WithIncludePartialMessages(true),
		claudeagent.WithPermissionMode(claudeagent.PermissionModeBypassAll),
		claudeagent.WithAllowDangerouslySkipPermissions(true),
		claudeagent.WithEnv(map[string]string{"CLAUDE_CODE_AUTO_COMPACT_WINDOW": autoCompactWindow}),
		claudeagent.WithRawMessageObserver(func(raw json.RawMessage) error {
			err := project.raw(raw)
			if err != nil {
				select {
				case nativeErrors <- err:
				default:
				}
			}
			return err
		}),
		claudeagent.WithStderr(func(data string) {
			if err := fatalStderr(data); err != nil {
				select {
				case nativeErrors <- err:
				default:
				}
			}
		}),
	}
	extra := map[string]*string{}
	if !a.userConfiguration {
		options = append(options, isolate(extra)...)
	}
	nativeSchema, err := completionSchema(schema)
	if err != nil {
		return gimble.TurnResult{}, fmt.Errorf("claude: compose completion schema: %w", err)
	}
	text := string(nativeSchema)
	extra["json-schema"] = &text
	switch {
	case parent != "":
		options = append(options, claudeagent.WithForkSession(parent))
		extra["session-id"] = &sessionID
	case fresh:
		extra["session-id"] = &sessionID
	default:
		options = append(options, claudeagent.WithResume(sessionID))
	}
	if s.effort != "" {
		options = append(options, claudeagent.WithEffort(claudeagent.EffortLevel(s.effort)))
	}
	options = append(options, claudeagent.WithExtraArgs(extra))

	// The process is not tied to ctx: on cancellation the turn is
	// interrupted first so Claude Code can wind down, then closed.
	processCtx, stop := context.WithCancel(context.Background())
	defer stop()
	client, err := claudeagent.NewClient(options...)
	if err != nil {
		return gimble.TurnResult{}, fmt.Errorf("claude: %w", err)
	}
	defer func() { _ = client.Close() }()
	stream, err := client.Stream(processCtx)
	if err != nil {
		return gimble.TurnResult{}, fmt.Errorf("claude: %w", err)
	}
	defer func() { _ = stream.Close() }()

	active := &activeTurn{stream: stream, emit: project}
	s.setActive(active)
	defer s.setActive(nil)
	if err := stream.Send(processCtx, prompt); err != nil {
		return gimble.TurnResult{}, fmt.Errorf("claude: %w", err)
	}

	out, err := waitTurn(ctx, stream, nativeErrors, sessionID, s)
	if ctx.Err() != nil {
		return gimble.TurnResult{}, ctx.Err()
	}
	if err != nil {
		return gimble.TurnResult{}, err
	}
	usage := project.turnUsage()
	return gimble.TurnResult{Output: out, Usage: usage}, nil
}

// Steer sends message on the running turn's live SDK stream and reports
// whether it landed. With no stream live, or when the send fails because
// the turn ended while the steer was on its way, the message is dropped:
// false and no error.
func (a *adapter) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	s, err := a.session(sessionID)
	if err != nil {
		return false, err
	}
	active := s.getActive()
	if active == nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, controlTimeout)
	defer cancel()
	if err := active.stream.Send(ctx, message); err != nil {
		if s.getActive() != active {
			return false, nil // the turn ended first: the message is dropped
		}
		return false, fmt.Errorf("claude: %w", err)
	}
	return true, nil
}

// Close forgets sessionID. Claude Code's own process is already gone by
// the time Close runs: RunTurn's process is scoped to one turn, not the
// session. Idempotent: an unknown id returns nil.
func (a *adapter) Close(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	return nil
}

func (a *adapter) session(id string) (*session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.sessions[id]; s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("claude: no session %q", id)
}

func (s *session) setActive(active *activeTurn) {
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
}

func (s *session) getActive() *activeTurn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// waitTurn reads the turn to its result. When ctx ends first it interrupts
// the turn and waits briefly for Claude Code to wind down.
type turnStream interface {
	Messages() iter.Seq[claudeagent.Message]
	InterruptWithReceipt(context.Context) (*claudeagent.InterruptReceipt, error)
}

// invalidCompletion is intentionally malformed JSON. Returning it as the
// turn's output sends a missing or malformed private completion through
// Generate's existing bounded validation retry, regardless of the caller's
// schema (including schemas that accept null).
var invalidCompletion = json.RawMessage("{")

// waitTurn reads one submitted prompt through any explicit waiting results and
// native automatic continuations. A resumed process can first replay orphan
// results from work owned by an earlier process. The process's init followed by
// status=requesting is the bounded post-submit barrier: origin and result index
// describe a generation, but do not prove that the queued prompt was received.
func waitTurn(ctx context.Context, stream turnStream, nativeErrors <-chan error, sessionID string, s *session) (json.RawMessage, error) {
	messages := make(chan claudeagent.Message)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		defer close(messages)
		for message := range stream.Messages() {
			select {
			case messages <- message:
			case <-stop:
				return
			}
		}
	}()

	done := ctx.Done()
	var grace <-chan time.Time
	initialized := false
	promptReceived := false
	canceling := false
	for {
		select {
		case err := <-nativeErrors:
			if err != nil {
				return nil, err
			}
		case <-done:
			canceling = true
			done = nil
			interruptCtx, cancel := context.WithTimeout(context.Background(), controlTimeout)
			_, _ = stream.InterruptWithReceipt(interruptCtx)
			cancel()
			grace = time.After(controlTimeout)
		case <-grace:
			return nil, ctx.Err()
		case message, ok := <-messages:
			if !ok {
				if canceling {
					return nil, ctx.Err()
				}
				return nil, errors.New("claude: the stream ended without a completed result")
			}
			envelope := decodeEnvelope(message)
			if envelope.SessionID != "" && envelope.SessionID != sessionID {
				return nil, fmt.Errorf("claude: got session %s, want %s", envelope.SessionID, sessionID)
			}
			if envelope.SessionID != "" && (envelope.Type != "system" || envelope.Subtype != "init") {
				s.mu.Lock()
				s.fresh, s.parent = false, "" // the native conversation exists now
				s.mu.Unlock()
			}
			if err := assistantError(message); err != nil {
				return nil, err
			}
			if envelope.Type == "system" && envelope.Subtype == "init" {
				initialized = true
			}
			if initialized && requesting(message) {
				promptReceived = true
			}
			if result, ok := asResult(message); ok {
				if canceling {
					return nil, ctx.Err()
				}
				if result.Subtype != "success" && result.Status != "success" {
					return nil, errors.New(resultFailure(result))
				}
				if !promptReceived {
					continue
				}
				state, value, valid := completionValue(result.StructuredOutput)
				if !valid {
					return invalidCompletion, nil
				}
				switch state {
				case "waiting":
					continue
				case "completed":
					return value, nil
				default:
					return invalidCompletion, nil
				}
			}
		}
	}
}

func completionValue(value any) (string, json.RawMessage, bool) {
	raw, err := json.Marshal(value)
	if err != nil || value == nil {
		return "", nil, false
	}
	var completion struct {
		State   string          `json:"state"`
		Message *string         `json:"message"`
		Value   json.RawMessage `json:"value"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&completion); err != nil {
		return "", nil, false
	}
	switch completion.State {
	case "waiting":
		// Native structured output cannot express conditional fields without
		// top-level combinators. A model may include a provisional caller value
		// while explicitly waiting; it is ignored and can never complete the turn.
		return completion.State, nil, completion.Message != nil
	case "completed":
		return completion.State, completion.Value, completion.Value != nil
	default:
		return completion.State, nil, false
	}
}

func requesting(message claudeagent.Message) bool {
	switch status := message.(type) {
	case claudeagent.StatusMessage:
		return status.Status != nil && *status.Status == claudeagent.SDKStatusRequesting
	case *claudeagent.StatusMessage:
		return status.Status != nil && *status.Status == claudeagent.SDKStatusRequesting
	default:
		return false
	}
}

// completionSchema embeds the caller's schema under an adapter-private
// completion envelope. Local references keep their caller-schema meaning:
// root definitions are hoisted, other root references are redirected to the
// embedded caller root, and a collision-free private definition name is used.
// Text receives the same lifecycle with a string as its final value.
func completionSchema(schema json.RawMessage) (json.RawMessage, error) {
	var caller any = map[string]any{"type": "string"}
	if len(schema) > 0 {
		if err := json.Unmarshal(schema, &caller); err != nil {
			return nil, err
		}
	}
	root, isObject := caller.(map[string]any)
	defs := make(map[string]any)
	var legacyDefinitions map[string]any
	name := "__gimble_completion_value"
	if isObject {
		if existing, ok := root["$defs"].(map[string]any); ok {
			maps.Copy(defs, existing)
		}
		for {
			if _, exists := defs[name]; !exists {
				break
			}
			name += "_"
		}
		if _, hasID := root["$id"]; !hasID {
			rewriteLocalRefs(root, name, false)
			delete(root, "$defs")
			if legacy, ok := root["definitions"].(map[string]any); ok {
				// Legacy definitions use a separate JSON-pointer namespace but
				// still have to remain at the composed document root.
				legacyDefinitions = make(map[string]any, len(legacy))
				maps.Copy(legacyDefinitions, legacy)
				delete(root, "definitions")
			}
		}
	}
	defs[name] = caller
	valueRef := map[string]any{"$ref": "#/$defs/" + name}
	composed := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"state": map[string]any{
				"type": "string", "enum": []string{"waiting", "completed"},
				"description": "Use waiting while required work is pending, then completed only when the whole assignment is done.",
			},
			"message": map[string]any{"type": "string", "description": "Required for waiting: a short witness of what remains pending. Omit when completed."},
			"value":   valueRef,
		},
		"required":             []string{"state"},
		"additionalProperties": false,
		"$defs":                defs,
	}
	if isObject {
		if dialect, ok := root["$schema"]; ok {
			composed["$schema"] = dialect
			delete(root, "$schema")
		}
	}
	if legacyDefinitions != nil {
		composed["definitions"] = legacyDefinitions
	}
	return json.Marshal(composed)
}

func rewriteLocalRefs(value any, rootName string, nestedResource bool) {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			rewriteLocalRefs(item, rootName, nestedResource)
		}
	case map[string]any:
		if _, ok := value["$id"]; ok {
			nestedResource = true
		}
		for key, item := range value {
			if key == "properties" || key == "patternProperties" || key == "dependentSchemas" || key == "$defs" || key == "definitions" {
				rewriteSchemaNameMap(item, rootName, nestedResource)
				continue
			}
			// These keywords contain caller instances, not subschemas. In
			// particular, an object with a "$ref" member under const/default is
			// literal data and changing it changes the caller's contract.
			if key == "const" || key == "default" || key == "enum" || key == "examples" {
				continue
			}
			if !nestedResource && (key == "$ref" || key == "$dynamicRef" || key == "$recursiveRef") {
				// A fragment beginning "#/" is a JSON Pointer into the caller's
				// old document root. A fragment such as "#node" is a named anchor;
				// relocation does not change its anchor identity, so preserve it.
				if ref, ok := item.(string); ok && (ref == "#" || strings.HasPrefix(ref, "#/")) && ref != "#/$defs" && !strings.HasPrefix(ref, "#/$defs/") && ref != "#/definitions" && !strings.HasPrefix(ref, "#/definitions/") {
					value[key] = "#/$defs/" + rootName + strings.TrimPrefix(ref, "#")
				}
			}
			rewriteLocalRefs(item, rootName, nestedResource)
		}
	}
}

// rewriteSchemaNameMap walks the values of maps whose keys are caller-chosen
// names. A property or definition named "default" or "$id" is not itself a
// schema keyword; only its value is a schema node.
func rewriteSchemaNameMap(value any, rootName string, nestedResource bool) {
	named, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, schema := range named {
		rewriteLocalRefs(schema, rootName, nestedResource)
	}
}

type envelope struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
}

func decodeEnvelope(message claudeagent.Message) envelope {
	var e envelope
	if raw, err := json.Marshal(message); err == nil {
		_ = json.Unmarshal(raw, &e)
	}
	return e
}

// assistantError is the error of a failed assistant message. Claude Code
// states the code on the message and its own explanation of the failure as
// the message's text; both belong in the error, so a reader of the run does
// not have to open the CLI's transcript to learn why the turn failed.
func assistantError(message claudeagent.Message) error {
	var code claudeagent.AssistantMessageError
	var requestID, explanation string
	switch assistant := message.(type) {
	case claudeagent.AssistantMessage:
		code, requestID, explanation = assistant.Error, assistant.RequestID, blockText(assistant.Message.Content)
	case *claudeagent.AssistantMessage:
		code, requestID, explanation = assistant.Error, assistant.RequestID, blockText(assistant.Message.Content)
	}
	if code == "" {
		return nil
	}
	text := "claude: assistant error: " + string(code)
	if explanation != "" {
		text += ": " + explanation
	}
	if requestID != "" {
		text += " (request ID: " + requestID + ")"
	}
	return errors.New(text)
}

// blockText is the text Claude Code wrote on a message, as one line.
func blockText(blocks []claudeagent.ContentBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return oneLine(strings.Join(parts, " "))
}

func asResult(message claudeagent.Message) (claudeagent.ResultMessage, bool) {
	switch result := message.(type) {
	case claudeagent.ResultMessage:
		return result, true
	case *claudeagent.ResultMessage:
		return *result, true
	}
	return claudeagent.ResultMessage{}, false
}

func resultFailure(result claudeagent.ResultMessage) string {
	parts := append([]string(nil), result.Errors...)
	if result.TerminalReason != nil {
		parts = append(parts, string(*result.TerminalReason))
	}
	if len(parts) == 0 {
		parts = append(parts, result.Subtype)
	}
	return "claude: the turn failed: " + strings.Join(parts, "; ")
}

func fatalStderr(data string) error {
	lower := strings.ToLower(data)
	for _, marker := range []string{"no conversation found", "invalid model", "unknown model"} {
		if strings.Contains(lower, marker) {
			return errors.New("claude: " + strings.TrimSpace(data))
		}
	}
	return nil
}

var _ gimble.HarnessAdapter = (*adapter)(nil)
