package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/observation"
	generated "github.com/tylergannon/gimble/internal/skgo"
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
		if err := os.MkdirAll(filepath.Join(project, ".gimble", "conversations"), 0o755); err != nil {
			t.Fatal(err)
		}
		item := conversation.Conversation{ID: filepath.Base(project), Title: "Selected conversation " + filepath.Base(project)}
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, ".gimble", "conversations", item.ID+".json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	i, err := NewInstance(ctx, filepath.Join(base, "instance-one"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	a, err := i.AdmitProject(projectA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := i.AdmitProject(projectB)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := i.AdmitProject(projectA); err != nil || again != a {
		t.Fatalf("readmit project: %p, %v", again, err)
	}
	alias := filepath.Join(base, "alias-a")
	if err := os.Symlink(projectA, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alias, filepath.Join(base, "b", "..", "a"), projectA} {
		if again, err := i.AdmitProject(path); err != nil || again != a {
			t.Fatalf("alias %s admitted separate project: %p, %v", path, again, err)
		}
	}
	if a.registry == b.registry || a.runs == b.runs || a.conversations == b.conversations {
		t.Fatal("projects share live state")
	}
	if a.instance != b.instance {
		t.Fatal("admission made a second owner or listener")
	}
	for _, p := range []*Runtime{a, b} {
		items := p.conversations.List()
		if len(items) != 1 || items[0].ID != filepath.Base(p.project) {
			t.Fatalf("conversation state for %s: %+v", p.dir, items)
		}
	}
	if entries, err := filepath.Glob(filepath.Join(i.dir, "control", "*.json")); err != nil || len(entries) != 1 {
		t.Fatalf("instance control discovery: %v, %v", entries, err)
	}
	for _, project := range []string{projectA, projectB} {
		if entries, err := filepath.Glob(filepath.Join(project, ".gimble", "control", "*.json")); err != nil || len(entries) != 1 {
			t.Fatalf("project discovery: %v, %v", entries, err)
		}
	}
	j, err := NewInstance(ctx, filepath.Join(base, "instance-two"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	c, err := j.AdmitProject(projectC)
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
	finish := map[*Runtime]chan struct{}{a: make(chan struct{}), b: make(chan struct{}), c: make(chan struct{})}
	var wg sync.WaitGroup
	for _, p := range []*Runtime{a, b, c} {
		wg.Go(func() {
			if err := p.Run(ctx, "held", nil, func(context.Context) error {
				started <- struct{}{}
				<-finish[p]
				return nil
			}); err != nil {
				t.Errorf("run in %s: %v", p.dir, err)
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
	for _, p := range []*Runtime{a, b, c} {
		rows, err := p.controlRuns()
		if err != nil || len(rows) != 1 || rows[0].Status != observation.StatusRunning {
			t.Fatalf("live runs in %s: %+v, %v", p.dir, rows, err)
		}
		if _, err := os.Stat(filepath.Join(p.dir, "runs", rows[0].ID, "run.jsonl")); err != nil {
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
	stream, err := (&http.Client{Timeout: 2 * time.Second}).Get("http://" + i.address + "/projects/" + a.id + "/api/runs/" + aID + "/events")
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
		{"/projects/" + a.id, aID, bID, http.StatusOK},
		{"/projects/" + b.id, bID, aID, http.StatusOK},
		{"/projects/" + a.id + "/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.id + "/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.id + "/api/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.id + "/api/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.id + "/conversations/a", "Selected conversation a", "Selected conversation b", http.StatusOK},
		{"/projects/" + b.id + "/conversations/b", "Selected conversation b", "Selected conversation a", http.StatusOK},
		{"/projects/" + b.id + "/api/runs/" + aID + "/events", "", "", http.StatusNotFound},
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
	releaseProbe := a.runs.Hook("control-probe", &controlledRun{})
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
		rows, _ := a.controlRuns()
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
	reopened, err := NewInstance(t.Context(), filepath.Join(base, "restart"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{projectA, projectB} {
		if _, err := reopened.AdmitProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ path, id string }{
		{"/projects/" + a.id + "/runs/" + aID, aID},
		{"/projects/" + b.id + "/runs/" + bID, bID},
		{"/projects/" + a.id + "/conversations/a", "a"},
		{"/projects/" + b.id + "/conversations/b", "b"},
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

func TestBrowserControlUsesReferringProject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	i, err := NewInstance(ctx, filepath.Join(base, "instance"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	aPath, bPath := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, path := range []string{aPath, bPath} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := i.AdmitProject(aPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := i.AdmitProject(bPath)
	if err != nil {
		t.Fatal(err)
	}
	controller := &controlledRun{}
	release := a.runs.Hook("control-probe", controller)
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
	if status, body := post("/projects/" + b.id + "/runs/control-probe"); status != http.StatusOK || !strings.Contains(body, "not in progress") || controller.cause != nil {
		t.Fatalf("B controlled A: status %d, body %s, cause %v", status, body, controller.cause)
	}
	if status, body := post(""); status != http.StatusNotFound || controller.cause != nil {
		t.Fatalf("unqualified control reached A: status %d, body %s, cause %v", status, body, controller.cause)
	}
	if status, body := post("/projects/" + a.id + "/runs/control-probe"); status != http.StatusOK || controller.cause == nil {
		t.Fatalf("A control: status %d, body %s, cause %v", status, body, controller.cause)
	}
}

func TestInstanceCreatesConversationsInEachProject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	i, err := NewInstance(ctx, filepath.Join(t.TempDir(), "instance"), WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		repo := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		gitForConversationPage(t, repo, "init", "-q")
		gitForConversationPage(t, repo, "config", "user.email", "gimble-test@example.invalid")
		gitForConversationPage(t, repo, "config", "user.name", "Gimble Test")
		gitForConversationPage(t, repo, "commit", "-qm", "initial", "--allow-empty")
		p, err := i.AdmitProject(repo)
		if err != nil {
			t.Fatal(err)
		}
		item, err := p.conversations.Create(ctx, conversation.NewConversation{Title: name, Provider: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(item.Worktree, filepath.Join(p.dir, "conversation-worktrees")) {
			t.Fatalf("%s worktree in %s", name, item.Worktree)
		}
		if _, err := os.Stat(filepath.Join(p.dir, "conversations", item.ID+".json")); err != nil {
			t.Fatal(err)
		}
		if len(p.conversations.List()) != 1 {
			t.Fatalf("%s conversation list: %+v", name, p.conversations.List())
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
	req.Header.Set("X-Gimble-Project", project)
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
	req.Header.Set("X-Gimble-Project", project)
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
		request.Header.Set("X-Gimble-Project", project)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}
