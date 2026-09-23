// Package conversation owns the web application's durable conversations and
// their live harness sessions.
package conversation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/observation"
)

const (
	StatusIdle    = "idle"
	StatusWorking = "working"
	StatusError   = "error"

	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusError     = "error"
)

// Message is one visible turn in a conversation transcript.
type Message struct {
	Role    string `json:"role"`
	Text    string `json:"text"`
	Created int64  `json:"created"`
}

// Run is one workflow the conversation agent successfully launched.
type Run struct {
	ID       string `json:"id"`
	Workflow string `json:"workflow"`
	Status   string `json:"status"`
	Error    string `json:"error"`
}

// Conversation is the durable metadata and visible transcript for one chat.
type Conversation struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Branch        string    `json:"branch"`
	Worktree      string    `json:"worktree"`
	NativeSession string    `json:"native_session"`
	Live          bool      `json:"live"`
	Status        string    `json:"status"`
	Error         string    `json:"error"`
	Created       int64     `json:"created"`
	Updated       int64     `json:"updated"`
	Messages      []Message `json:"messages"`
	Runs          []Run     `json:"runs"`
}

// NewConversation is the provider and optional presentation selected by the
// person opening a conversation.
type NewConversation struct {
	Title    string
	Provider string
	Model    string
}

// AdapterFactory resolves a provider name to its existing Gimble harness.
type AdapterFactory func(string) (gimble.HarnessAdapter, error)

type activeConversation struct {
	turnMu  sync.Mutex
	adapter gimble.HarnessAdapter
	session string
	resume  bool
}

// persistentSessionAdapter is the extra lifecycle used only by durable,
// human-facing conversations. Workflow sessions continue to use Close.
type persistentSessionAdapter interface {
	ResumeSession(context.Context, string, string, string, string) error
	DetachSession(context.Context, string) error
}

// Manager is owned by one host project. Durable state is under projectDir;
// adapters exist only for that runtime's life. Providers that support durable
// conversation sessions may persist their native identity with the transcript.
type Manager struct {
	ctx         context.Context
	projectDir  string
	factory     AdapterFactory
	cli         string
	instanceDir string
	project     string
	registry    *observation.Registry

	mu      sync.RWMutex
	items   map[string]*Conversation
	active  map[string]*activeConversation
	closing bool
	closed  chan struct{}
}

type managerKey struct{}

// WithManager exposes the runtime-owned manager to the page's Go handlers.
func WithManager(ctx context.Context, manager *Manager) context.Context {
	return context.WithValue(ctx, managerKey{}, manager)
}

// FromContext returns the manager owned by the runtime serving ctx.
func FromContext(ctx context.Context) *Manager {
	if ctx == nil {
		return nil
	}
	manager, _ := ctx.Value(managerKey{}).(*Manager)
	return manager
}

// New loads the saved conversation metadata. It does not touch Git or start a
// harness until the person creates or sends to a conversation.
func New(ctx context.Context, projectDir string, factory AdapterFactory, cli, instanceDir, project string, registry *observation.Registry) (*Manager, error) {
	if ctx == nil {
		return nil, errors.New("conversation: runtime context is nil")
	}
	if factory == nil {
		return nil, errors.New("conversation: adapter factory is nil")
	}
	dir := filepath.Join(projectDir, "conversations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("conversation: create store: %w", err)
	}
	manager := &Manager{
		ctx: ctx, projectDir: projectDir, factory: factory, cli: cli, instanceDir: instanceDir, project: project, registry: registry,
		items: make(map[string]*Conversation), active: make(map[string]*activeConversation), closed: make(chan struct{}),
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("conversation: read store: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("conversation: read %s: %w", entry.Name(), err)
		}
		var item Conversation
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("conversation: decode %s: %w", entry.Name(), err)
		}
		if item.ID == "" {
			return nil, fmt.Errorf("conversation: %s has no id", entry.Name())
		}
		if item.Messages == nil {
			item.Messages = []Message{}
		}
		if item.Runs == nil {
			item.Runs = []Run{}
		}
		// The conversation turn is no longer active after a restart. Run state
		// is read separately from the project's authoritative run record.
		if item.Status == StatusWorking {
			item.Status = StatusIdle
		}
		manager.refresh(&item)
		// A persisted native identity can be resumed lazily on the next send.
		// Loading the page itself still does not start or contact a harness.
		item.Live = item.NativeSession != ""
		copy := item
		manager.items[item.ID] = &copy
	}
	context.AfterFunc(ctx, func() { manager.Close() })
	return manager, nil
}

