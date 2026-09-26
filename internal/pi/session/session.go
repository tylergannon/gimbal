// Package session is the headless Pinative agent session: it assembles the
// ported agent loop, tools, history, settings, resources and compaction into
// one coding session. It is a semantic port of the agent-session, sdk,
// agent-session-runtime and agent-session-services modules at upstream pin
// d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31.
//
// TUI, HTML export, RPC and JavaScript extension execution are out of scope.
// Every session is bound to an explicit working directory, agent directory and
// storage root; nothing here mutates process-global state.
package session

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/resources"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// ScopedModel is one model available to model cycling.
type ScopedModel struct {
	Model         *model.Model
	ThinkingLevel model.ThinkingLevel
}

// Config assembles a Session from already-created services. The caller owns
// constructing the agent, history manager and resource loader; Session
// subscribes to the agent and uses the rest for persistence, settings and
// summarization.
type Config struct {
	Agent          *agent.Agent
	SessionManager *history.SessionManager
	Settings       *config.SettingsManager
	Cwd            string

	ResourceLoader *resources.DefaultResourceLoader

	// StreamFn, APIKey, Headers and Env are the summarization provider call
	// used by compaction. StreamFn is required for compaction; a nil value
	// makes automatic compaction fail loudly rather than silently skip.
	StreamFn model.StreamFunction
	APIKey   string
	Headers  model.ProviderHeaders
	Env      model.ProviderEnv

	ScopedModels []ScopedModel

	// InitialActiveToolNames is the active built-in loadout. Nil falls back to
	// the base tool override keys, then DefaultActiveToolNames.
	InitialActiveToolNames []string
	AllowedToolNames       []string
	ExcludedToolNames      []string

	// BaseTools overrides the built-in tool set entirely (used by tests and
	// custom runtimes). Names map to AgentTool implementations.
	BaseTools map[string]model.AgentTool
	// CustomTools are additional definition-first tools.
	CustomTools []model.ToolDefinition
}

// Session is one headless coding session.
type Session struct {
	agent          *agent.Agent
	sessionManager *history.SessionManager
	settings       *config.SettingsManager
	cwd            string
	resourceLoader *resources.DefaultResourceLoader

	streamFn model.StreamFunction
	apiKey   string
	headers  model.ProviderHeaders
	env      model.ProviderEnv

	mu sync.Mutex

	listeners []Listener

	runActive      bool
	abortRequested bool
	idleCh         chan struct{}
	closed         bool
	disposed       bool

	lastAssistant         *model.AssistantMessage
	lastAssistantEntryID  string
	lastToolResults       []model.ToolResultMessage
	lastToolResultEntryID []string
	overflowAttempted     bool
	retryAttempt          int
	retryCancel           context.CancelFunc
	autoCompactionCancel  context.CancelFunc
	manualCompactionCx    context.CancelFunc
	manualCompaction      int
	basePromptOptions     resources.BuildSystemPromptOptions

	steeringMessages []string
	followUpMessages []string

	toolRegistry        map[string]model.AgentTool
	toolDefinitions     map[string]model.ToolDefinition
	baseToolDefinitions map[string]model.ToolDefinition
	customTools         []model.ToolDefinition
	baseToolsOverride   map[string]model.AgentTool
	allowedToolNames    map[string]bool
	excludedToolNames   map[string]bool
	initialActiveNames  []string

	scopedModels []ScopedModel

	template Config
}

