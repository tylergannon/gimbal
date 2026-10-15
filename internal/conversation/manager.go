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

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/sessionstate"
	"github.com/tylergannon/polytype"
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
	ID             string                       `json:"id"`
	Title          string                       `json:"title"`
	Provider       string                       `json:"provider"`
	Model          string                       `json:"model"`
	Branch         string                       `json:"branch"`
	Worktree       string                       `json:"worktree"`
	NativeSession  string                       `json:"native_session"`
	Live           bool                         `json:"live"`
	Status         string                       `json:"status"`
	Error          string                       `json:"error"`
	Created        int64                        `json:"created"`
	Updated        int64                        `json:"updated"`
	Messages       []Message                    `json:"messages"`
	Runs           []Run                        `json:"runs"`
	Requests       []RequestState               `json:"requests"`
	ActiveRequest  string                       `json:"active_request"`
	State          gimbal.JSONText              `json:"state"`
	Revision       uint64                       `json:"revision"`
	Environment    string                       `json:"environment"`
	Placement      polytype.Nullable[Placement] `json:"placement"`
	Contact        string                       `json:"contact"`
	Capabilities   environments.Capabilities    `json:"capabilities"`
	CleanupPending bool                         `json:"cleanup_pending"`
}

// Placement exposes display facts, while transport configuration remains in the
// private durable Transport snapshot and is never projected into browser state.
type Placement struct {
	Machine     PlacementMachine             `json:"machine"`
	Environment environments.EnvironmentInfo `json:"environment"`
	Directory   string                       `json:"directory"`
}
type PlacementMachine struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// NewConversation is the provider and optional presentation selected by the
// person opening a conversation.
type NewConversation struct {
	Title          string
	Provider       string
	Model          string
	SharedCheckout bool
	Branch         string
	Base           string
	Environment    string
}

// AdapterFactory resolves a provider name to its existing Gimbal harness.
type AdapterFactory func(string) (gimbal.HarnessAdapter, error)

type activeConversation struct {
	turnMu    sync.Mutex
	admitMu   sync.Mutex
	steerMu   sync.Mutex
	adapter   gimbal.HarnessAdapter
	session   string
	resume    bool
	request   string
	cancel    context.CancelFunc
	remote    *environments.Binding
	observing bool
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

	mu            sync.RWMutex
	items         map[string]*Conversation
	active        map[string]*activeConversation
	projections   map[string]*sessionstate.Projection
	closing       bool
	closed        chan struct{}
	environments  *environments.Service
	projectID     string
	remoteStates  map[string]*remoteState
	remoteReaders sync.WaitGroup
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
		items: make(map[string]*Conversation), active: make(map[string]*activeConversation), projections: make(map[string]*sessionstate.Projection), closed: make(chan struct{}), remoteStates: make(map[string]*remoteState),
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
		var record conversationRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("conversation: decode %s: %w", entry.Name(), err)
		}
		item := record.Conversation
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
		if item.ActiveRequest != "" {
			item.Status, item.Error = "unresolved", "The server restarted before this request's outcome was confirmed."
			if !remotePlacement(&item) {
				item.ActiveRequest = ""
			}
			for i := range item.Requests {
				if item.Requests[i].Status == "running" || (item.Requests[i].Status == "accepted" && !remotePlacement(&item)) {
					item.Requests[i].Status = "unresolved"
				}
			}
		}
		manager.refresh(&item)
		// A persisted native identity can be resumed lazily on the next send.
		// Loading the page itself still does not start or contact a harness.
		if !remotePlacement(&item) {
			item.Live = item.NativeSession != ""
		} else {
			item.Live = item.Live || item.ActiveRequest != "" || item.CleanupPending
		}
		if !item.Placement.Present {
			item.Environment = environments.Local
			item.Contact = "connected"
			item.Capabilities = localCapabilities()
		}
		if len(record.Transport) > 0 {
			var remote remoteState
			if err := json.Unmarshal(record.Transport, &remote); err != nil {
				return nil, fmt.Errorf("conversation: read retained transport: %w", err)
			}
			manager.remoteStates[item.ID] = &remote
		}
		copy := item
		manager.items[item.ID] = &copy
		var state sessionstate.ProjectionState
		if len(item.State) > 0 {
			if err := json.Unmarshal([]byte(item.State), &state); err != nil {
				return nil, err
			}
		}
		manager.projections[item.ID] = sessionstate.New(state)
	}
	context.AfterFunc(ctx, func() { manager.Close() })
	return manager, nil
}

