package web

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/gimble/internal/skgo/client"
	forms "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/gimble/internal/workflows/validateproduct"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/skgo"
)

func TestGeneratedStartsOnProductionListeners(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	instanceDir := filepath.Join(base, "instance")
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	instance, err := NewInstance(ctx, instanceDir, nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); instance.Wait() })
	if len(instance.Owner.Projects()) != 0 {
		t.Fatal("instance admitted a project before a start")
	}
	selected, err := selectedInstance(ctx, instanceDir, project)
	if err != nil {
		t.Fatalf("discover instance for unknown project: %v", err)
	}
	if len(instance.Owner.Projects()) != 0 {
		t.Fatal("discovery admitted the project")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", selected.socket)
	}}
	defer transport.CloseIdleConnections()
	uds := client.Client{BaseURL: "http://gimble", HTTPClient: &http.Client{Transport: transport}}
	browser := client.Client{BaseURL: "http://" + instance.address}
	for _, caller := range []struct {
		name string
		form client.Client
	}{{"control", uds}, {"browser", browser}} {
		for _, tc := range []struct {
			name string
			call func(client.Client) error
		}{
			{"review", func(c client.Client) error {
				_, err := c.StartReview(ctx, forms.StartReviewInput{Goal: "review"})
				return err
			}},
			{"implement", func(c client.Client) error {
				_, err := c.StartImplement(ctx, forms.StartImplementInput{OutcomesFile: "outcomes.json", MaxTasksPerOutcome: 1})
				return err
			}},
			{"research-document", func(c client.Client) error {
				_, err := c.StartResearchDocument(ctx, forms.StartResearchDocumentInput{Goal: "goal", ResearchDir: "research", Output: "out.md", TokenBudget: 100})
				return err
			}},
			{"pyramid-summary", func(c client.Client) error {
				_, err := c.StartPyramidSummary(ctx, forms.StartPyramidSummaryInput{Goal: "goal", SemanticIndex: "index", LargestDocument: "doc", OutputDir: "out"})
				return err
			}},
			{"validate-product", func(c client.Client) error {
				_, err := c.StartValidateProduct(ctx, forms.StartValidateProductInput{SuiteFile: "missing-suite.json"})
				return err
			}},
		} {
			t.Run(caller.name+"/"+tc.name, func(t *testing.T) {
				var invalid *skgo.Invalid
				if err := tc.call(caller.form); !errors.As(err, &invalid) || len(invalid.Issues) != 1 || invalid.Issues[0].Field != "project_dir" {
					t.Fatalf("generated Form error = %v, want project_dir issue", err)
				}
			})
		}
	}
	if len(instance.Owner.Projects()) != 0 {
		t.Fatal("rejected Forms admitted a project")
	}

	// The form client supplies no Referer or project header. Concurrent
	// first-use requests admit one canonical owner from the payload alone.
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(project, alias); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan forms.StartAccepted, 8)
	errors := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			accepted, err := browser.StartValidateProduct(ctx, forms.StartValidateProductInput{ProjectDir: alias, SuiteFile: "missing-suite.json"})
			results <- accepted
			errors <- err
		})
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("browser listener start: %v", err)
		}
	}
	var first forms.StartAccepted
	for accepted := range results {
		if first.RunID == "" {
			first = accepted
		}
		if accepted.ProjectID != first.ProjectID || accepted.RunID == "" || instanceSnapshotStatus(t, instance, project, accepted.RunID) != http.StatusOK {
			t.Fatalf("browser admission was not published by canonical owner: %+v", accepted)
		}
	}
	if len(instance.Owner.Projects()) != 1 {
		t.Fatalf("alias/concurrent starts made %d owners", len(instance.Owner.Projects()))
	}
	fromControl, err := uds.StartValidateProduct(ctx, forms.StartValidateProductInput{ProjectDir: project, SuiteFile: "missing-suite.json"})
	if err != nil || fromControl.ProjectID != first.ProjectID || fromControl.RunID == "" || instanceSnapshotStatus(t, instance, project, fromControl.RunID) != http.StatusOK {
		t.Fatalf("control admission through same identity = %+v, %v", fromControl, err)
	}
	other := filepath.Join(base, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	second, err := browser.StartValidateProduct(ctx, forms.StartValidateProductInput{ProjectDir: other, SuiteFile: "missing-suite.json"})
	if err != nil || second.ProjectID == first.ProjectID || second.RunID == "" {
		t.Fatalf("browser first-use admission = %+v, %v", second, err)
	}
	if status := instanceSnapshotStatus(t, instance, other, first.RunID); status != http.StatusNotFound {
		t.Fatalf("foreign project read status = %d", status)
	}
	if status := instanceSnapshotStatus(t, instance, project, second.RunID); status != http.StatusNotFound {
		t.Fatalf("reverse foreign project read status = %d", status)
	}
	for path := range instance.startPaths {
		request, err := http.NewRequest(http.MethodPost, "http://"+instance.address+path, strings.NewReader("invalid"))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Origin", "http://foreign.invalid")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("cross-origin start status = %d", response.StatusCode)
		}
		break
	}
	if len(instance.Owner.Projects()) != 2 {
		t.Fatal("cross-origin request changed project admission")
	}
}

