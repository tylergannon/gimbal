package runview

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	"testing"
)

func TestListUsesCurrentTitleOnlyForTheAssociatedBackingRun(t *testing.T) {
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	for _, id := range []string{"backing", "child", "unrelated"} {
		dir := filepath.Join(project, "runs", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		store, err := observation.Open(registry, id, "conversation", dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
	}
	if err := os.MkdirAll(filepath.Join(project, "conversations"), 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(conversation.Conversation{ID: "session", Title: "Original title", WorkflowRun: "backing", Status: "ended", Runs: []conversation.Run{{ID: "child", Workflow: "conversation"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "conversations", "session.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := conversation.New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return nil, errors.New("no harness should start") }, "", "", project, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	ctx := conversation.WithManager(host.WithProjectDir(observation.WithRegistry(context.Background(), registry), project), manager)
	assertTitles := func(want string) {
		t.Helper()
		data, err := List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(data.Items) != 3 {
			t.Fatalf("got %d runs, want three distinct runs", len(data.Items))
		}
		for _, item := range data.Items {
			if item.Run.Name != "conversation" {
				t.Fatalf("changed stored workflow name: %+v", item)
			}
			title := ""
			if item.Run.ID == "backing" {
				title = want
			}
			if item.SessionTitle != title {
				t.Errorf("run %s title = %q, want %q", item.Run.ID, item.SessionTitle, title)
			}
		}
	}
	assertTitles("Original title")
	if _, err := manager.Rename("session", "Pi macOS startup notes"); err != nil {
		t.Fatal(err)
	}
	assertTitles("Pi macOS startup notes")
}

func TestSmallNonzeroCostNeverLooksFree(t *testing.T) {
	for _, cost := range []float64{.009, .0001, .000000001} {
		snapshot := observation.RunSnapshot{Totals: observation.Totals{Scopes: map[string]observation.Total{"": {ByModel: map[string]observation.Usage{"unknown-model": {StatedCost: cost}}}}}}
		if got := formatCost(snapshot); got != "<$0.01" {
			t.Errorf("cost %g renders %q", cost, got)
		}
	}
}

func TestEndedRunDoesNotAskForAnUnanswerableQuestion(t *testing.T) {
	snapshot := observation.RunSnapshot{Run: observation.RunRow{Status: observation.StatusCancelled}, Interactions: map[string]observation.InteractiveRow{"question": {Status: observation.InteractiveStatusPending, Question: "Which color?"}}}
	if got := pendingInteractions(snapshot); len(got) != 0 {
		t.Fatalf("ended run has waiting questions: %+v", got)
	}
	if got := summarizeRun(snapshot, pendingInteractions(snapshot)); got != "Cancelled" {
		t.Fatalf("ended summary = %q", got)
	}
}