// New creates a Session from a Config. It subscribes to the agent, builds the
// tool registry and seeds the transcript's system prompt.
func New(cfg Config) (*Session, error) {
	if cfg.Agent == nil {
		return nil, errors.New("session: agent is required")
	}
	if cfg.SessionManager == nil {
		return nil, errors.New("session: session manager is required")
	}
	if cfg.Settings == nil {
		return nil, errors.New("session: settings manager is required")
	}
	if cfg.Cwd == "" {
		cfg.Cwd = cfg.SessionManager.GetCwd()
	}

	s := &Session{
		agent:               cfg.Agent,
		sessionManager:      cfg.SessionManager,
		settings:            cfg.Settings,
		cwd:                 cfg.Cwd,
		resourceLoader:      cfg.ResourceLoader,
		streamFn:            cfg.StreamFn,
		apiKey:              cfg.APIKey,
		headers:             cfg.Headers,
		env:                 cfg.Env,
		scopedModels:        append([]ScopedModel(nil), cfg.ScopedModels...),
		initialActiveNames:  append([]string(nil), cfg.InitialActiveToolNames...),
		baseToolsOverride:   cfg.BaseTools,
		customTools:         append([]model.ToolDefinition(nil), cfg.CustomTools...),
		allowedToolNames:    toNameSet(cfg.AllowedToolNames),
		excludedToolNames:   toNameSet(cfg.ExcludedToolNames),
		toolRegistry:        map[string]model.AgentTool{},
		toolDefinitions:     map[string]model.ToolDefinition{},
		baseToolDefinitions: map[string]model.ToolDefinition{},
	}
	template := cfg
	template.Agent = nil
	template.SessionManager = nil
	s.template = template
	s.buildToolDefinitions()
	s.agent.Subscribe(s.handleAgentEvent)
	s.refreshContext()
	s.restoreToolsFromTranscript()
	s.rebuildSystemPrompt()
	s.ensureSystemMessage()
	return s, nil
}

func toNameSet(names []string) map[string]bool {
	if names == nil {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func (s *Session) isToolAllowed(name string) bool {
	if s.allowedToolNames != nil && !s.allowedToolNames[name] {
		return false
	}
	if s.excludedToolNames != nil && s.excludedToolNames[name] {
		return false
	}
	return true
}

func (s *Session) buildToolDefinitions() {
	s.baseToolDefinitions = map[string]model.ToolDefinition{}
	if s.baseToolsOverride != nil {
		for name, tool := range s.baseToolsOverride {
			if !s.isToolAllowed(name) {
				continue
			}
			s.baseToolDefinitions[name] = ToolDefinitionFromAgentTool(tool)
		}
	} else {
		for name, definition := range CreateAllToolDefinitions(s.cwd, nil) {
			if !s.isToolAllowed(name) {
				continue
			}
			s.baseToolDefinitions[name] = definition
		}
	}
	definitionRegistry := map[string]model.ToolDefinition{}
	maps.Copy(definitionRegistry, s.baseToolDefinitions)
	for _, definition := range s.customTools {
		if !s.isToolAllowed(definition.Name) {
			continue
		}
		definitionRegistry[definition.Name] = definition
	}
	s.toolDefinitions = definitionRegistry

	registry := map[string]model.AgentTool{}
	for name, definition := range definitionRegistry {
		registry[name] = WrapToolDefinition(definition)
	}
	s.toolRegistry = registry

	defaultActive := s.initialActiveNames
	if len(defaultActive) == 0 {
		if s.baseToolsOverride != nil {
			defaultActive = make([]string, 0, len(s.baseToolsOverride))
			for name := range s.baseToolsOverride {
				defaultActive = append(defaultActive, name)
			}
		} else {
			defaultActive = DefaultActiveToolNames
		}
	}
	s.setActiveToolsByName(defaultActive)
}

// Agent returns the underlying agent.
func (s *Session) Agent() *agent.Agent { return s.agent }

// SessionManager returns the history manager.
func (s *Session) SessionManager() *history.SessionManager { return s.sessionManager }

// Cwd returns the session working directory.
func (s *Session) Cwd() string { return s.cwd }

// SessionID returns the persisted session id.
func (s *Session) SessionID() string { return s.sessionManager.GetSessionID() }

// SessionFile returns the session file path, or "" for in-memory sessions.
func (s *Session) SessionFile() string { return s.sessionManager.GetSessionFile() }

// Model returns the active model.
func (s *Session) Model() *model.Model {
	state := s.agent.State()
	return state.Model
}

// ThinkingLevel returns the active thinking level.
func (s *Session) ThinkingLevel() model.ThinkingLevel {
	return s.agent.State().ThinkingLevel
}

// Messages returns the current transcript. The slice shares backing storage
// with agent state and should be treated read-only.
func (s *Session) Messages() []model.AgentMessage { return s.agent.State().Messages }

// IsStreaming reports whether a session run is active.
func (s *Session) IsStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runActive
}

// IsIdle reports whether the session has no active run or compaction.
func (s *Session) IsIdle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isIdleLocked()
}

