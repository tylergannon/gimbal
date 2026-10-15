package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"github.com/tylergannon/gimbal/internal/environments"
)

type conversationBus struct {
	*messagefiles.Store
	mu                         sync.Mutex
	requests                   map[string]backend.MessageRequest
	turns                      map[string]backend.MessageRequest
	admissions                 int
	cleanupFailures            int
	cleanupPublicationFailures int
}

func (b *conversationBus) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	if _, err := ulid.ParseStrict(r.ID); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.Kind == "cleanup" && b.cleanupPublicationFailures > 0 {
		b.cleanupPublicationFailures--
		return errors.New("publication reply lost")
	}
	if _, ok := b.requests[r.ID]; ok {
		return nil
	}
	b.requests[r.ID] = r
	emit := func(request backend.MessageRequest, result *backend.MessageResult) error {
		return b.Store.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: request.ID, Created: time.Now().UTC(), Owner: request.Owner, SessionID: request.SessionID, Kind: "completed", Result: result})
	}
	switch r.Kind {
	case "process":
		var in environments.HelperRequest
		_ = json.Unmarshal([]byte(r.Process.Args[1]), &in)
		out := environments.HelperResult{Directory: environments.Directory{Path: in.Path, Exists: true, ExecutionDirectory: in.Path}, Prepared: environments.Prepared{Directory: in.Path}, Harnesses: []environments.Harness{{ID: "claude", Default: "claude-haiku-4-5", Models: []string{"claude-haiku-4-5"}, State: environments.HarnessReady}}}
		raw, _ := json.Marshal(out)
		return emit(r, &backend.MessageResult{Process: &backend.ProcessResult{}, Stdout: raw})
	case "session":
		return emit(r, &backend.MessageResult{ID: r.Task})
	case "turn":
		b.admissions++
		b.turns[r.Task] = r
		if b.admissions > 1 {
			return emit(r, &backend.MessageResult{Output: []byte(`"queued reply"`)})
		}
		return nil
	case "interrupt":
		target := b.turns[r.Target.ID]
		if err := emit(target, &backend.MessageResult{Interrupted: true}); err != nil {
			return err
		}
		return emit(r, &backend.MessageResult{})
	case "cleanup":
		if b.cleanupFailures > 0 {
			b.cleanupFailures--
			return emit(r, &backend.MessageResult{Unresolved: true, Error: "native Close failed"})
		}
		return emit(r, &backend.MessageResult{})
	}
	return nil
}

