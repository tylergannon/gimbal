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
)

const (
	StatusIdle    = "idle"
	StatusWorking = "working"
	StatusError   = "error"
)

// Message is one visible turn in a conversation transcript.
type Message struct {
	Role    string `json:"role"`
	Text    string `json:"text"`
	Created int64  `json:"created"`
}

// Conversation is the durable metadata and visible transcript for one chat.
type Conversation struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Branch   string    `json:"branch"`
	Worktree string    `json:"worktree"`
	Live     bool      `json:"live"`
	Status   string    `json:"status"`
	Error    string    `json:"error"`
	Created  int64     `json:"created"`
	Updated  int64     `json:"updated"`
	Messages []Message `json:"messages"`
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
}

// Manager is owned by one web.Runtime. Durable state is under projectDir;
// adapters and native session identities exist only for that runtime's life.
type Manager struct {
	ctx        context.Context
	projectDir string
	factory    AdapterFactory

	mu      sync.RWMutex
	items   map[string]*Conversation
	active  map[string]*activeConversation
	closing bool
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
func New(ctx context.Context, projectDir string, factory AdapterFactory) (*Manager, error) {
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
		ctx: ctx, projectDir: projectDir, factory: factory,
		items: make(map[string]*Conversation), active: make(map[string]*activeConversation),
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
		// A process cannot honestly claim that a native session from the prior
		// runtime is still working. Its saved transcript remains readable.
		if item.Status == StatusWorking {
			item.Status = StatusIdle
		}
		item.Live = false
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
		Created: now, Updated: now, Messages: []Message{},
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
	m.mu.RLock()
	items := make([]Conversation, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, clone(item))
	}
	m.mu.RUnlock()
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
	m.mu.RLock()
	item := m.items[id]
	m.mu.RUnlock()
	if item == nil {
		return Conversation{}, false
	}
	return clone(item), true
}

// Send records the user's message, runs one turn on the conversation's live
// provider session in its worktree, and records the final assistant response.
func (m *Manager) Send(id, text string) (Conversation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Conversation{}, errors.New("conversation: message is blank")
	}
	m.mu.RLock()
	item := m.items[id]
	active := m.active[id]
	m.mu.RUnlock()
	if item == nil {
		return Conversation{}, fmt.Errorf("conversation: unknown conversation %q", id)
	}
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
	if active.session == "" {
		_, effort, _ := providerModel(provider, model)
		session, err := active.adapter.CreateSession(m.ctx, model, effort, worktree)
		if err != nil {
			return m.fail(item, err)
		}
		active.session = session
	}
	result, err := active.adapter.RunTurn(m.ctx, active.session, text, nil, func(gimble.AgentEvent) error { return nil })
	if err != nil {
		return m.fail(item, err)
	}
	var response string
	if err := json.Unmarshal(result.Output, &response); err != nil {
		return m.fail(item, fmt.Errorf("provider returned an invalid text response: %w", err))
	}

	m.mu.Lock()
	item = m.items[id]
	now = time.Now().UnixMilli()
	item.Messages = append(item.Messages, Message{Role: "assistant", Text: response, Created: now})
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
		return
	}
	m.closing = true
	active := make([]*activeConversation, 0, len(m.active))
	for _, conversation := range m.active {
		active = append(active, conversation)
	}
	m.mu.Unlock()
	for _, conversation := range active {
		conversation.turnMu.Lock()
		if conversation.adapter != nil && conversation.session != "" {
			_ = conversation.adapter.Close(context.WithoutCancel(m.ctx), conversation.session)
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
			model = "gpt-5.6-luna"
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
	return copy
}