func (s *Session) isIdleLocked() bool { return !s.runActive && s.manualCompaction == 0 }

// Subscribe registers an event listener and returns an unsubscribe function.
func (s *Session) Subscribe(listener Listener) func() {
	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	index := len(s.listeners) - 1
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if index < len(s.listeners) {
			s.listeners[index] = nil
		}
	}
}

func (s *Session) emit(event Event) error {
	s.mu.Lock()
	listeners := append([]Listener(nil), s.listeners...)
	s.mu.Unlock()
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		if err := listener(event); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) emitOrPanic(event Event) {
	if err := s.emit(event); err != nil {
		panic(err)
	}
}

// SystemPrompt renders the current system prompt.
func (s *Session) SystemPrompt() string {
	s.mu.Lock()
	options := s.basePromptOptions
	s.mu.Unlock()
	prompt, err := resources.BuildSystemPrompt(options)
	if err != nil {
		return ""
	}
	return prompt
}

func (s *Session) rebuildSystemPrompt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuildSystemPromptLocked()
}

func (s *Session) rebuildSystemPromptLocked() {
	selected := s.activeToolNamesLocked()
	snippets := map[string]string{}
	guidelines := map[string][]string{}
	for name, definition := range s.toolDefinitions {
		if snippet := normalizePromptSnippet(definition.PromptSnippet); snippet != "" {
			snippets[name] = snippet
		}
		if normalized := normalizePromptGuidelines(definition.PromptGuidelines); len(normalized) > 0 {
			guidelines[name] = normalized
		}
	}
	var customPrompt string
	var appendPrompt []string
	var skills []resources.Skill
	var contextFiles []resources.ContextFile
	if s.resourceLoader != nil {
		if prompt := s.resourceLoader.GetSystemPrompt(); prompt != nil {
			customPrompt = *prompt
		}
		appendPrompt = s.resourceLoader.GetAppendSystemPrompt()
		skills = s.resourceLoader.GetSkills().Skills
		contextFiles = s.resourceLoader.GetAgentsFiles()
	}
	s.basePromptOptions = resources.NormalizeBuildSystemPromptOptions(resources.BuildSystemPromptOptions{
		Cwd:                s.cwd,
		Skills:             skills,
		ContextFiles:       contextFiles,
		CustomPrompt:       customPrompt,
		AppendSystemPrompt: strings.Join(appendPrompt, "\n\n"),
		SelectedTools:      selected,
		ToolSnippets:       snippets,
		ToolGuidelines:     guidelines,
	})
}

func normalizePromptSnippet(text string) string {
	if text == "" {
		return ""
	}
	oneLine := strings.Join(strings.Fields(text), " ")
	return oneLine
}

func normalizePromptGuidelines(guidelines []string) []string {
	if len(guidelines) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(guidelines))
	for _, guideline := range guidelines {
		trimmed := strings.TrimSpace(guideline)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

// GetActiveToolNames returns the active tool names in order.
func (s *Session) GetActiveToolNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeToolNamesLocked()
}

func (s *Session) activeToolNamesLocked() []string {
	tools := s.agent.State().Tools
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// GetAllTools returns every registered tool definition.
func (s *Session) GetAllTools() []model.ToolDefinition {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.ToolDefinition, 0, len(s.toolDefinitions))
	for _, definition := range s.toolDefinitions {
		out = append(out, definition)
	}
	return out
}