func TestGeneratedStartOnHeadlessControlListener(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	project := filepath.Join(base, "unknown")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(base, "instance")
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); instance.Wait() })
	selected, err := selectedInstance(ctx, instanceDir, project)
	if err != nil {
		t.Fatalf("discover headless instance for unknown project: %v", err)
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", selected.socket)
	}}
	defer transport.CloseIdleConnections()
	formsClient := client.Client{BaseURL: "http://gimble", HTTPClient: &http.Client{Transport: transport}}
	accepted, err := formsClient.StartValidateProduct(ctx, forms.StartValidateProductInput{ProjectDir: project, SuiteFile: "missing-suite.json"})
	if err != nil || accepted.RunID == "" || accepted.ProjectID == "" {
		t.Fatalf("headless generated start = %+v, %v", accepted, err)
	}
	if status := instanceSnapshotStatus(t, instance, project, accepted.RunID); status != http.StatusOK {
		t.Fatalf("accepted headless run was not published: %d", status)
	}
}

func TestGeneratedStartOutlivesClientRequest(t *testing.T) {
	if instanceDir := os.Getenv("GIMBLE_TEST_START_INSTANCE"); instanceDir != "" {
		project := os.Getenv("GIMBLE_TEST_START_PROJECT")
		selected, err := selectedInstance(context.Background(), instanceDir, project)
		if err != nil {
			t.Fatal(err)
		}
		transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", selected.socket)
		}}
		defer transport.CloseIdleConnections()
		formsClient := client.Client{BaseURL: "http://gimble", HTTPClient: &http.Client{Transport: transport}}
		cheap := polytype.Optional[string]{Present: true, Value: "gpt-5.6-luna"}
		accepted, err := formsClient.StartValidateProduct(context.Background(), forms.StartValidateProductInput{
			ProjectDir: project, SuiteFile: "suite.json",
			RoleProductOperation: cheap, RoleProductVisualReview: cheap, RoleProductTriage: cheap,
		})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = os.Stdout.WriteString(accepted.RunID + "\n")
		return
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "assignment.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	suite := validateproduct.Suite{
		Product: "test", OutputDir: "output", PlaywrightCLI: "true", IssueRepo: "example/example", Timeout: "10s",
		Workloads: []validateproduct.Workload{{Name: "waiting", AssignmentFile: "assignment.txt", Workdir: project, URL: "http://localhost", Ready: "false"}},
	}
	data, err := json.Marshal(suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "suite.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(base, "instance")
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); instance.Wait() })
	launch := exec.Command(os.Args[0], "-test.run=^TestGeneratedStartOutlivesClientRequest$")
	launch.Env = append(os.Environ(), "GIMBLE_TEST_START_INSTANCE="+instanceDir, "GIMBLE_TEST_START_PROJECT="+project)
	output, err := launch.CombinedOutput()
	if err != nil {
		t.Fatalf("generated client process: %v: %s", err, output)
	}
	runID := strings.Fields(string(output))[0]
	p, err := instance.Owner.Project(project)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	snapshot, err := p.Registry().Snapshot(runID)
	if err != nil || snapshot.Run.Status != observation.StatusRunning {
		t.Fatalf("accepted run after client request ended: %+v, %v", snapshot.Run, err)
	}
	other := filepath.Join(base, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := instance.Owner.AdmitProject(other); err != nil {
		t.Fatal(err)
	}
	if status := instanceControlStatus(t, instance, other, runID); status != http.StatusNotFound {
		t.Fatalf("foreign control status = %d", status)
	}
	snapshot, err = p.Registry().Snapshot(runID)
	if err != nil || snapshot.Run.Status != observation.StatusRunning {
		t.Fatalf("foreign control affected accepted run: %+v, %v", snapshot.Run, err)
	}
}
