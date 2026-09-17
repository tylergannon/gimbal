package web

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/runlog"
	routes "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
)

// planning answers a loop's planner: the first decision takes on the one
// task, the next ends dispatch. Its prompts are kept so a test can read
// what the planner was told.
type planning struct {
	mu      sync.Mutex
	prompts []string
}

func (*planning) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-planner", nil
}

func (p *planning) RunTurn(_ context.Context, _, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	p.mu.Lock()
	p.prompts = append(p.prompts, prompt)
	first := len(p.prompts) == 1
	p.mu.Unlock()
	plan := map[string]any{
		"tasks": []any{map[string]any{
			"name": "do the thing", "description": "Do it.", "definition_of_done": "It is done.",
			"validation": map[string]any{"command": "", "query": ""},
		}},
		"next": nil,
	}
	if first {
		plan["next"] = 0
	}
	raw, err := json.Marshal(plan)
	return gimble.TurnResult{Output: raw}, err
}

func (*planning) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*planning) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*planning) Close(context.Context, string) error                 { return nil }

func (p *planning) said(n int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.prompts) < n {
		return ""
	}
	return p.prompts[n-1]
}

func TestIterateScopesHaveNoPlannerControls(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runtime.Run(ctx, "repeating", nil, func(ctx context.Context) error {
			for ctx := range gimble.Iterate(ctx, "round", []string{"one"}) {
				gimble.Set(ctx, "answer", "visible in this iteration")
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
				break
			}
			return nil
		})
	}()
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Errorf("iteration run: %v", err)
		}
	}()
	<-entered
	id := runID(t, project)
	response, err := http.Get("http://" + runtime.address + "/runs/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	document, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(document), "round.1") {
		t.Fatalf("iteration page: status %d, body %s", response.StatusCode, document)
	}
	if strings.Contains(string(document), "Wrap up") || strings.Contains(string(document), "Say something to the planner") {
		t.Fatal("plain iteration offered planner controls")
	}
	var status *skgo.HTTPError
	_, err = routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "round.1", Message: "hello"})
	if !errors.As(err, &status) || status.Status != http.StatusNotFound {
		t.Fatalf("steer plain loop = %v, want 404", err)
	}
}