// Create prepares the selected checkout before admitting a conversation. Git
// projects default to an isolated worktree; SharedCheckout uses their directory.
func (m *Manager) Create(ctx context.Context, in NewConversation) (Conversation, error) {
	provider := strings.TrimSpace(in.Provider)
	model, _, err := providerModel(provider, strings.TrimSpace(in.Model))
	if err != nil {
		return Conversation{}, err
	}
	if in.Environment != "" && in.Environment != environments.Local {
		m.mu.RLock()
		service, projectID := m.environments, m.projectID
		m.mu.RUnlock()
		if service == nil {
			return Conversation{}, errors.New("conversation: environment service unavailable")
		}
		if m.pendingRemotePreparation(in, provider, model) != "" {
			return m.createRemote(ctx, in, provider, model)
		}
		target, err := service.Resolve(ctx, projectID, in.Environment)
		if err != nil {
			return Conversation{}, err
		}
		if target.Machine.ID == environments.Local || target.Machine.ID == "" {
			return m.createLocalEnvironment(ctx, in, provider, model, target)
		}
		return m.createRemote(ctx, in, provider, model)
	}
	checkout, err := m.prepareCheckout(ctx, in)
	if err != nil {
		return Conversation{}, err
	}
	if _, err := m.factory(provider); err != nil {
		return Conversation{}, fmt.Errorf("conversation: provider %s: %w", provider, err)
	}
	id, err := randomID()
	if err != nil {
		return Conversation{}, fmt.Errorf("conversation: create id: %w", err)
	}
	branch, worktree := checkout.branch, filepath.Dir(m.projectDir)
	if checkout.repo != "" && !in.SharedCheckout {
		if branch == "" {
			branch = "gimbal/conversation-" + id[:12]
		}
		worktree = filepath.Join(m.projectDir, "conversation-worktrees", id)
		if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
			return Conversation{}, fmt.Errorf("conversation: create worktree directory: %w", err)
		}
		if _, err := gitOutput(ctx, checkout.repo, "worktree", "add", "-b", branch, worktree, checkout.commit); err != nil {
			return Conversation{}, fmt.Errorf("conversation: create worktree: %w", err)
		}
	}

	now := time.Now().UnixMilli()
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = providerLabel(provider) + " conversation"
	}
	item := &Conversation{
		ID: id, Title: title, Provider: provider, Model: model,
		Branch: branch, Worktree: worktree, Live: true, Status: StatusIdle,
		Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{},
		Environment: environments.Local, Contact: "connected", Capabilities: localCapabilities(),
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
	m.projections[id] = sessionstate.New(sessionstate.NewProjectionState())
	m.mu.Unlock()
	return clone(item), nil
}

// Harness is a provider a person can start a conversation with: its cheap
// default model and the newest model of each family it serves.
type Harness struct {
	ID      string   `json:"id"`
	Default string   `json:"default"`
	Models  []string `json:"models"`
}

// Setup is what a new conversation in this project can choose and where it
// will work. Bases are locally available branches; no discovery fetches Git.
// Git means this admitted directory has usable committed history.
type Setup struct {
	Harnesses     []Harness                 `json:"harnesses"`
	Directory     string                    `json:"directory"`
	Git           bool                      `json:"git"`
	Branch        string                    `json:"branch"`
	Unborn        bool                      `json:"unborn"`
	Bases         []string                  `json:"bases"`
	Base          string                    `json:"base"`
	Environment   string                    `json:"environment"`
	Machine       string                    `json:"machine"`
	Capabilities  environments.Capabilities `json:"capabilities"`
	Environments  environments.Snapshot     `json:"environments"`
	HarnessStates []environments.Harness    `json:"harness_states"`
}

// Setup reports the choices and the fixed environment for a new conversation.
func (m *Manager) Setup(ctx context.Context) (Setup, error) {
	newest := modelalias.Newest()
	setup := Setup{Harnesses: []Harness{}, Bases: []string{}}
	for _, provider := range []string{"claude", "codex", "agy"} {
		model, _, err := providerModel(provider, "")
		if err != nil {
			return Setup{}, err
		}
		models := slices.Clone(newest[provider])
		if !slices.Contains(models, model) {
			models = append([]string{model}, models...)
		}
		setup.Harnesses = append(setup.Harnesses, Harness{ID: provider, Default: model, Models: models})
	}
	directory := filepath.Dir(m.projectDir)
	setup.Directory = directory
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(directory, home); ok && (rest == "" || rest[0] == filepath.Separator) {
			setup.Directory = "~" + rest
		}
	}
	repo, err := conversationRepository(ctx, directory)
	if err != nil {
		return Setup{}, fmt.Errorf("conversation: find repository: %w", err)
	}
	if setup.Git = repo != ""; setup.Git {
		setup.Branch, setup.Bases, setup.Base, err = checkoutBases(ctx, repo)
		if err != nil {
			return Setup{}, err
		}
	} else if _, err := os.Stat(filepath.Join(directory, ".git")); err == nil {
		setup.Unborn = true
	}
	return setup, nil
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
		return errors.New("conversation: run must use the conversation's working directory")
	}
	item.Revision++
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
		item.Revision++
		item.Updated = time.Now().UnixMilli()
		_ = m.save(item)
	}
}

