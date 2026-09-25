// Package agy implements Gimbal's HarnessAdapter using Google Antigravity's
// `agy` print-mode CLI.
package agy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tylergannon/gimbal/harness"
	"github.com/tylergannon/gimbal/harness/agy/schema"
)

const (
	controlTimeout = 5 * time.Second
	createTimeout  = 2 * time.Minute
	// printBackstop bounds a native process when the caller's ctx has no
	// deadline; agy insists on a --print-timeout.
	printBackstop = 24 * time.Hour
)

var errInterrupted = errors.New("turn was interrupted")

type runnerConfig struct {
	binary   string
	baseArgs []string
	env      []string
	homeDir  string // override for ensureNativeWriteHook; "" means os.UserHomeDir()
}

// Adapter translates the neutral harness contract to agy's stream-json CLI.
// Every turn is one `agy -p` process that resumes the native conversation.
type Adapter struct {
	mu       sync.Mutex
	closed   bool
	stderr   io.Writer
	states   map[string]*sessionState
	config   runnerConfig
	hookOnce sync.Once
	hookErr  error
}

type sessionState struct {
	ops     sync.Mutex // serializes turns and compaction on this session
	mu      sync.Mutex // guards the fields below
	workdir string
	active  *activeTurn
	// agyConversationID overrides the external session ID as the actual
	// --conversation argument once an artifact-path repair has moved this
	// session onto a fresh agy conversation. Empty means unchanged.
	agyConversationID string
	pendingSteer      [][]harness.ContentPart
}

type activeTurn struct {
	command     *exec.Cmd
	done        chan struct{}
	interrupted atomic.Bool
	steered     atomic.Bool
}

type runRequest struct {
	prompt          string
	model           string
	effort          string
	workdir         string
	sessionID       string
	newProject      bool
	outputSchema    string
	projector       *eventProjector
	requireResultID bool
}

type nativeResult struct {
	conversationID string
	status         string
	errorMessage   string
	response       string
	structured     any
	echoedSchema   any
}

// New constructs an adapter that launches `agy` from PATH.
func New() *Adapter {
	return newAdapter(runnerConfig{binary: "agy", env: os.Environ()})
}

func newAdapter(config runnerConfig) *Adapter {
	return &Adapter{states: make(map[string]*sessionState), config: config}
}

// SetStderr forwards agy stderr. It must be called before use.
func (a *Adapter) SetStderr(stderr io.Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stderr = stderr
}

// CreateSession performs a short native turn so agy mints and persists a real
// conversation bound to workdir.
func (a *Adapter) CreateSession(model, workdir string) (string, error) {
	if err := harness.ValidateCreateSessionInput(model, workdir); err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), createTimeout)
	defer cancel()
	result, _, err := a.runOnce(ctx, nil, runRequest{
		prompt:          "Reply with the single word OK. Do not use any tools.",
		model:           model,
		workdir:         absolute,
		newProject:      true,
		requireResultID: true,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.conversationID) == "" {
		return "", errors.New("agy create result omitted conversation_id")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return "", errors.New("agy adapter is closed")
	}
	a.states[result.conversationID] = &sessionState{workdir: absolute}
	return result.conversationID, nil
}