// GetToolDefinition returns one registered tool definition.
func (s *Session) GetToolDefinition(name string) (model.ToolDefinition, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, ok := s.toolDefinitions[name]
	return definition, ok
}

// SetActiveToolsByName enables the named registered tools. Unknown names are
// ignored. Changes take effect on the next turn.
func (s *Session) SetActiveToolsByName(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setActiveToolsByNameLocked(names)
}

func (s *Session) setActiveToolsByName(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setActiveToolsByNameLocked(names)
}

func (s *Session) setActiveToolsByNameLocked(names []string) {
	tools := make([]model.AgentTool, 0, len(names))
	for _, name := range names {
		if tool, ok := s.toolRegistry[name]; ok {
			tools = append(tools, tool)
		}
	}
	s.agent.SetTools(tools)
	s.rebuildSystemPromptLocked()
}

func (s *Session) restoreToolsFromTranscript() {
	context := s.sessionManager.BuildSessionContext()
	current, ok := model.GetCurrentSystemMessage(context.Messages)
	if !ok {
		return
	}
	var names []string
	for _, tool := range current.ToolsAdded {
		if _, registered := s.toolRegistry[tool.Name]; registered {
			names = append(names, tool.Name)
		}
	}
	if len(names) == 0 && len(s.initialActiveNames) > 0 {
		return
	}
	s.setActiveToolsByName(names)
}

// ensureSystemMessage makes sure the transcript begins with a system message
// carrying the current prompt and tool loadout.
func (s *Session) ensureSystemMessage() {
	state := s.agent.State()
	if _, ok := model.GetInitialSystemMessage(state.Messages); ok {
		return
	}
	declarations := toolDeclarations(s.agent.State().Tools)
	initial, ok := model.CreateInitialSystemMessage(s.SystemPrompt(), declarations)
	if !ok {
		return
	}
	_, _ = s.sessionManager.AppendMessage(initial)
	s.refreshContext()
}

func toolDeclarations(tools []model.AgentTool) []model.Tool {
	out := make([]model.Tool, len(tools))
	for i, tool := range tools {
		out[i] = model.ToToolDeclaration(tool.AsTool())
	}
	return out
}

// refreshContext replaces agent state with the session projection.
func (s *Session) refreshContext() {
	projection := s.sessionManager.BuildSessionProjection()
	s.agent.SetMessages(projection.Messages)
}

// RefreshContext is the exported form of refreshContext.
func (s *Session) RefreshContext() { s.refreshContext() }

// SetModel sets the active model and persists the change.
func (s *Session) SetModel(m *model.Model) {
	s.agent.SetModel(m)
	_, _ = s.sessionManager.AppendModelChange(string(m.Provider), m.ID)
}

// SetThinkingLevel sets the active thinking level and persists the change.
func (s *Session) SetThinkingLevel(level model.ThinkingLevel) {
	s.agent.SetThinkingLevel(level)
	_, _ = s.sessionManager.AppendThinkingLevelChange(string(level))
}

// ScopedModels returns the models available for cycling.
func (s *Session) ScopedModels() []ScopedModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ScopedModel(nil), s.scopedModels...)
}

// PendingMessageCount is the number of undelivered steering and follow-up
// messages.
func (s *Session) PendingMessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.steeringMessages) + len(s.followUpMessages)
}

// GetSteeringMessages returns the pending steering message texts.
func (s *Session) GetSteeringMessages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.steeringMessages...)
}

// GetFollowUpMessages returns the pending follow-up message texts.
func (s *Session) GetFollowUpMessages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.followUpMessages...)
}

// ClearQueue clears both queues and returns what was cleared.
func (s *Session) ClearQueue() (steering, followUp []string) {
	s.mu.Lock()
	steering = append([]string(nil), s.steeringMessages...)
	followUp = append([]string(nil), s.followUpMessages...)
	s.steeringMessages = nil
	s.followUpMessages = nil
	s.mu.Unlock()
	s.agent.ClearAllQueues()
	s.emitQueueUpdate()
	return steering, followUp
}