func TestMessagingConversationQueuesAndStopsByRetainedEnvelopeIdentity(t *testing.T) {
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := &conversationBus{Store: store, requests: map[string]backend.MessageRequest{}, turns: map[string]backend.MessageRequest{}}
	registry := &gimbal.Registry{}
	err = registry.RegisterMessaging(gimbal.MessagingDefinition{Name: "fixture-messaging", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		return bus, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	project, instance := t.TempDir(), t.TempDir()
	raw, _ := json.Marshal(environments.Config{Version: 1, Machines: []environments.Machine{{ID: "queue", Name: "Queue worker", Kind: environments.KindRoute, Route: &backend.Route{Name: "fixture-messaging", Configuration: json.RawMessage(`{}`)}}}, Environments: []environments.Environment{{ID: "queue-project", Project: "project", Machine: "queue", Directory: "/queue/project"}}})
	if err = os.WriteFile(filepath.Join(instance, "environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := environments.New(t.Context(), environments.Options{InstanceDir: instance, Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err = service.RegisterProject("project", project); err != nil {
		t.Fatal(err)
	}
	manager, err := New(t.Context(), filepath.Join(project, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { t.Fatal("queue used local adapter"); return nil, nil }, "/local/gimbal", instance, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.SetEnvironments(service, "project")
	item, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Environment: "queue-project", SharedCheckout: true}, "Start on queue")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		bus.mu.Lock()
		started := bus.admissions == 1
		bus.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue turn not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = manager.Submit(item.ID, Request{ID: "followup-domain-id", Created: time.Now().UnixMilli(), Kind: "prompt", Message: "Queued follow-up"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	bus.mu.Lock()
	admissions := bus.admissions
	bus.mu.Unlock()
	if admissions != 1 {
		t.Fatalf("queued message started early: %d", admissions)
	}
	original := item.Requests[0].ID
	if _, err = manager.Submit(item.ID, Request{ID: "stop-domain-id", Created: time.Now().UnixMilli(), Kind: "interrupt", Target: original}); err != nil {
		t.Fatal(err)
	}
	waitRequest(t, manager, item.ID, original, "canceled")
	waitRequest(t, manager, item.ID, "stop-domain-id", "completed")
	final := waitRequest(t, manager, item.ID, "followup-domain-id", "completed")
	if final.Capabilities.Steer || len(final.Messages) != 3 || final.Messages[2].Text != "queued reply" {
		t.Fatalf("queue state=%+v", final)
	}
	manager.mu.RLock()
	envelope := manager.remoteStates[item.ID].Requests[original]
	manager.mu.RUnlock()
	if envelope.ID == original || envelope.Turn.Owner.Task != original || envelope.Created.UnixMilli() != item.Requests[0].Created {
		t.Fatalf("identity=%+v", envelope)
	}
	if err = manager.Cleanup(t.Context(), item.ID); err != nil {
		t.Fatal(err)
	}
	// A fresh cleanup completion cannot substitute for the original turn event.
	bus.mu.Lock()
	bus.admissions = 0
	bus.mu.Unlock()
	lost, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Environment: "queue-project", SharedCheckout: true}, "Retain missing terminal fact")
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for {
		bus.mu.Lock()
		started := bus.admissions > 0
		bus.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue turn not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	budget, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	if err = manager.Cleanup(budget, lost.ID); err == nil {
		t.Fatal("cleanup event invented original turn cessation")
	}
	pending, _ := manager.Get(lost.ID)
	if !pending.Live || !pending.CleanupPending || pending.ActiveRequest != lost.Requests[0].ID {
		t.Fatalf("missing original event lost ownership: %+v", pending)
	}

	// A completed cleanup control failure is retryable with a fresh control ID;
	// republishing its old ID would replay that retained failure forever.
	idle, err := manager.Create(t.Context(), NewConversation{Provider: "claude", Environment: "queue-project", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	bus.mu.Lock()
	bus.cleanupFailures = 1
	bus.mu.Unlock()
	if err = manager.Cleanup(t.Context(), idle.ID); err == nil {
		t.Fatal("cleanup failure lost")
	}
	manager.mu.RLock()
	failedID := manager.remoteStates[idle.ID].Requests["cleanup"].ID
	manager.mu.RUnlock()
	if err = manager.Cleanup(t.Context(), idle.ID); err != nil {
		t.Fatal(err)
	}
	manager.mu.RLock()
	retriedID := manager.remoteStates[idle.ID].Requests["cleanup"].ID
	manager.mu.RUnlock()
	closed, _ := manager.Get(idle.ID)
	if failedID == retriedID || closed.Live || closed.CleanupPending {
		t.Fatalf("failed cleanup could not be retried: %s %s %+v", failedID, retriedID, closed)
	}

	uncertain, err := manager.Create(t.Context(), NewConversation{Provider: "claude", Environment: "queue-project", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	bus.mu.Lock()
	bus.cleanupPublicationFailures = 1
	bus.mu.Unlock()
	if err = manager.Cleanup(t.Context(), uncertain.ID); err == nil {
		t.Fatal("uncertain publication lost")
	}
	manager.mu.RLock()
	originalCleanupID := manager.remoteStates[uncertain.ID].Requests["cleanup"].ID
	manager.mu.RUnlock()
	if err = manager.Cleanup(t.Context(), uncertain.ID); err != nil {
		t.Fatal(err)
	}
	manager.mu.RLock()
	resumedCleanupID := manager.remoteStates[uncertain.ID].Requests["cleanup"].ID
	manager.mu.RUnlock()
	if resumedCleanupID != originalCleanupID {
		t.Fatal("uncertain cleanup publication replaced retained identity")
	}

}

// Stop reconciliation and the original reader may see one retained completion
// concurrently; it must produce one visible reply, not merely one request row.
func TestConcurrentRemoteSettlementAppendsOneNativeReply(t *testing.T) {
	m, item := managerWithControl(t, &controlledChat{})
	r := request("one-native-result", "One request")
	m.mu.Lock()
	m.active[item.ID].request = r.ID
	if err := m.reduce(m.items[item.ID], requestEvent{kind: "accepted", request: r}); err != nil {
		t.Fatal(err)
	}
	if err := m.reduce(m.items[item.ID], requestEvent{kind: "started", request: r, delivery: "turn"}); err != nil {
		t.Fatal(err)
	}
	if err := m.userEvent(m.items[item.ID], r, "turn"); err != nil {
		t.Fatal(err)
	}
	m.mu.Unlock()
	start := make(chan struct{})
	var readers sync.WaitGroup
	for range 16 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			m.settleRemote(item.ID, r, "completed", "Single native reply", nil)
		}()
	}
	close(start)
	readers.Wait()
	got, _ := m.Get(item.ID)
	if len(got.Messages) != 2 || got.Messages[1].Text != "Single native reply" || len(got.Requests) != 1 || got.Requests[0].Status != "completed" || got.ActiveRequest != "" {
		t.Fatalf("duplicate settlement: %+v", got)
	}
}