// RunTurn runs one turn on the session and blocks until it ends. With an
// output schema the structured result is validated against it exactly, with
// one repair turn for nonconforming output; without one the assistant's
// response text is returned. Cancelling ctx interrupts the native process.
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
	absolute, err := filepath.Abs(input.Workdir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	state, err := a.state(input.SessionID)
	if err != nil {
		return nil, err
	}
	state.ops.Lock()
	defer state.ops.Unlock()
	if err := state.bind(absolute); err != nil {
		return nil, err
	}

	request := runRequest{
		model:           input.Model,
		effort:          input.ReasoningEffort,
		workdir:         absolute,
		sessionID:       state.conversationID(input.SessionID),
		projector:       newEventProjector(onEvent),
		requireResultID: true,
	}
	if validator != nil {
		schemaFile, err := writeSchema(input.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("write agy output schema: %w", err)
		}
		defer func() { _ = os.Remove(schemaFile) }()
		request.outputSchema = schemaFile
	}
	request.projector.user(input.Parts)
	originalPrompt := artifactMetadataPromptPreamble + joinParts(input.Parts)
	request.prompt = originalPrompt

	result, err := a.run(ctx, state, request)
	if err != nil && isArtifactPathError(err) && ctx.Err() == nil {
		// Do not resume the conversation that just failed: once a
		// conversation has taken one artifact-path failure, resumed turns
		// keep reporting that stale error. Start a fresh conversation with
		// the original prompt plus a corrective note and, on success, adopt
		// it as this session's live conversation.
		request.prompt = artifactWriteRepairPrompt(originalPrompt, err.Error())
		request.projector.user([]harness.ContentPart{{Type: harness.ContentPartText, Text: request.prompt}})
		request.sessionID = ""
		request.newProject = true
		result, err = a.run(ctx, state, request)
		if err == nil {
			state.adoptConversationID(result.conversationID)
			request.sessionID = result.conversationID
			request.newProject = false
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if validator == nil {
		return harness.TextResult(result.response)
	}

	if err := compareEchoedSchema(input.OutputSchema, result.echoedSchema); err != nil {
		return nil, err
	}
	raw, invalid := validateStructured(validator, result.structured)
	if invalid == nil {
		return raw, nil
	}
	request.prompt = fmt.Sprintf("Your previous structured output did not conform to the required JSON schema: %s. Respond again. Output only a JSON object conforming exactly to the schema. Do not run any tools.", invalid.Error())
	request.projector.user([]harness.ContentPart{{Type: harness.ContentPartText, Text: request.prompt}})
	result, err = a.run(ctx, state, request)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if err := compareEchoedSchema(input.OutputSchema, result.echoedSchema); err != nil {
		return nil, err
	}
	raw, invalid = validateStructured(validator, result.structured)
	if invalid != nil {
		return nil, fmt.Errorf("agy result does not conform to the supplied schema: %w", invalid)
	}
	return raw, nil
}

// Steer interrupts the active print process and resumes the native
// conversation with the steering message inside the same RunTurn.
func (a *Adapter) Steer(sessionID string, parts []harness.ContentPart) {
	if harness.ValidateContentParts(parts) != nil {
		return
	}
	a.mu.Lock()
	state := a.states[sessionID]
	a.mu.Unlock()
	if state == nil {
		return
	}
	state.mu.Lock()
	active := state.active
	if active != nil {
		state.pendingSteer = append(state.pendingSteer, append([]harness.ContentPart(nil), parts...))
		active.steered.Store(true)
	}
	state.mu.Unlock()
	if active != nil {
		interruptProcess(active)
	}
}

// Interrupt signals the active native process and returns immediately.
func (a *Adapter) Interrupt(sessionID string) {
	a.mu.Lock()
	state := a.states[sessionID]
	a.mu.Unlock()
	if state == nil {
		return
	}
	state.mu.Lock()
	active := state.active
	state.mu.Unlock()
	if active != nil {
		interruptProcess(active)
	}
}

// Compact asks agy to compact the native conversation with its /compact
// command.
func (a *Adapter) Compact(sessionID, workdir string) error {
	if err := harness.ValidateSessionInput(sessionID, workdir); err != nil {
		return err
	}
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	state, err := a.state(sessionID)
	if err != nil {
		return err
	}
	if !state.ops.TryLock() {
		return errors.New("cannot compact a session with an active turn")
	}
	defer state.ops.Unlock()
	if err := state.bind(absolute); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), createTimeout)
	defer cancel()
	_, _, err = a.runOnce(ctx, state, runRequest{
		prompt:          "/compact",
		workdir:         absolute,
		sessionID:       state.conversationID(sessionID),
		requireResultID: true,
	})
	return err
}

// Close interrupts all active agy processes.
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
			interruptProcess(active)
		}
	}
}

func (a *Adapter) state(sessionID string) (*sessionState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, errors.New("agy adapter is closed")
	}
	state := a.states[sessionID]
	if state == nil {
		state = &sessionState{}
		a.states[sessionID] = state
	}
	return state, nil
}

func (s *sessionState) bind(workdir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workdir != "" && s.workdir != workdir {
		return errors.New("session working directory cannot change")
	}
	s.workdir = workdir
	return nil
}