func (s *Session) emitQueueUpdate() {
	s.mu.Lock()
	steering := append([]string(nil), s.steeringMessages...)
	followUp := append([]string(nil), s.followUpMessages...)
	s.mu.Unlock()
	s.emitOrPanic(Event{Type: EventQueueUpdate, Steering: steering, FollowUp: followUp})
}

// PromptOptions configures Prompt.
type PromptOptions struct {
	// Images are image attachments for the user message.
	Images []model.ImageContent
	// StreamingBehavior queues the prompt when a run is active. Required while
	// streaming.
	StreamingBehavior string
}

// Prompt sends a user prompt and blocks until the session settles.
func (s *Session) Prompt(ctx context.Context, text string, options *PromptOptions) error {
	if options == nil {
		options = &PromptOptions{}
	}
	if options.StreamingBehavior != "" && s.IsStreaming() {
		if options.StreamingBehavior == "followUp" {
			return s.queueFollowUp(text, options.Images)
		}
		return s.queueSteer(text, options.Images)
	}

	if !s.beginRun() {
		return errors.New("Agent is already processing. Specify streamingBehavior ('steer' or 'followUp') to queue the message.") //nolint:staticcheck // pi's exact error message
	}
	runErr := s.runPrompt(ctx, text, options)
	s.endRun()
	return runErr
}

// Continue continues from the current transcript.
func (s *Session) Continue(ctx context.Context) error {
	if !s.beginRun() {
		return errors.New("Agent is already processing. Wait for completion before continuing.") //nolint:staticcheck // pi's exact error message
	}
	runErr := s.runContinue(ctx)
	s.endRun()
	return runErr
}

func (s *Session) beginRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runActive || s.closed {
		return false
	}
	s.runActive = true
	s.abortRequested = false
	if s.idleCh == nil {
		s.idleCh = make(chan struct{})
	}
	return true
}

func (s *Session) endRun() {
	s.mu.Lock()
	s.runActive = false
	s.abortRequested = false
	s.notifyIdleLocked()
	s.mu.Unlock()
	_ = s.emit(Event{Type: EventAgentSettled})
}

func (s *Session) notifyIdleLocked() {
	if s.isIdleLocked() && s.idleCh != nil {
		close(s.idleCh)
		s.idleCh = nil
	}
}