// Create makes a distinct branch and Git worktree before saving the
// conversation that names them.
func (m *Manager) Create(ctx context.Context, in NewConversation) (Conversation, error) {
	provider := strings.TrimSpace(in.Provider)
	model, _, err := providerModel(provider, strings.TrimSpace(in.Model))
	if err != nil {
		return Conversation{}, err
	}
	if _, err := m.factory(provider); err != nil {
		return Conversation{}, fmt.Errorf("conversation: provider %s: %w", provider, err)
	}
	repo, err := gitOutput(ctx, filepath.Dir(m.projectDir), "rev-parse", "--show-toplevel")
	if err != nil {
		return Conversation{}, fmt.Errorf("conversation: find repository: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return Conversation{}, fmt.Errorf("conversation: create id: %w", err)
	}
	branch := "gimble/conversation-" + id[:12]
	worktree := filepath.Join(m.projectDir, "conversation-worktrees", id)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return Conversation{}, fmt.Errorf("conversation: create worktree directory: %w", err)
	}
	command := exec.CommandContext(ctx, "git", "-C", repo, "worktree", "add", "-b", branch, worktree, "HEAD")
	if output, err := command.CombinedOutput(); err != nil {
		return Conversation{}, fmt.Errorf("conversation: create worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := time.Now().UnixMilli()
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = providerLabel(provider) + " conversation"
	}
	item := &Conversation{
		ID: id, Title: title, Provider: provider, Model: model,
		Branch: branch, Worktree: worktree, Live: true, Status: StatusIdle,
		Created: now, Updated: now, Messages: []Message{}, Runs: []Run{},
	}
	if err := m.save(item); err != nil {
		return Conversation{}, err
	}
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return Conversation{}, errors.New("conversation: runtime is shutting down")
	}
	m.items[id] = item
	m.active[id] = &activeConversation{}
	m.mu.Unlock()
	return clone(item), nil
}

// List returns newest-first durable snapshots.
func (m *Manager) List() []Conversation {
	m.mu.Lock()
	items := make([]Conversation, 0, len(m.items))
	for _, item := range m.items {
		m.refresh(item)
		items = append(items, clone(item))
	}
	m.mu.Unlock()
	slices.SortFunc(items, func(left, right Conversation) int {
		if left.Updated == right.Updated {
			return strings.Compare(right.ID, left.ID)
		}
		if left.Updated > right.Updated {
			return -1
		}
		return 1
	})
	return items
}

// Get returns one durable snapshot.
func (m *Manager) Get(id string) (Conversation, bool) {
	m.mu.Lock()
	item := m.items[id]
	if item == nil {
		m.mu.Unlock()
		return Conversation{}, false
	}
	m.refresh(item)
	snapshot := clone(item)
	m.mu.Unlock()
	return snapshot, true
}

// Associate records a run admitted by the ordinary CLI for this conversation.
// The association is saved before the submission response returns to the CLI.
func (m *Manager) Associate(id, runID, workflow, workdir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	if item == nil || item.Worktree != workdir {
		return errors.New("conversation: run must use the conversation's worktree")
	}
	item.Runs = append(item.Runs, Run{ID: runID, Workflow: workflow, Status: RunStatusRunning})
	item.Updated = time.Now().UnixMilli()
	return m.save(item)
}

// Worktree returns the execution directory permitted for an associated run.
func (m *Manager) Worktree(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item := m.items[id]
	if item == nil {
		return "", false
	}
	return item.Worktree, true
}

// RefreshRun reads the authoritative project run row after execution ends.
func (m *Manager) RefreshRun(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.items {
		for _, run := range item.Runs {
			if run.ID == id {
				m.refresh(item)
				return
			}
		}
	}
}

