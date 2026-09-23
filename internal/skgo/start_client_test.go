package skgo_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimble/internal/host"
	generated "github.com/tylergannon/gimble/internal/skgo"
	"github.com/tylergannon/gimble/internal/skgo/client"
	forms "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/skgo"
)

func TestGeneratedStartClientsReachConcreteHandlers(t *testing.T) {
	ctx := t.Context()
	owner := host.New(ctx, filepath.Join(t.TempDir(), "instance"))
	t.Cleanup(owner.Close)
	remotes, err := skgo.NewRemotes(skgo.RemoteConfig{}, generated.Remotes()...)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remotes.ServeHTTP(w, r.WithContext(host.WithOwner(r.Context(), owner)))
	}))
	defer server.Close()
	formsClient := client.Client{BaseURL: server.URL}
	project := t.TempDir()

	// Each generated method reaches its actual generated handler. The missing
	// project must be rejected before the host admits anything or starts a run.
	cases := []struct {
		name string
		call func() error
	}{
		{"review", func() error {
			_, err := formsClient.StartReview(ctx, forms.StartReviewInput{Goal: "review this"})
			return err
		}},
		{"implement", func() error {
			_, err := formsClient.StartImplement(ctx, forms.StartImplementInput{OutcomesFile: "outcomes.json", MaxTasksPerOutcome: 1})
			return err
		}},
		{"research-document", func() error {
			_, err := formsClient.StartResearchDocument(ctx, forms.StartResearchDocumentInput{Goal: "goal", ResearchDir: "research", Output: "out.md", TokenBudget: 100})
			return err
		}},
		{"pyramid-summary", func() error {
			_, err := formsClient.StartPyramidSummary(ctx, forms.StartPyramidSummaryInput{Goal: "goal", SemanticIndex: "index", LargestDocument: "doc", OutputDir: "out"})
			return err
		}},
		{"validate-product", func() error {
			_, err := formsClient.StartValidateProduct(ctx, forms.StartValidateProductInput{SuiteFile: "suite.json"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var invalid *skgo.Invalid
			if err := tc.call(); !errors.As(err, &invalid) || len(invalid.Issues) != 1 || invalid.Issues[0].Field != "project_dir" {
				t.Fatalf("error = %v, want project_dir issue", err)
			}
			if len(owner.Projects()) != 0 {
				t.Fatal("rejected request admitted a project")
			}
		})
	}

	// An explicitly empty role is distinct from an omitted override: the
	// former is rejected while the latter takes the shared default.
	_, err = formsClient.StartReview(ctx, forms.StartReviewInput{ProjectDir: project, Goal: "review", RoleCodeReview: polytype.Optional[string]{Present: true, Value: ""}})
	var invalid *skgo.Invalid
	if !errors.As(err, &invalid) || invalid.Issues[0].Field != "role_code_review" {
		t.Fatalf("empty role error = %v", err)
	}
	if len(owner.Projects()) != 0 {
		t.Fatal("invalid role admitted a project")
	}
	_, err = formsClient.StartReview(ctx, forms.StartReviewInput{ProjectDir: project, Goal: "review", RoleCodeReview: polytype.Optional[string]{Present: true, Value: "no-such-model"}})
	if !errors.As(err, &invalid) || invalid.Issues[0].Field != "role_code_review" {
		t.Fatalf("unknown role model error = %v", err)
	}
	if len(owner.Projects()) != 0 {
		t.Fatal("unknown role model admitted a project")
	}

	_, err = formsClient.StartReview(ctx, forms.StartReviewInput{ProjectDir: project, Goal: ""})
	if !errors.As(err, &invalid) || invalid.Issues[0].Field != "goal" {
		t.Fatalf("missing goal error = %v", err)
	}
	if len(owner.Projects()) != 0 {
		t.Fatal("missing goal admitted a project")
	}

	// ValidateProduct fails quickly in its body on the nonexistent suite, so
	// acceptance demonstrates run publication without spending on a model.
	accepted, err := formsClient.StartValidateProduct(ctx, forms.StartValidateProductInput{ProjectDir: project, SuiteFile: "missing-suite.json"})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.RunID == "" || accepted.ProjectID == "" {
		t.Fatalf("admission = %+v", accepted)
	}
	admitted, err := owner.Project(project)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.ID() != accepted.ProjectID {
		t.Fatalf("project ID = %q, response = %q", admitted.ID(), accepted.ProjectID)
	}
}

func TestGeneratedResearchClientPreservesOptionalZero(t *testing.T) {
	var received []forms.StartResearchDocumentInput
	// Use the generated remote identity and result codec while observing the
	// decoded argument. Admission behavior is covered against the real handler
	// above; this checks the two presence states on the actual HTTP transport.
	probe := skgo.NewRemote(skgo.RemoteSpec{
		Kind: skgo.KindForm, Module: "src/routes/researchdocument_start.remote.ts", Name: "startResearchDocument",
		Fn: func(context.Context, forms.StartResearchDocumentInput) (forms.StartAccepted, error) {
			return forms.StartAccepted{}, nil
		},
		Call: func(_ context.Context, call skgo.Call) (any, error) {
			var input forms.StartResearchDocumentInput
			if err := skgo.DecodeForm(call.Arg, &input); err != nil {
				return nil, err
			}
			received = append(received, input)
			return generated.EncodeRoot6(forms.StartAccepted{ProjectID: "project", RunID: "run"})
		},
	})
	remotes, err := skgo.NewRemotes(skgo.RemoteConfig{}, probe)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(remotes.Intercept(http.NotFoundHandler()))
	defer server.Close()
	formsClient := client.Client{BaseURL: server.URL}
	input := forms.StartResearchDocumentInput{Goal: "goal", ResearchDir: "research", Output: "out", TokenBudget: 100}
	if _, err := formsClient.StartResearchDocument(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.MinSourcesPerTopic = polytype.Optional[int]{Present: true, Value: 0}
	if _, err := formsClient.StartResearchDocument(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].MinSourcesPerTopic.Present || !received[1].MinSourcesPerTopic.Present || received[1].MinSourcesPerTopic.Value != 0 {
		t.Fatalf("optional fields after HTTP = %+v", received)
	}
}