// WaitForIdle blocks until no run or compaction is active.
func (s *Session) WaitForIdle(ctx context.Context) error {
	s.mu.Lock()
	if s.isIdleLocked() {
		s.mu.Unlock()
		return nil
	}
	if s.idleCh == nil {
		s.idleCh = make(chan struct{})
	}
	ch := s.idleCh
	s.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Abort cancels the active run and any retry or compaction.
func (s *Session) Abort() {
	s.mu.Lock()
	if s.runActive {
		s.abortRequested = true
	}
	if s.retryCancel != nil {
		s.retryCancel()
	}
	if s.autoCompactionCancel != nil {
		s.autoCompactionCancel()
	}
	if s.manualCompactionCx != nil {
		s.manualCompactionCx()
	}
	s.mu.Unlock()
	s.agent.Abort()
}

// Dispose releases listeners and aborts any active work. It is idempotent.
func (s *Session) Dispose() {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	s.disposed = true
	s.closed = true
	s.listeners = nil
	retryCancel := s.retryCancel
	compactionCancel := s.autoCompactionCancel
	manualCompactionCancel := s.manualCompactionCx
	s.mu.Unlock()

	if retryCancel != nil {
		retryCancel()
	}
	if compactionCancel != nil {
		compactionCancel()
	}
	if manualCompactionCancel != nil {
		manualCompactionCancel()
	}
	s.agent.Abort()
}

// Close is an alias for Dispose for callers that release resources.
func (s *Session) Close() { s.Dispose() }

func (s *Session) queueSteer(text string, images []model.ImageContent) error {
	s.mu.Lock()
	s.steeringMessages = append(s.steeringMessages, text)
	s.mu.Unlock()
	s.emitQueueUpdate()
	content := model.ContentList{model.TextContent{Text: text}}
	for _, image := range images {
		content = append(content, image)
	}
	s.agent.Steer(model.UserMessage{Content: content, Timestamp: time.Now().UnixMilli()})
	return nil
}

func (s *Session) queueFollowUp(text string, images []model.ImageContent) error {
	s.mu.Lock()
	s.followUpMessages = append(s.followUpMessages, text)
	s.mu.Unlock()
	s.emitQueueUpdate()
	content := model.ContentList{model.TextContent{Text: text}}
	for _, image := range images {
		content = append(content, image)
	}
	s.agent.FollowUp(model.UserMessage{Content: content, Timestamp: time.Now().UnixMilli()})
	return nil
}

// Steer queues a steering message while a run is active.
func (s *Session) Steer(text string, images ...model.ImageContent) error {
	return s.queueSteer(text, images)
}

// FollowUp queues a follow-up message while a run is active.
func (s *Session) FollowUp(text string, images ...model.ImageContent) error {
	return s.queueFollowUp(text, images)
}

func (s *Session) buildUserMessage(text string, images []model.ImageContent) model.AgentMessage {
	content := model.ContentList{model.TextContent{Text: text}}
	for _, image := range images {
		content = append(content, image)
	}
	return model.UserMessage{Content: content, Timestamp: time.Now().UnixMilli()}
}

func (s *Session) runPrompt(ctx context.Context, text string, options *PromptOptions) error {
	s.ensureSystemMessage()
	message := s.buildUserMessage(text, options.Images)
	var runErr error
	if err := s.agent.PromptMessages(ctx, []model.AgentMessage{message}); err != nil {
		runErr = err
	}
	if err := s.runPostRunLoop(ctx); err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

func (s *Session) runContinue(ctx context.Context) error {
	var runErr error
	if err := s.agent.Continue(ctx); err != nil {
		runErr = err
	}
	if err := s.runPostRunLoop(ctx); err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

func (s *Session) runPostRunLoop(ctx context.Context) error {
	for {
		cont, err := s.handlePostAgentRun(ctx)
		if err != nil {
			return err
		}
		if s.abortRequestedNow() || !cont {
			return nil
		}
		if err := s.agent.Continue(ctx); err != nil {
			return err
		}
	}
}

func (s *Session) abortRequestedNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.abortRequested
}

func (s *Session) handlePostAgentRun(ctx context.Context) (bool, error) {
	s.mu.Lock()
	message := s.lastAssistant
	entryID := s.lastAssistantEntryID
	s.lastAssistant = nil
	s.lastAssistantEntryID = ""
	toolResults := s.lastToolResults
	toolResultIDs := s.lastToolResultEntryID
	s.lastToolResults = nil
	s.lastToolResultEntryID = nil
	aborted := s.abortRequested
	s.mu.Unlock()

	if aborted {
		s.finishCancelledRetry()
		return false, nil
	}
	if message == nil {
		return s.agent.HasQueuedMessages(), nil
	}
	if s.isRetryableError(message) && s.prepareRetry(ctx, message, entryID) {
		if s.abortRequestedNow() {
			s.finishCancelledRetry()
			return false, nil
		}
		return true, nil
	}
	if aborted {
		s.finishCancelledRetry()
		return false, nil
	}
	if message.StopReason == model.StopError && s.retryAttempt > 0 {
		attempt := s.retryAttempt
		s.retryAttempt = 0
		s.emitOrPanic(Event{Type: EventAutoRetryEnd, Success: false, Attempt: attempt, FinalError: message.ErrorMessage})
	}
	cont, err := s.checkCompaction(ctx, message, entryID, toolResults, toolResultIDs, true)
	if err != nil {
		return false, err
	}
	if cont {
		return !s.abortRequestedNow(), nil
	}
	return !s.abortRequestedNow() && s.agent.HasQueuedMessages(), nil
}

// handleAgentEvent is the agent subscription that persists messages and
// forwards events.
func (s *Session) handleAgentEvent(ctx context.Context, event model.AgentEvent) error {
	if event.Type == model.EvMessageStart && event.Message != nil && event.Message.MessageRole() == model.RoleUser {
		s.clearDeliveredQueueEntry(event.Message)
	}

	forwarded := Event{Type: EventType(event.Type), Agent: event}
	if event.Type == model.EvAgentEnd {
		forwarded.Messages = event.Messages
		forwarded.WillRetry = s.willRetryAfterAgentEnd(event.Messages)
	}
	if err := s.emit(forwarded); err != nil {
		return err
	}

	if event.Type == model.EvMessageEnd && event.Message != nil {
		s.persistMessage(event.Message)
	}
	if event.Type == model.EvTurnEnd {
		s.mu.Lock()
		s.lastToolResults = append([]model.ToolResultMessage(nil), event.ToolResults...)
		s.mu.Unlock()
	}
	return nil
}

func (s *Session) clearDeliveredQueueEntry(message model.AgentMessage) {
	text := messageText(message)
	if text == "" {
		return
	}
	s.mu.Lock()
	removed := false
	if index := indexOf(s.steeringMessages, text); index >= 0 {
		s.steeringMessages = append(s.steeringMessages[:index], s.steeringMessages[index+1:]...)
		removed = true
	} else if index := indexOf(s.followUpMessages, text); index >= 0 {
		s.followUpMessages = append(s.followUpMessages[:index], s.followUpMessages[index+1:]...)
		removed = true
	}
	s.mu.Unlock()
	if removed {
		s.emitQueueUpdate()
	}
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func messageText(message model.AgentMessage) string {
	switch value := message.(type) {
	case model.UserMessage:
		return model.ContentText(value.Content)
	case *model.UserMessage:
		if value != nil {
			return model.ContentText(value.Content)
		}
	case model.CustomMessage:
		return model.ContentText(value.Content)
	case *model.CustomMessage:
		if value != nil {
			return model.ContentText(value.Content)
		}
	case model.SystemMessage:
		return model.ContentText(value.Content)
	case *model.SystemMessage:
		if value != nil {
			return model.ContentText(value.Content)
		}
	case model.AssistantMessage:
		return model.ContentText(value.Content)
	case *model.AssistantMessage:
		if value != nil {
			return model.ContentText(value.Content)
		}
	case model.ToolResultMessage:
		return model.ContentText(value.Content)
	case *model.ToolResultMessage:
		if value != nil {
			return model.ContentText(value.Content)
		}
	}
	return ""
}

func (s *Session) persistMessage(message model.AgentMessage) {
	var entryID string
	switch message.MessageRole() {
	case model.RoleCustom:
		if custom, ok := message.(model.CustomMessage); ok {
			entryID, _ = s.sessionManager.AppendCustomMessageEntry(custom.CustomType, custom.Content, custom.Display, custom.Details)
		}
	default:
		entryID, _ = s.sessionManager.AppendMessage(message)
	}
	if message.MessageRole() == model.RoleAssistant {
		if assistant, ok := assistantOf(message); ok {
			s.mu.Lock()
			s.lastAssistant = assistant
			s.lastAssistantEntryID = entryID
			s.lastToolResultEntryID = nil
			if assistant.StopReason != model.StopError && assistant.StopReason != model.StopLength {
				s.overflowAttempted = false
			}
			if assistant.StopReason != model.StopError && s.retryAttempt > 0 {
				attempt := s.retryAttempt
				s.retryAttempt = 0
				s.mu.Unlock()
				s.emitOrPanic(Event{Type: EventAutoRetryEnd, Success: true, Attempt: attempt})
				return
			}
			s.mu.Unlock()
		}
	}
	if message.MessageRole() == model.RoleToolResult && entryID != "" {
		s.mu.Lock()
		s.lastToolResultEntryID = append(s.lastToolResultEntryID, entryID)
		s.mu.Unlock()
	}
}

func assistantOf(message model.AgentMessage) (*model.AssistantMessage, bool) {
	switch value := message.(type) {
	case model.AssistantMessage:
		clone := value
		return &clone, true
	case *model.AssistantMessage:
		return value, true
	default:
		return nil, false
	}
}

func (s *Session) willRetryAfterAgentEnd(messages []model.AgentMessage) bool {
	s.mu.Lock()
	aborted := s.abortRequested
	s.mu.Unlock()
	if aborted {
		return false
	}
	settings := s.settings.GetRetrySettings()
	if !settings.Enabled || s.retryAttemptValue() >= settings.MaxRetries {
		return false
	}
	for _, message := range slices.Backward(messages) {
		if assistant, ok := assistantOf(message); ok {
			return s.isRetryableError(assistant)
		}
	}
	return false
}

func (s *Session) retryAttemptValue() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryAttempt
}

func (s *Session) isRetryableError(message *model.AssistantMessage) bool {
	if wire.IsContextOverflow(message, s.modelContextWindow()) {
		return false
	}
	return wire.IsRetryableAssistantError(message)
}

func (s *Session) modelContextWindow() int {
	if model := s.Model(); model != nil {
		return model.ContextWindow
	}
	return 0
}

func (s *Session) prepareRetry(ctx context.Context, message *model.AssistantMessage, entryID string) bool {
	settings := s.settings.GetRetrySettings()
	if !settings.Enabled {
		return false
	}
	s.mu.Lock()
	s.retryAttempt++
	attempt := s.retryAttempt
	if attempt > settings.MaxRetries {
		s.retryAttempt--
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	policy := retryPolicy(settings)
	delay := wire.RetryDelayMs(policy, attempt)
	s.emitOrPanic(Event{
		Type: EventAutoRetryStart, Attempt: attempt, MaxAttempts: settings.MaxRetries,
		DelayMs: delay, ErrorMessage: fallback(message.ErrorMessage, "Unknown error"),
	})

	s.omitRecoveryAttempt(message, entryID, nil, nil)

	retryCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.retryCancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.retryCancel = nil
		s.mu.Unlock()
		cancel()
	}()

	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-retryCtx.Done():
		s.finishCancelledRetry()
		return false
	case <-timer.C:
	}
	return true
}

func (s *Session) finishCancelledRetry() {
	s.mu.Lock()
	attempt := s.retryAttempt
	if attempt == 0 {
		s.mu.Unlock()
		return
	}
	s.retryAttempt = 0
	s.mu.Unlock()
	s.emitOrPanic(Event{Type: EventAutoRetryEnd, Success: false, Attempt: attempt, FinalError: "Retry cancelled"})
}

func retryPolicy(settings config.RetrySettingsValue) wire.RetryPolicy {
	maxAgentDelay := settings.MaxAgentDelayMs
	return wire.RetryPolicy{
		Enabled:         settings.Enabled,
		MaxRetries:      settings.MaxRetries,
		BaseDelayMs:     settings.BaseDelayMs,
		MaxAgentDelayMs: &maxAgentDelay,
	}
}

func (s *Session) omitRecoveryAttempt(message *model.AssistantMessage, entryID string, toolResults []model.ToolResultMessage, toolResultIDs []string) {
	ids := make([]string, 0, 1+len(toolResults))
	if entryID != "" {
		ids = append(ids, entryID)
	}
	for _, id := range toolResultIDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		editID, err := s.sessionManager.AppendContextEdit(id, nil)
		if err != nil {
			continue
		}
		if entry := s.sessionManager.GetEntry(editID); entry != nil {
			s.emitOrPanic(Event{Type: EventEntryAppended, Entry: entry})
		}
	}
	s.refreshContext()
}

func fallback(value, fallbackValue string) string {
	if value != "" {
		return value
	}
	return fallbackValue
}
