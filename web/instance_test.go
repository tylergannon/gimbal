package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/host"

	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/observation"
	generated "github.com/tylergannon/gimbal/internal/skgo"
	"github.com/tylergannon/polytype/devalue"
)

func TestInstanceOwnsEndpointsAndProjectsOwnState(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	projectA := filepath.Join(base, "a")
	projectB := filepath.Join(base, "b")
	projectC := filepath.Join(base, "c")
	for _, project := range []string{projectA, projectB, projectC} {
		if err := os.MkdirAll(filepath.Join(project, ".gimbal", "conversations"), 0o755); err != nil {
			t.Fatal(err)
		}
		item := conversation.Conversation{ID: filepath.Base(project), Title: "Selected conversation " + filepath.Base(project)}
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, ".gimbal", "conversations", item.ID+".json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	i, err := NewInstance(ctx, filepath.Join(base, "instance-one"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	a, err := i.Owner.AdmitProject(projectA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := i.Owner.AdmitProject(projectB)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := i.Owner.AdmitProject(projectA); err != nil || again != a {
		t.Fatalf("readmit project: %p, %v", again, err)
	}
	alias := filepath.Join(base, "alias-a")
	if err := os.Symlink(projectA, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alias, filepath.Join(base, "b", "..", "a"), projectA} {
		if again, err := i.Owner.AdmitProject(path); err != nil || again != a {
			t.Fatalf("alias %s admitted separate project: %p, %v", path, again, err)
		}
	}
	if a.Registry() == b.Registry() || a.Runs() == b.Runs() || a.Conversations() == b.Conversations() {
		t.Fatal("projects share live state")
	}
	if a.Owner() != b.Owner() {
		t.Fatal("admission made a second owner or listener")
	}
	for _, p := range []*host.Project{a, b} {
		items := p.Conversations().List()
		if len(items) != 1 || items[0].ID != filepath.Base(p.Path()) {
			t.Fatalf("conversation state for %s: %+v", p.Dir(), items)
		}
	}
	if entries, err := filepath.Glob(filepath.Join(i.dir, "control", "*.json")); err != nil || len(entries) != 1 {
		t.Fatalf("instance control discovery: %v, %v", entries, err)
	}
	for _, project := range []string{projectA, projectB} {
		if entries, err := filepath.Glob(filepath.Join(project, ".gimbal", "control", "*.json")); err != nil || len(entries) != 1 {
			t.Fatalf("project discovery: %v, %v", entries, err)
		}
	}
	j, err := NewInstance(ctx, filepath.Join(base, "instance-two"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	c, err := j.Owner.AdmitProject(projectC)
	if err != nil {
		t.Fatal(err)
	}
	if i.address == j.address || i.dir == j.dir {
		t.Fatal("independent instances share endpoint or state")
	}
	for _, host := range []*Instance{i, j} {
		response, err := http.Get("http://" + host.address + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("web listener %s: status %d", host.address, response.StatusCode)
		}
	}

	started := make(chan struct{}, 3)
	finish := map[*host.Project]chan struct{}{a: make(chan struct{}), b: make(chan struct{}), c: make(chan struct{})}
	var wg sync.WaitGroup
	for _, p := range []*host.Project{a, b, c} {
		wg.Go(func() {
			if err := p.Run(ctx, "held", nil, func(context.Context) error {
				started <- struct{}{}
				<-finish[p]
				return nil
			}); err != nil {
				t.Errorf("run in %s: %v", p.Dir(), err)
			}
		})
	}
	t.Cleanup(func() {
		for _, ch := range finish {
			select {
			case <-ch:
			default:
				close(ch)
			}
		}
		wg.Wait()
	})
	for range 3 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("runs did not overlap")
		}
	}
	for _, p := range []*host.Project{a, b, c} {
		rows, err := controlRuns(p)
		if err != nil || len(rows) != 1 || rows[0].Status != observation.StatusRunning {
			t.Fatalf("live runs in %s: %+v, %v", p.Dir(), rows, err)
		}
		if _, err := os.Stat(filepath.Join(p.Dir(), "runs", rows[0].ID, "run.jsonl")); err != nil {
			t.Fatal(err)
		}
	}
	if got := instanceControlRuns(t, i, projectA); len(got) != 1 {
		t.Fatalf("A control rows: %+v", got)
	}
	if got := instanceControlRuns(t, i, projectB); len(got) != 1 {
		t.Fatalf("B control rows: %+v", got)
	}
	if got := instanceControlRuns(t, j, projectC); len(got) != 1 {
		t.Fatalf("C control rows: %+v", got)
	}
	aID := instanceControlRuns(t, i, projectA)[0].ID
	bID := instanceControlRuns(t, i, projectB)[0].ID
	stream, err := (&http.Client{Timeout: 2 * time.Second}).Get("http://" + i.address + "/projects/" + a.ID() + "/api/runs/" + aID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	if stream.StatusCode != http.StatusOK || !strings.HasPrefix(stream.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("A stream: status %d, content type %q", stream.StatusCode, stream.Header.Get("Content-Type"))
	}
	_ = stream.Body.Close()
	for _, test := range []struct {
		path, contains, absent string
		status                 int
	}{
		{"/projects/" + a.ID(), aID, bID, http.StatusOK},
		{"/projects/" + b.ID(), bID, aID, http.StatusOK},
		{"/projects/" + a.ID() + "/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.ID() + "/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.ID() + "/api/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.ID() + "/api/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.ID() + "/conversations/a", "Selected conversation a", "Selected conversation b", http.StatusOK},
		{"/projects/" + b.ID() + "/conversations/b", "Selected conversation b", "Selected conversation a", http.StatusOK},
		{"/projects/" + b.ID() + "/api/runs/" + aID + "/events", "", "", http.StatusNotFound},
	} {
		response, err := http.Get("http://" + i.address + test.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.StatusCode != test.status || test.contains != "" && !strings.Contains(string(body), test.contains) || test.absent != "" && strings.Contains(string(body), test.absent) {
			t.Fatalf("GET %s: status %d, expected %d, contains %q, absent %q", test.path, response.StatusCode, test.status, test.contains, test.absent)
		}
	}
	if status := instanceSnapshotStatus(t, i, projectA, aID); status != http.StatusOK {
		t.Fatalf("A snapshot status %d", status)
	}
	if status := instanceSnapshotStatus(t, i, projectB, bID); status != http.StatusOK {
		t.Fatalf("B snapshot status %d", status)
	}
	if status := instanceSnapshotStatus(t, i, projectB, aID); status != http.StatusNotFound {
		t.Fatalf("A run visible in B: status %d", status)
	}
	releaseProbe := a.Runs().Hook("control-probe", &controlledRun{})
	defer releaseProbe()
	if status := instanceControlStatus(t, i, projectA, "control-probe"); status != http.StatusOK {
		t.Fatalf("A control status %d", status)
	}
	for _, project := range []string{"", projectB} {
		if status := instanceControlStatus(t, i, project, "control-probe"); status != http.StatusNotFound {
			t.Fatalf("foreign or unqualified control reached A: project %q, status %d", project, status)
		}
	}
	if err := a.KillScope(bID, "", "test", "wrong project"); err == nil {
		t.Fatal("A controlled B's run")
	}
	close(finish[a])
	deadline := time.After(5 * time.Second)
	for {
		rows, _ := controlRuns(a)
		if len(rows) == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("A did not end")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if got := instanceControlRuns(t, i, projectB); len(got) != 1 {
		t.Fatalf("B ended with A: %+v", got)
	}
	if got := instanceControlRuns(t, j, projectC); len(got) != 1 {
		t.Fatalf("C ended with A: %+v", got)
	}
	close(finish[b])
	close(finish[c])
	wg.Wait()
	cancel()
	<-i.done
	reopened, err := NewInstance(t.Context(), filepath.Join(base, "restart"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{projectA, projectB} {
		if _, err := reopened.Owner.AdmitProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ path, id string }{
		{"/projects/" + a.ID() + "/runs/" + aID, aID},
		{"/projects/" + b.ID() + "/runs/" + bID, bID},
		{"/projects/" + a.ID() + "/conversations/a", "a"},
		{"/projects/" + b.ID() + "/conversations/b", "b"},
	} {
		response, err := http.Get("http://" + reopened.address + test.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK || !strings.Contains(string(body), test.id) {
			t.Fatalf("reopen %s: status %d, missing %s", test.path, response.StatusCode, test.id)
		}
	}
}

func TestRouteAdmissionIsImmediatelyVisibleToPageAndControl(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := NewInstance(ctx, filepath.Join(base, "instance"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	instance.projectRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner := host.OwnerFrom(r.Context())
		if owner == nil {
			t.Error("route has no host owner")
			return
		}
		if _, err := owner.AdmitProject(project); err != nil {
			t.Error(err)
		}
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	admitted, err := instance.Owner.Project(project)
	if err != nil {
		t.Fatal(err)
	}
	page, err := http.Get("http://" + instance.address + "/projects/" + admitted.ID())
	if err != nil {
		t.Fatal(err)
	}
	_ = page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("new project page: HTTP %d", page.StatusCode)
	}
	if runs := instanceControlRuns(t, instance, project); len(runs) != 0 {
		t.Fatalf("new project control rows: %+v", runs)
	}
}

func TestInitialProjectsAreAdmittedBeforeWebServes(t *testing.T) {
	base := t.TempDir()
	projects := []string{filepath.Join(base, "a"), filepath.Join(base, "b")}
	for _, project := range projects {
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	i, err := NewInstance(ctx, filepath.Join(base, "host"), projects, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + i.address + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), projects[0]) || !strings.Contains(string(page), projects[1]) {
		t.Fatalf("first page: status %d, missing initial projects", response.StatusCode)
	}
}

func TestBrowserControlUsesReferringProject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	i, err := NewInstance(ctx, filepath.Join(base, "instance"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	aPath, bPath := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, path := range []string{aPath, bPath} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := i.Owner.AdmitProject(aPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := i.Owner.AdmitProject(bPath)
	if err != nil {
		t.Fatal(err)
	}
	controller := &controlledRun{}
	release := a.Runs().Hook("control-probe", controller)
	defer release()
	remoteID := ""
	for _, remote := range generated.Remotes() {
		if strings.HasSuffix(remote.ID(), "/cancelRun") {
			remoteID = remote.ID()
			break
		}
	}
	if remoteID == "" {
		t.Fatal("cancelRun remote missing")
	}
	encoded, err := devalue.Stringify(map[string]any{"run": "control-probe"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"payload": base64.RawURLEncoding.EncodeToString([]byte(encoded))})
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + i.address
	post := func(referrer string) (int, string) {
		request, err := http.NewRequest(http.MethodPost, origin+"/_app/remote/"+remoteID, strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Origin", origin)
		request.Header.Set("Content-Type", "application/json")
		if referrer != "" {
			request.Header.Set("Referer", origin+referrer)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		result, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(result)
	}
	if status, body := post("/projects/" + b.ID() + "/runs/control-probe"); status != http.StatusOK || !strings.Contains(body, "not in progress") || controller.cause != nil {
		t.Fatalf("B controlled A: status %d, body %s, cause %v", status, body, controller.cause)
	}
	if status, body := post(""); status != http.StatusNotFound || controller.cause != nil {
		t.Fatalf("unqualified control reached A: status %d, body %s, cause %v", status, body, controller.cause)
	}
	if status, body := post("/projects/" + a.ID() + "/runs/control-probe"); status != http.StatusOK || controller.cause == nil {
		t.Fatalf("A control: status %d, body %s, cause %v", status, body, controller.cause)
	}
}

func TestInstanceCreatesConversationsInEachProject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	i, err := NewInstance(ctx, filepath.Join(t.TempDir(), "instance"), nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		repo := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		gitForConversationPage(t, repo, "init", "-q")
		gitForConversationPage(t, repo, "config", "user.email", "gimbal-test@example.invalid")
		gitForConversationPage(t, repo, "config", "user.name", "Gimbal Test")
		gitForConversationPage(t, repo, "commit", "-qm", "initial", "--allow-empty")
		p, err := i.Owner.AdmitProject(repo)
		if err != nil {
			t.Fatal(err)
		}
		item, err := p.Conversations().Create(ctx, conversation.NewConversation{Title: name, Provider: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(item.Worktree, filepath.Join(p.Dir(), "conversation-worktrees")) {
			t.Fatalf("%s worktree in %s", name, item.Worktree)
		}
		if _, err := os.Stat(filepath.Join(p.Dir(), "conversations", item.ID+".json")); err != nil {
			t.Fatal(err)
		}
		if len(p.Conversations().List()) != 1 {
			t.Fatalf("%s conversation list: %+v", name, p.Conversations().List())
		}
	}
}

func instanceSnapshotStatus(t *testing.T, i *Instance, project, runID string) int {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(i.dir, "control", "*.json"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("control discovery: %v, %v", entries, err)
	}
	var discovery controlDiscovery
	encoded, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &discovery); err != nil {
		t.Fatal(err)
	}
	client := controlClient(discovery.Socket)
	defer client.CloseIdleConnections()
	req, err := http.NewRequest(http.MethodGet, "http://control/api/runs/"+runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Gimbal-Project", project)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}

func instanceControlRuns(t *testing.T, i *Instance, project string) []observation.RunRow {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(i.dir, "control", "*.json"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("control discovery: %v, %v", entries, err)
	}
	var discovery controlDiscovery
	encoded, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &discovery); err != nil {
		t.Fatal(err)
	}
	client := controlClient(discovery.Socket)
	defer client.CloseIdleConnections()
	req, err := http.NewRequest(http.MethodGet, "http://control/control/runs", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Gimbal-Project", project)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var rows []observation.RunRow
	if err := json.NewDecoder(response.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("control status %d", response.StatusCode)
	}
	return rows
}

func instanceControlStatus(t *testing.T, i *Instance, project, runID string) int {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(i.dir, "control", "*.json"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("control discovery: %v, %v", entries, err)
	}
	var discovery controlDiscovery
	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &discovery); err != nil {
		t.Fatal(err)
	}
	client := controlClient(discovery.Socket)
	defer client.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodPost, "http://control/control/steer", strings.NewReader(`{"run":"`+runID+`","session":"missing","message":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if project != "" {
		request.Header.Set("X-Gimbal-Project", project)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}