// conversationID returns the agy conversation to resume for this session:
// the repaired one if an artifact-path repair adopted it, else external.
func (s *sessionState) conversationID(external string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agyConversationID != "" {
		return s.agyConversationID
	}
	return external
}

func (s *sessionState) adoptConversationID(id string) {
	s.mu.Lock()
	s.agyConversationID = id
	s.mu.Unlock()
}

func (s *sessionState) takeSteer() []harness.ContentPart {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingSteer) == 0 {
		return nil
	}
	parts := s.pendingSteer[0]
	s.pendingSteer = s.pendingSteer[1:]
	return parts
}

// run executes request and, whenever a steering message arrives, resumes the
// same conversation with it until no steer is pending.
func (a *Adapter) run(ctx context.Context, state *sessionState, request runRequest) (nativeResult, error) {
	for {
		result, active, err := a.runOnce(ctx, state, request)
		parts := state.takeSteer()
		if len(parts) == 0 {
			return result, err
		}
		steerInterrupted := active != nil && active.steered.Load() && errors.Is(err, errInterrupted)
		if err != nil && !steerInterrupted {
			return nativeResult{}, err
		}
		request.projector.user(parts)
		request.prompt = joinParts(parts)
	}
}

// runOnce launches one `agy -p` process, streams its events, and returns its
// result. When state is non-nil the process is published as the session's
// active turn so Steer and Interrupt can reach it.
func (a *Adapter) runOnce(ctx context.Context, state *sessionState, request runRequest) (nativeResult, *activeTurn, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nativeResult{}, nil, errors.New("agy adapter is closed")
	}
	config := a.config
	stderrWriter := a.stderr
	a.mu.Unlock()
	if err := a.ensureHook(); err != nil {
		return nativeResult{}, nil, err
	}

	args := append([]string(nil), config.baseArgs...)
	args = append(args, "-p", request.prompt, "--output-format", "stream-json", "--dangerously-skip-permissions", "--add-dir", request.workdir)
	if request.newProject {
		args = append(args, "--new-project")
	} else {
		args = append(args, "--conversation", request.sessionID)
	}
	if request.model != "" {
		args = append(args, "--model", request.model)
	}
	if request.effort != "" && !modelIncludesEffort(request.model) {
		args = append(args, "--effort", request.effort)
	}
	if request.outputSchema != "" {
		args = append(args, "--json-schema", request.outputSchema)
	}
	backstop := printBackstop
	if deadline, ok := ctx.Deadline(); ok {
		backstop = time.Until(deadline) + 30*time.Second
	}
	args = append(args, "--print-timeout", backstop.String())

	command := exec.Command(config.binary, args...)
	command.Dir = request.workdir
	command.Env = config.env
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nativeResult{}, nil, err
	}
	var stderr bytes.Buffer
	if stderrWriter != nil {
		command.Stderr = io.MultiWriter(stderrWriter, &stderr)
	} else {
		command.Stderr = &stderr
	}
	if err := command.Start(); err != nil {
		return nativeResult{}, nil, err
	}
	active := &activeTurn{command: command, done: make(chan struct{})}
	if state != nil {
		state.mu.Lock()
		state.active = active
		state.mu.Unlock()
		defer func() {
			state.mu.Lock()
			if state.active == active {
				state.active = nil
			}
			state.mu.Unlock()
		}()
	}
	envelopes := make(chan schema.Envelope)
	go scanStream(stdout, envelopes)
	waited := make(chan error, 1)
	go func() {
		waited <- command.Wait()
		close(active.done)
	}()

	ctxDone := ctx.Done()
	var result *nativeResult
	initSeen := false
	initID := ""
	var processErr, protocolErr error
	for envelopes != nil || waited != nil {
		select {
		case envelope, ok := <-envelopes:
			if !ok {
				envelopes = nil
				continue
			}
			if request.projector != nil {
				request.projector.envelope(envelope)
			}
			if envelope.Event == "init" {
				initSeen = true
				initID = envelopeConversationID(envelope)
				if request.sessionID != "" && initID != request.sessionID {
					interruptProcess(active)
					protocolErr = errors.New("agy init returned a different conversation ID")
				}
			}
			if envelope.Event == "result" && envelope.Result != nil {
				parsed := nativeResult{
					conversationID: envelopeConversationID(envelope),
					status:         stringValue(envelope.Result.Status),
					errorMessage:   stringValue(envelope.Result.Error),
					response:       stringValue(envelope.Result.Response),
					structured:     envelope.Result.StructuredOutput,
					echoedSchema:   envelope.Result.JsonSchema,
				}
				if parsed.status == "SUCCESS" && request.sessionID != "" && parsed.conversationID != request.sessionID {
					interruptProcess(active)
					protocolErr = errors.New("agy result returned a different conversation ID")
				}
				result = &parsed
			}
		case waitErr := <-waited:
			waited = nil
			processErr = waitErr
		case <-ctxDone:
			ctxDone = nil
			interruptProcess(active)
		}
	}
	if protocolErr != nil {
		return nativeResult{}, active, protocolErr
	}
	if active.interrupted.Load() {
		return nativeResult{}, active, errInterrupted
	}
	if result == nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" && processErr != nil {
			message = processErr.Error()
		}
		if message == "" {
			message = "agy stream ended without a result"
		}
		return nativeResult{}, active, errors.New(message)
	}
	if result.status != "SUCCESS" {
		message := result.errorMessage
		if message == "" {
			message = fmt.Sprintf("agy turn ended with status %q", result.status)
		}
		if result.status == "CANCELED" || result.status == "INTERRUPTED" {
			return nativeResult{}, active, fmt.Errorf("%w: %s", errInterrupted, message)
		}
		return nativeResult{}, active, errors.New(message)
	}
	if request.requireResultID && (!initSeen || result.conversationID == "") {
		return nativeResult{}, active, errors.New("agy stream omitted the required conversation ID")
	}
	if request.requireResultID && initID != result.conversationID {
		return nativeResult{}, active, errors.New("agy init and result returned different conversation IDs")
	}
	return *result, active, nil
}