func (m *Manager) refresh(item *Conversation) {
	if m.registry == nil {
		return
	}
	changed := false
	for index := range item.Runs {
		run := &item.Runs[index]
		if run.Status != RunStatusRunning {
			continue
		}
		snapshot, err := m.registry.Snapshot(run.ID)
		if err != nil || snapshot.Run.Status == observation.StatusRunning {
			continue
		}
		run.Status, run.Error = RunStatusCompleted, ""
		if snapshot.Run.Status != observation.StatusCompleted {
			run.Status, run.Error = RunStatusError, snapshot.Run.Error
		}
		changed = true
	}
	if changed {
		item.Updated = time.Now().UnixMilli()
		_ = m.save(item)
	}
}

// Send records the user's message, runs one turn on the conversation's live
// provider session in its worktree, and records the final assistant response.
func (m *Manager) Send(id, text string) (Conversation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Conversation{}, errors.New("conversation: message is blank")
	}
	m.mu.Lock()
	item := m.items[id]
	active := m.active[id]
	if item == nil {
		m.mu.Unlock()
		return Conversation{}, fmt.Errorf("conversation: unknown conversation %q", id)
	}
	if active == nil && item.NativeSession != "" {
		active = &activeConversation{session: item.NativeSession, resume: true}
		m.active[id] = active
	}
	m.mu.Unlock()
	if active == nil {
		return clone(item), errors.New("conversation: this saved conversation belongs to an earlier server session; its history is readable, but its native provider session is no longer live")
	}

	active.turnMu.Lock()
	defer active.turnMu.Unlock()
	if err := m.ctx.Err(); err != nil {
		return Conversation{}, context.Cause(m.ctx)
	}

	now := time.Now().UnixMilli()
	m.mu.Lock()
	item = m.items[id]
	item.Messages = append(item.Messages, Message{Role: "user", Text: text, Created: now})
	item.Status, item.Error, item.Updated = StatusWorking, "", now
	if err := m.save(item); err != nil {
		m.mu.Unlock()
		return Conversation{}, err
	}
	provider, model, worktree := item.Provider, item.Model, item.Worktree
	m.mu.Unlock()

	if active.adapter == nil {
		adapter, err := m.factory(provider)
		if err != nil {
			return m.fail(item, err)
		}
		active.adapter = adapter
	}
	if active.resume {
		resumer, ok := active.adapter.(persistentSessionAdapter)
		if !ok {
			return m.fail(item, errors.New("conversation: this provider cannot resume a saved native session"))
		}
		_, effort, _ := providerModel(provider, model)
		if err := resumer.ResumeSession(m.ctx, active.session, model, effort, worktree); err != nil {
			return m.fail(item, fmt.Errorf("conversation: resume native session: %w", err))
		}
		active.resume = false
	}
	if active.session == "" {
		_, effort, _ := providerModel(provider, model)
		session, err := active.adapter.CreateSession(m.ctx, model, effort, worktree)
		if err != nil {
			return m.fail(item, err)
		}
		active.session = session
		if _, ok := active.adapter.(persistentSessionAdapter); ok {
			m.mu.Lock()
			item = m.items[id]
			item.NativeSession = session
			if err := m.save(item); err != nil {
				m.mu.Unlock()
				return m.fail(item, err)
			}
			m.mu.Unlock()
		}
	}
	prompt := fmt.Sprintf(conversationPrompt, m.cli, m.instanceDir, m.project, worktree, id) + text
	result, err := active.adapter.RunTurn(m.ctx, active.session, prompt, conversationReplySchema, func(gimble.AgentEvent) error { return nil })
	if err != nil {
		return m.fail(item, err)
	}
	var response conversationReply
	if err := json.Unmarshal(result.Output, &response); err != nil {
		return m.fail(item, fmt.Errorf("provider returned an invalid conversation response: %w", err))
	}
	response.Message = strings.TrimSpace(response.Message)
	if response.Message == "" {
		return m.fail(item, errors.New("provider returned an empty conversation response"))
	}
	m.mu.Lock()
	item = m.items[id]
	now = time.Now().UnixMilli()
	item.Messages = append(item.Messages, Message{Role: "assistant", Text: response.Message, Created: now})
	m.refresh(item)
	item.Status, item.Error, item.Updated = StatusIdle, "", now
	err = m.save(item)
	snapshot := clone(item)
	m.mu.Unlock()
	if err != nil {
		return Conversation{}, err
	}
	return snapshot, nil
}