// Start creates a conversation titled by its first message and admits that
// message through the same request lifecycle as subsequent turns.
func (m *Manager) Start(ctx context.Context, in NewConversation, text string) (Conversation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Conversation{}, errors.New("conversation: message is blank")
	}
	if strings.TrimSpace(in.Title) == "" {
		in.Title = titleFrom(text)
	}
	item, err := m.Create(ctx, in)
	if err != nil {
		return Conversation{}, err
	}
	requestID, err := randomID()
	if err != nil {
		return Conversation{}, err
	}
	if _, err := m.Submit(item.ID, Request{ID: requestID, Created: time.Now().UnixMilli(), Kind: "prompt", Message: text}); err != nil {
		return Conversation{}, err
	}
	item, _ = m.Get(item.ID)
	return item, nil
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
		if conversation.cancel != nil {
			conversation.cancel()
		}
		active = append(active, conversation)
	}
	m.mu.Unlock()
	// Join even admitted operations that have not acquired turnMu yet.
	m.remoteReaders.Wait()
	for _, conversation := range active {
		conversation.turnMu.Lock()
		if conversation.remote != nil {
			// Shared transport handles belong to the instance environment service.
		} else if conversation.adapter != nil && conversation.session != "" {
			if persistent, ok := conversation.adapter.(persistentSessionAdapter); ok {
				_ = persistent.DetachSession(context.WithoutCancel(m.ctx), conversation.session)
			} else {
				_ = conversation.adapter.Close(context.WithoutCancel(m.ctx), conversation.session)
			}
		}
		conversation.turnMu.Unlock()
	}
}

// background registers before spawning so Close cannot miss a not-yet-started
// observer or queue continuation. Call without holding m.mu.
func (m *Manager) background(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return
	}
	m.remoteReaders.Add(1)
	go func() { defer m.remoteReaders.Done(); fn() }()
}

type conversationRecord struct {
	Conversation
	Transport json.RawMessage `json:"transport,omitempty"`
}

func (m *Manager) save(item *Conversation) error {
	var raw json.RawMessage
	if remote := m.remoteStates[item.ID]; remote != nil {
		var err error
		raw, err = json.Marshal(remote)
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(conversationRecord{Conversation: *item, Transport: raw}, "", "  ")
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
			model = "claude-haiku-4-5"
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
	case "opencode":
		if model == "" {
			model = "opencode/gpt-5-nano"
		}
		return model, "", nil
	case "pi":
		if model == "" {
			model = "anthropic/claude-haiku-4-5"
		}
		return model, "", nil
	default:
		return "", "", fmt.Errorf("conversation: unknown provider %q", provider)
	}
}

func providerLabel(provider string) string {
	switch provider {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude Code"
	case "agy":
		return "Antigravity"
	default:
		return provider
	}
}

// titleFrom names a conversation by the first line of its first message,
// cut at a word near sixty characters.
func titleFrom(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) <= 60 {
		return line
	}
	cut := string(runes[:60])
	if space := strings.LastIndex(cut, " "); space > 30 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

// conversationRepository returns a repository only when it can supply a worktree.
// Chat in an ordinary folder or an unborn repository uses the folder itself.
func conversationRepository(ctx context.Context, directory string) (string, error) {
	// Git can discover a parent repository, but that does not make this folder
	// a Git project. Ordinary folders do not need Git to start a chat.
	if _, err := os.Stat(filepath.Join(directory, ".git")); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	repo, err := gitOutput(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if _, err := gitOutput(ctx, repo, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return repo, nil
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
	copy.Requests = slices.Clone(item.Requests)

	return copy
}

const conversationPrompt = `You are the agent in a Gimbal conversation. Work in the conversation working directory and reply to the person's message.

For an ordinary chat request, answer without launching a workflow. When the person asks you to run a built-in workflow, invoke the ordinary Gimbal CLI yourself. The executable is %q. Use its "run" help to choose the workflow and flags. Always pass --instance-dir %q, --project %q, --work-dir %q, and --conversation %q. The project owns observation and saved history; the working directory is where you execute. For a review use "run review --goal ...". For an implementation use "run implement --outcomes-file ..." with a local outcomes file. The CLI prints the accepted run ID. Do not infer acceptance from your own prose; report what the CLI actually said. You may omit --follow; the server records the run's terminal status independently.

Reply in ordinary prose.

Person: `