func scanStream(reader io.Reader, output chan<- schema.Envelope) {
	defer close(output)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var envelope schema.Envelope
		if json.Unmarshal(scanner.Bytes(), &envelope) == nil && envelope.Event != "" {
			output <- envelope
		}
	}
}

func interruptProcess(active *activeTurn) {
	if active == nil || active.command == nil || active.command.Process == nil {
		return
	}
	active.interrupted.Store(true)
	_ = active.command.Process.Signal(os.Interrupt)
	go func() {
		select {
		case <-active.done:
		case <-time.After(controlTimeout):
			_ = active.command.Process.Kill()
		}
	}()
}

// ensureHook provisions, once per Adapter, the Gimbal-owned PreToolUse hook
// that blocks agy's native write tools from targeting workspace files with
// ArtifactMetadata. See native_write_hook.go for the mechanism. It first
// checks the installed agy meets minSupportedAgyVersion, the version the hook
// was verified live against, and fails fast otherwise.
func (a *Adapter) ensureHook() error {
	a.hookOnce.Do(func() {
		if err := verifyAgyHookSupport(a.config); err != nil {
			a.hookErr = err
			return
		}
		home := a.config.homeDir
		if home == "" {
			resolved, err := os.UserHomeDir()
			if err != nil {
				a.hookErr = fmt.Errorf("resolve home directory for agy native-write hook: %w", err)
				return
			}
			home = resolved
		}
		if err := ensureNativeWriteHook(home); err != nil {
			a.hookErr = fmt.Errorf("provision gimbal agy native-write hook: %w", err)
		}
	})
	return a.hookErr
}

func verifyAgyHookSupport(config runnerConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	args := append(append([]string(nil), config.baseArgs...), "--version")
	cmd := exec.CommandContext(ctx, config.binary, args...)
	cmd.Env = config.env
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("verify agy version before provisioning the gimbal-no-native-write PreToolUse hook: run %q --version: %w", config.binary, err)
	}
	version := strings.TrimSpace(string(out))
	ok, parseErr := agyVersionAtLeast(version, minSupportedAgyVersion)
	if parseErr != nil {
		return fmt.Errorf("agy --version reported %q, which could not be parsed as a dotted version to confirm PreToolUse hooks.json support (verified from agy %s onward; see harness/agy/native_write_hook.go)", version, minSupportedAgyVersion)
	}
	if !ok {
		return fmt.Errorf("agy %s is older than %s, the minimum version verified to honor the PreToolUse hooks.json mechanism the gimbal-no-native-write hook depends on; upgrade agy", version, minSupportedAgyVersion)
	}
	return nil
}

