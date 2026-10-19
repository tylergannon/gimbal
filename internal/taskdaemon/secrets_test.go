package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

func waitTask(t *testing.T, d *Daemon, o backend.Owner) backend.ProcessResult {
	t.Helper()
	d.mu.Lock()
	task := d.tasks[key(o.Run, o.Task)]
	d.mu.Unlock()
	select {
	case <-task.done:
	case <-time.After(3 * time.Second):
		t.Fatal("task did not complete")
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	var r backend.ProcessResult
	if err := json.Unmarshal(task.result, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestProducerAdmissionAndScopedCleanup(t *testing.T) {
	d := New(t.TempDir())
	d.Secrets = backend.NewMemorySecrets()
	ctx := context.Background()
	producer := backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Run: "run", Scope: "outer", Task: "producer", Command: "sh", Args: []string{"-c", `printf 'TOKEN=from-producer\nUNDECLARED=no\n' > "$ENV_FILE"`}, Produces: true, Yields: []string{"TOKEN"}, Store: "private"}
	if err := d.process(producer); err != nil {
		t.Fatal(err)
	}
	if r := waitTask(t, d, producer.Owner); r.Error != "" || r.ExitCode != 0 {
		t.Fatal(r)
	}
	consumer := backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Run: "run", Scope: "outer/child", Task: "consumer", Command: "sh", Args: []string{"-c", `test "$TOKEN" = from-producer && test -z "$UNDECLARED"`}, Requires: backend.Requirements{{Store: "private", Names: []string{"TOKEN"}}}}
	if err := d.process(consumer); err != nil {
		t.Fatal(err)
	}
	if r := waitTask(t, d, consumer.Owner); r.ExitCode != 0 || r.Error != "" {
		t.Fatal(r)
	}
	consumer.Task = "missing"
	consumer.Requires[0].Names = []string{"MISSING"}
	if err := d.process(consumer); err == nil {
		t.Fatal("missing requirement admitted")
	}
	if d.tasks[key("run", "missing")] != nil {
		t.Fatal("native task admitted")
	}
	if err := d.Secrets.Publish(ctx, backend.Scope{Run: "other"}, backend.StoreWrite{Store: "private", Values: []backend.NamedValue{{Name: "TOKEN", Value: "unrelated"}}}); err != nil {
		t.Fatal(err)
	}
	if err := d.cleanup(ctx, backend.Scope{Run: "run", Scope: "outer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Secrets.Acquire(ctx, backend.Scope{Run: "run", Scope: "outer"}, backend.Requirements{{Store: "private", Names: []string{"TOKEN"}}}); err == nil {
		t.Fatal("released values remain")
	}
	if _, err := d.Secrets.Acquire(ctx, backend.Scope{Run: "other"}, backend.Requirements{{Store: "private", Names: []string{"TOKEN"}}}); err != nil {
		t.Fatal("unrelated release", err)
	}
}
func TestEmptyServiceWaitsForReadinessAndCleansUp(t *testing.T) {
	d := New(t.TempDir())
	p := backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Run: "run", Task: "service", Command: "sh", Args: []string{"-c", `trap 'exit 0' INT; sleep .15; touch "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; while :; do sleep .1; done`}, Service: true}
	start := time.Now()
	if err := d.process(p); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatal("launch treated as readiness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := d.cleanup(ctx, backend.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
	if d.state(backend.Scope{Run: "run"}).Pending != 0 {
		t.Fatal("native ownership retained")
	}
}

type failedPublish struct{ backend.SecretsBackend }

func (f failedPublish) Publish(context.Context, backend.Scope, backend.StoreWrite) error {
	return errors.New("publication uncertain")
}
func TestPublicationFailurePreventsReadiness(t *testing.T) {
	d := New(t.TempDir())
	d.Secrets = failedPublish{backend.NewMemorySecrets()}
	p := backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Run: "run", Task: "bad", Command: "sh", Args: []string{"-c", `trap 'exit 0' INT; printf 'TOKEN=x\n' > "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; while :; do sleep .1; done`}, Service: true, Yields: []string{"TOKEN"}}
	if err := d.process(p); err == nil || !strings.Contains(err.Error(), "publication uncertain") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := d.cleanup(ctx, backend.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
}

type callbackHarness struct {
	testHarness
	got    map[string]string
	prompt string
}

func (h *callbackHarness) RunTurn(_ context.Context, _, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	h.prompt = prompt
	address := regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+/secrets/[A-Z0-9]+`).FindString(prompt)
	response, err := http.Get(address)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	defer func() { _ = response.Body.Close() }()
	err = json.NewDecoder(response.Body).Decode(&h.got)
	return gimbal.TurnResult{Output: json.RawMessage(`"done"`)}, err
}
func TestAgentCallbackUsesConsumingScopeAndWholeStore(t *testing.T) {
	d := New(t.TempDir())
	d.Secrets = backend.NewMemorySecrets()
	ctx := context.Background()
	for scope, values := range map[string][]backend.NamedValue{"": {{Name: "TOKEN", Value: "parent"}}, "child": {{Name: "TOKEN", Value: "child-secret"}, {Name: "EXTRA", Value: "whole-store"}}} {
		if err := d.Secrets.Publish(ctx, backend.Scope{Run: "run", Scope: scope}, backend.StoreWrite{Store: "private", Values: values}); err != nil {
			t.Fatal(err)
		}
	}
	h := &callbackHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return h, nil }
	if _, err := d.create(ctx, backend.Session{ExecutionIdentity: backend.NewExecutionIdentity("session"), Run: "run", Task: "agent"}); err != nil {
		t.Fatal(err)
	}
	o := backend.Owner{Run: "run", Scope: "child", Task: "turn"}
	if err := d.turn(backend.Turn{ExecutionIdentity: backend.NewExecutionIdentity("turn"), Owner: o, Session: "agent", Requires: backend.Requirements{{Store: "private", Names: []string{"TOKEN"}}}}); err != nil {
		t.Fatal(err)
	}
	task := d.tasks[key("run", "turn")]
	select {
	case <-task.done:
	case <-time.After(time.Second):
		t.Fatal("turn timeout")
	}
	if task.err != nil || h.got["TOKEN"] != "child-secret" || h.got["EXTRA"] != "whole-store" {
		t.Fatal(task.err, h.got)
	}
	if strings.Contains(h.prompt, "child-secret") {
		t.Fatal("secret automatically inserted")
	}
	_ = d.cleanup(ctx, backend.Scope{Run: "run"})
}

func TestCancelServiceBeforeReadiness(t *testing.T) {
	d := New(t.TempDir())
	owner := backend.Owner{Run: "run", Scope: "child", Task: "service"}
	result := make(chan error, 1)
	go func() {
		result <- d.process(backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Owner: owner, Command: "sh", Args: []string{"-c", "exec sleep 60"}, Service: true})
	}()
	deadline := time.After(time.Second)
	for {
		d.mu.Lock()
		started := d.tasks[key(owner.Run, owner.Task)] != nil
		d.mu.Unlock()
		if started {
			break
		}
		select {
		case <-deadline:
			t.Fatal("native startup timeout")
		case <-time.After(time.Millisecond):
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := d.cleanup(ctx, backend.Scope{Run: owner.Run, Scope: owner.Scope}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled service became ready")
		}
	case <-ctx.Done():
		t.Fatal("readiness wait did not cancel")
	}
}