// TestLoopFormReachesThePlannerOfALoop is the page half of #235: the wrap-up
// button and the message box on a loop's card post to the same run the
// runtime reaches, the message waits while a task runs and no turn is
// running, and the planner is told at its next decision. What the page
// cannot deliver is refused: an empty box, a scope that is not a loop, and
// a run that has finished since the page was drawn.
func TestLoopFormReachesThePlannerOfALoop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	p := &planning{}
	dispatched, sent := make(chan struct{}), make(chan struct{})
	var runWG sync.WaitGroup
	runWG.Go(func() {
		_ = runtime.Run(ctx, "looping", map[gimble.WorkflowRole]gimble.ModelBinding{"planner": {Adapter: p, Model: "m"}}, func(ctx context.Context) error {
			planner := gimble.NewSession(ctx, "planner", "/w")
			loop := gimble.PromiseLoop(ctx, "sprint", "ship it", planner)
			for range loop.Tasks {
				close(dispatched)
				<-sent
			}
			return loop.Err()
		})
	})

	<-dispatched
	id := runID(t, project)

	waiting, err := routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "sprint.1", WrapUp: true})
	if err != nil || waiting.Message != gimble.WrapUp {
		t.Fatalf("wrap up = %+v, %v; want the runtime's own wrap-up message", waiting, err)
	}
	if waiting, err := routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "sprint.1", Message: "  and nothing after it  "}); err != nil || waiting.Message != "and nothing after it" {
		t.Fatalf("message = %+v, %v; want it waiting, trimmed", waiting, err)
	}

	// An empty box is the field's problem, not the server's.
	var invalid *skgo.Invalid
	if _, err := routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "sprint.1", Message: "   "}); !errors.As(err, &invalid) {
		t.Fatalf("empty message = %v, want an issue on the field", err)
	}
	// Only a loop takes messages: a task's own scope is live and is not one.
	var status *skgo.HTTPError
	if _, err := routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "sprint.1/task.1", Message: "hello"}); !errors.As(err, &status) || status.Status != 404 {
		t.Errorf("a message to a task scope = %v, want a 404", err)
	}

	close(sent)
	runWG.Wait()

	if next := p.said(2); !strings.Contains(next, gimble.WrapUp) || !strings.Contains(next, "and nothing after it") {
		t.Fatalf("the planner was not told what the page sent:\n%s", next)
	}
	var steered []gimble.LifecycleRecord
	if err := runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(project, "runs", id), func(record gimble.LifecycleRecord) error {
		if _, ok := record.Event.(gimble.Steer); ok {
			steered = append(steered, record)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(steered) != 2 {
		t.Fatalf("Steer records = %+v, want two on the loop", steered)
	}
	for _, record := range steered {
		steer := record.Event.(gimble.Steer)
		if record.Scope != "sprint.1" || steer.Target != "sprint.1" || steer.Source != "person" || !steer.Landed {
			t.Errorf("Steer record = %+v %+v, want it landed on sprint.1 from the person", record, steer)
		}
	}
	if _, err := routes.Skgo_steerLoop(runtime.ctx, routes.LoopMessage{Run: id, Scope: "sprint.1", Message: "too late"}); !errors.As(err, &status) || status.Status != 404 {
		t.Errorf("finished run = %v, want a 404", err)
	}
}

// loopForm is the form on the loop's card whose hidden wrap_up field holds
// want ("on" for the wrap-up button, "off" for the message box), as the
// rendered document holds it: its action and every field a browser would
// post.
func loopForm(t *testing.T, body, want string) (string, url.Values) {
	t.Helper()
	for _, form := range regexp.MustCompile(`(?s)<form[^>]*class="loop[^"]*"[^>]*>.*?</form>`).FindAllString(body, -1) {
		action := regexp.MustCompile(`action="([^"]*)"`).FindStringSubmatch(form)
		if action == nil {
			t.Fatalf("a loop form has no action:\n%s", form)
		}
		fields := url.Values{}
		for _, input := range regexp.MustCompile(`<(?:input|textarea)[^>]*>`).FindAllString(form, -1) {
			name := regexp.MustCompile(`name="([^"]*)"`).FindStringSubmatch(input)
			value := regexp.MustCompile(`value="([^"]*)"`).FindStringSubmatch(input)
			if name == nil || value == nil {
				t.Fatalf("a loop form field has no name or value:\n%s", input)
			}
			fields.Set(html.UnescapeString(name[1]), html.UnescapeString(value[1]))
		}
		for field, value := range fields {
			if strings.Contains(field, "wrap_up") && value[0] == want {
				return html.UnescapeString(action[1]), fields
			}
		}
	}
	t.Fatalf("no loop form with wrap_up %q in:\n%s", want, body)
	return "", nil
}

// TestTheWrapUpButtonWorksWithoutJavaScript posts the loop card's forms as
// the browser itself would, from the document the page rendered: the
// wrap-up button and a typed message, each to the action and fields it
// actually carries. It is the whole path, with no client in it.
func TestTheWrapUpButtonWorksWithoutJavaScript(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	p := &planning{}
	dispatched, sent := make(chan struct{}), make(chan struct{})
	var runWG sync.WaitGroup
	runWG.Go(func() {
		_ = runtime.Run(ctx, "looping", map[gimble.WorkflowRole]gimble.ModelBinding{"planner": {Adapter: p, Model: "m"}}, func(ctx context.Context) error {
			planner := gimble.NewSession(ctx, "planner", "/w")
			loop := gimble.PromiseLoop(ctx, "sprint", "ship it", planner)
			for range loop.Tasks {
				close(dispatched)
				<-sent
			}
			return loop.Err()
		})
	})
	<-dispatched
	id := runID(t, project)

	page := "http://" + runtime.address + "/runs/" + id
	response, err := http.Get(page)
	if err != nil {
		t.Fatal(err)
	}
	document, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, post := range []struct {
		wrapUp  string
		message string
	}{{wrapUp: "on"}, {wrapUp: "off", message: "one typed message"}} {
		action, fields := loopForm(t, string(document), post.wrapUp)
		for field := range fields {
			if strings.Contains(field, "message") {
				fields.Set(field, post.message)
			}
		}
		// A browser states the origin it posted from, and kit refuses a form
		// POST that does not: the header is part of the submission, not of
		// the client's scripting.
		request, err := http.NewRequest(http.MethodPost, page+action, strings.NewReader(fields.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", "http://"+runtime.address)
		submitted, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = submitted.Body.Close()
		if submitted.StatusCode >= 400 {
			t.Fatalf("posting the %s form: status %d", post.wrapUp, submitted.StatusCode)
		}
	}

	close(sent)
	runWG.Wait()
	if next := p.said(2); !strings.Contains(next, gimble.WrapUp) || !strings.Contains(next, "one typed message") {
		t.Fatalf("the planner was not told what the forms posted:\n%s", next)
	}
}