func (m *Manager) fail(item *Conversation, cause error) (Conversation, error) {
	m.mu.Lock()
	item = m.items[item.ID]
	item.Status, item.Error, item.Updated = StatusError, cause.Error(), time.Now().UnixMilli()
	saveErr := m.save(item)
	snapshot := clone(item)
	m.mu.Unlock()
	if saveErr != nil {
		return snapshot, errors.Join(cause, saveErr)
	}
	return snapshot, cause
}

// Close releases each live native session. Worktrees and saved conversations
// deliberately remain: they are the durable association the UI promises.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		<-m.closed
		return
	}
	m.closing = true
	defer close(m.closed)
	active := make([]*activeConversation, 0, len(m.active))
	for _, conversation := range m.active {
		active = append(active, conversation)
	}
	m.mu.Unlock()
	for _, conversation := range active {
		conversation.turnMu.Lock()
		if conversation.adapter != nil && conversation.session != "" {
			if persistent, ok := conversation.adapter.(persistentSessionAdapter); ok {
				_ = persistent.DetachSession(context.WithoutCancel(m.ctx), conversation.session)
			} else {
				_ = conversation.adapter.Close(context.WithoutCancel(m.ctx), conversation.session)
			}
		}
		conversation.turnMu.Unlock()
	}
}

func (m *Manager) save(item *Conversation) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("conversation: encode %s: %w", item.ID, err)
	}
	dir := filepath.Join(m.projectDir, "conversations")
	temporary, err := os.CreateTemp(dir, item.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("conversation: save %s: %w", item.ID, err)
	}
	name := temporary.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("conversation: save %s: %w", item.ID, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("conversation: save %s: %w", item.ID, err)
	}
	if err := os.Rename(name, filepath.Join(dir, item.ID+".json")); err != nil {
		return fmt.Errorf("conversation: save %s: %w", item.ID, err)
	}
	return nil
}

func providerModel(provider, model string) (string, string, error) {
	switch provider {
	case "codex":
		if model == "" {
			model = "gpt-6-luna"
		}
		if !strings.HasPrefix(model, "gpt-") && !strings.HasPrefix(model, "o1") && !strings.HasPrefix(model, "o3") && !strings.HasPrefix(model, "o4") {
			return "", "", fmt.Errorf("conversation: model %q is not a Codex model", model)
		}
		return model, "low", nil
	case "claude":
		if model == "" {
			model = "claude-haiku-4-5-20251001"
		}
		if !strings.HasPrefix(model, "claude-") {
			return "", "", fmt.Errorf("conversation: model %q is not a Claude model", model)
		}
		return model, "", nil
	case "agy":
		if model == "" {
			model = "gemini-3.8-flash-low"
		}
		if !strings.HasPrefix(model, "gemini-") {
			return "", "", fmt.Errorf("conversation: model %q is not an agy/Gemini model", model)
		}
		effort := ""
		for _, candidate := range []string{"low", "medium", "high"} {
			if strings.HasSuffix(model, "-"+candidate) {
				effort = candidate
			}
		}
		return model, effort, nil
	default:
		return "", "", fmt.Errorf("conversation: unknown provider %q", provider)
	}
}

func providerLabel(provider string) string {
	switch provider {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	case "agy":
		return "agy / Gemini"
	default:
		return provider
	}
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func clone(item *Conversation) Conversation {
	copy := *item
	copy.Messages = slices.Clone(item.Messages)
	copy.Runs = slices.Clone(item.Runs)
	return copy
}

type conversationReply struct {
	Message string `json:"message"`
}

const conversationPrompt = `You are the agent in a Gimble conversation. Work in the conversation worktree and reply to the person's message.

For an ordinary chat request, answer without launching a workflow. When the person asks you to run a built-in workflow, invoke the ordinary Gimble CLI yourself. The executable is %q. Use its "run" help to choose the workflow and flags. Always pass --instance-dir %q, --project %q, --work-dir %q, and --conversation %q. The project owns observation and saved history; the worktree is only the execution directory. For a review use "run review --goal ...". For an implementation use "run implement --outcomes-file ..." with a local outcomes file. The CLI prints the accepted run ID. Do not infer acceptance from your own prose; report what the CLI actually said. You may omit --follow; the server records the run's terminal status independently.

Return only the message requested by the schema.

Person: `

var conversationReplySchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "message": {"type": "string"}
  },
  "required": ["message"],
  "additionalProperties": false
}`)