func writeSchema(raw json.RawMessage) (string, error) {
	file, err := os.CreateTemp("", "gimbal-agy-schema-*.json")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(name)
		return "", err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(name)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(name)
		return "", closeErr
	}
	return name, nil
}

func validateStructured(validator *harness.ResultValidator, value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode agy structured output: %w", err)
	}
	return validator.Validate(raw)
}

// compareEchoedSchema checks that agy ran the turn against the schema it was
// given, which it echoes back in the result.
func compareEchoedSchema(requested json.RawMessage, echoed any) error {
	if echoed == nil {
		return errors.New("agy result omitted json_schema")
	}
	var expected any
	if err := json.Unmarshal(requested, &expected); err != nil {
		return fmt.Errorf("decode requested output schema: %w", err)
	}
	expectedRaw, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("normalize requested output schema: %w", err)
	}
	echoedRaw, err := json.Marshal(echoed)
	if err != nil {
		return fmt.Errorf("normalize agy output schema: %w", err)
	}
	if !bytes.Equal(expectedRaw, echoedRaw) {
		return errors.New("agy result echoed a different JSON schema")
	}
	return nil
}

func envelopeConversationID(envelope schema.Envelope) string {
	if envelope.ConversationID != nil {
		return *envelope.ConversationID
	}
	if envelope.Init != nil && envelope.Init.ConversationID != nil {
		return *envelope.Init.ConversationID
	}
	if envelope.Result != nil && envelope.Result.ConversationID != nil {
		return *envelope.Result.ConversationID
	}
	return ""
}

func joinParts(parts []harness.ContentPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		texts = append(texts, part.Text)
	}
	return strings.Join(texts, "\n\n")
}

func modelIncludesEffort(model string) bool {
	for _, suffix := range []string{"-low", "-medium", "-high"} {
		if strings.HasSuffix(model, suffix) {
			return true
		}
	}
	return false
}

// artifactPathErrorMarker is agy's own error text when the model's file
// write goes through agy's native artifact tool, which only accepts paths in
// its private per-conversation directory. The PreToolUse hook blocks this
// before agy reaches it; this detector is the fallback if the hook is ever
// bypassed.
const artifactPathErrorMarker = "is not a valid artifact path"

func isArtifactPathError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, artifactPathErrorMarker) || strings.Contains(message, nativeWriteHookMarker)
}

// artifactMetadataPromptPreamble is prepended to the original turn prompt to
// steer the model away from triggering the ArtifactMetadata bug at all. It
// is not a substitute for the hook or the repair; it just makes both rarer.
const artifactMetadataPromptPreamble = "When using write_to_file, replace_file_content, or multi_replace_file_content to create or edit files in this workspace, never include an ArtifactMetadata argument. ArtifactMetadata is only for your own internal session-tracking documents inside your private per-conversation directory; attaching it to an ordinary workspace file causes the write to fail.\n\n"

// artifactWriteRepairPrompt asks the model to redo the original task in a
// fresh conversation, omitting ArtifactMetadata. It deliberately does not
// push the model onto a different tool: retrying the same native write tool
// without ArtifactMetadata is the verified fix.
func artifactWriteRepairPrompt(originalPrompt, message string) string {
	return fmt.Sprintf(
		"%s\n\n---\nRetry note: a previous attempt at this exact task failed because a file-write tool call (write_to_file, replace_file_content, or multi_replace_file_content) included an ArtifactMetadata argument while targeting a path outside your private per-conversation artifact directory: %s. ArtifactMetadata is only valid for files inside that private directory. If you call one of those tools again for a file in this workspace, omit the ArtifactMetadata argument entirely — do not switch to a different tool for it. Complete the original task above now.",
		originalPrompt, message,
	)
}

var _ harness.HarnessAdapter = (*Adapter)(nil)
