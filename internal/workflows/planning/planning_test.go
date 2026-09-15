package planning

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
)

type fakeAdapter struct {
	mu                  sync.Mutex
	name                string
	failDraft           bool
	question            bool
	questions           []string
	prompts             []string
	turns               int
	schemaTurns         int
	blankFinalDecisions bool
}

type unreadablePlanningReader struct{}

func (unreadablePlanningReader) Read([]byte) (int, error) { panic("reader was used") }

func (f *fakeAdapter) CreateSession(context.Context, string, string) (string, error) {
	return f.name, nil
}
func (f *fakeAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, prompt)
	f.turns++
	f.mu.Unlock()
	if f.failDraft && len(schema) == 0 && strings.Contains(prompt, "draft a concise") {
		return gimble.TurnResult{}, errors.New("draft failed")
	}
	if len(schema) != 0 {
		f.mu.Lock()
		f.schemaTurns++
		schemaTurn := f.schemaTurns
		f.mu.Unlock()
		if len(f.questions) > 0 && schemaTurn <= len(f.questions) {
			return gimble.TurnResult{Output: json.RawMessage(`{"question":"` + f.questions[schemaTurn-1] + `","plan":"","overview":"draft differences and consensus","decisions":"pending interview"}`)}, nil
		}
		if f.question && schemaTurn == 1 {
			return gimble.TurnResult{Output: json.RawMessage(`{"question":"What matters most?","plan":"","overview":"draft differences and consensus","decisions":"pending interview"}`)}, nil
		}
		if f.blankFinalDecisions {
			return gimble.TurnResult{Output: json.RawMessage(`{"question":"","plan":"# Final plan","overview":"comparison","decisions":""}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"question":"","plan":"# Final plan\n","overview":"draft differences and consensus","decisions":"accepted consensus; rejected scope expansion"}`)}, nil
	}
	if strings.Contains(prompt, "Orient to the repository") {
		return gimble.TurnResult{Output: mustJSON("orientation context")}, nil
	}
	return gimble.TurnResult{Output: mustJSON(f.name + " output")}, nil
}
func mustJSON(value string) json.RawMessage                              { raw, _ := json.Marshal(value); return raw }
func (*fakeAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fakeAdapter) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*fakeAdapter) Close(context.Context, string) error                 { return nil }

func runInjected(t *testing.T, in Input, lanes []lane, reader *bufio.Reader, out *strings.Builder) error {
	t.Helper()
	if err := os.MkdirAll(in.OutputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return gimble.Run(gimble.Project(context.Background(), in.Repo), "planning-test", func(ctx context.Context) error {
		return plan(ctx, in, promptBrief(in), reader, out, lanes)
	})
}

func TestInjectedPlanRunsThreeDraftsThreeCrossCritiquesAndSynthesis(t *testing.T) {
	repo, output := t.TempDir(), filepath.Join(t.TempDir(), "output")
	fs := []*fakeAdapter{{name: "codex"}, {name: "claude"}, {name: "gemini"}}
	lanes := []lane{{name: "codex", model: "m", adapter: fs[0]}, {name: "claude", model: "m", adapter: fs[1]}, {name: "gemini", model: "m", adapter: fs[2]}}
	var out strings.Builder
	if err := runInjected(t, Input{Repo: repo, Goal: "goal", Checks: []string{"go test ./...", "go vet ./..."}, OutputDir: output}, lanes, nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"draft-codex.md", "draft-claude.md", "draft-gemini.md", "critique-codex.md", "critique-claude.md", "critique-gemini.md", "plan.md"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	intent, err := os.ReadFile(filepath.Join(output, "intent.md"))
	if err != nil || !strings.Contains(string(intent), "go test ./...") || !strings.Contains(string(intent), "go vet ./...") {
		t.Fatalf("intent lost fixed checks: %s, %v", intent, err)
	}
	merge, err := os.ReadFile(filepath.Join(output, "merge-notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merge), "draft differences and consensus") || !strings.Contains(string(merge), "rejected scope expansion") {
		t.Fatalf("merge notes lack actual synthesis: %s", merge)
	}
	orientation, err := os.ReadFile(filepath.Join(output, "orientation.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(orientation), "orientation context") {
		t.Fatalf("orientation artifact lacks returned evidence: %s", orientation)
	}
	if len(fs[0].prompts) != 3 || len(fs[1].prompts) != 2 || len(fs[2].prompts) != 3 {
		t.Fatalf("turns = %d,%d,%d; want 3,2,3", len(fs[0].prompts), len(fs[1].prompts), len(fs[2].prompts))
	}
	if !strings.Contains(fs[0].prompts[2], "claude output") || !strings.Contains(fs[0].prompts[2], "gemini output") || strings.Contains(fs[0].prompts[2], "codex output") {
		t.Fatalf("codex critique inputs are wrong: %s", fs[0].prompts[2])
	}
	if !strings.Contains(fs[1].prompts[1], "codex output") || !strings.Contains(fs[1].prompts[1], "gemini output") || strings.Contains(fs[1].prompts[1], "claude output") {
		t.Fatalf("claude critique inputs are wrong: %s", fs[1].prompts[1])
	}
	if !strings.Contains(fs[2].prompts[1], "codex output") || !strings.Contains(fs[2].prompts[1], "claude output") || strings.Contains(fs[2].prompts[1], "gemini output") {
		t.Fatalf("gemini critique inputs are wrong: %s", fs[2].prompts[1])
	}
	for i, adapter := range fs {
		for _, prompt := range adapter.prompts {
			if !strings.Contains(prompt, "go test ./...") || !strings.Contains(prompt, "go vet ./...") {
				t.Fatalf("lane %d prompt omitted exact checks: %s", i, prompt)
			}
		}
	}
}

func TestInjectedPlanQuestionHandoffPreservesExactAnswer(t *testing.T) {
	repo, output := t.TempDir(), filepath.Join(t.TempDir(), "output")
	fs := []*fakeAdapter{{name: "codex"}, {name: "claude"}, {name: "gemini", question: true}}
	lanes := []lane{{name: "codex", model: "m", adapter: fs[0]}, {name: "claude", model: "m", adapter: fs[1]}, {name: "gemini", model: "m", adapter: fs[2]}}
	var out strings.Builder
	if err := runInjected(t, Input{Repo: repo, Goal: "goal", OutputDir: output}, lanes, bufio.NewReader(strings.NewReader(" exact answer\n")), &out); err != nil {
		t.Fatal(err)
	}
	answers, err := os.ReadFile(filepath.Join(output, "answers.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(answers), "What matters most?") || !strings.Contains(string(answers), " exact answer\n") {
		t.Fatalf("answer artifact lost exact text: %q", answers)
	}
}

func TestInjectedPlanHandlesMultipleClarificationsAndSemanticContext(t *testing.T) {
	repo, output := t.TempDir(), filepath.Join(t.TempDir(), "output")
	index := filepath.Join(repo, "index.md")
	cache := filepath.Join(repo, "cache")
	if err := os.WriteFile(index, []byte("route"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	fs := []*fakeAdapter{{name: "codex"}, {name: "claude"}, {name: "gemini", questions: []string{"first?", "second?"}}}
	lanes := []lane{{name: "codex", model: "m", adapter: fs[0]}, {name: "claude", model: "m", adapter: fs[1]}, {name: "gemini", model: "m", adapter: fs[2]}}
	var out strings.Builder
	if err := os.MkdirAll(filepath.Join(repo, ".gimble"), 0o755); err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(map[string]string{"source": cache, "entrypoint": index})
	if err := os.WriteFile(filepath.Join(repo, ".gimble", "semantic-index.json"), config, 0o644); err != nil {
		t.Fatal(err)
	}
	in := Input{Repo: repo, Goal: "goal", OutputDir: output}
	if err := runInjected(t, in, lanes, bufio.NewReader(strings.NewReader("answer one\nanswer two\n")), &out); err != nil {
		t.Fatal(err)
	}
	answers, err := os.ReadFile(filepath.Join(output, "answers.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"first?", "answer one\n", "second?", "answer two\n"} {
		if !strings.Contains(string(answers), value) {
			t.Fatalf("answers missing %q: %s", value, answers)
		}
	}
	if strings.Index(out.String(), "draft differences and consensus") > strings.Index(out.String(), "first?") {
		t.Fatal("interview began before showing draft tradeoffs")
	}
	if !strings.Contains(fs[2].prompts[3], "another essential question") || !strings.Contains(fs[2].prompts[4], "answer one") || !strings.Contains(fs[2].prompts[4], "answer two") {
		t.Fatal("follow-up cannot continue informed interview or lost prior answers")
	}
	for _, adapter := range fs {
		for _, prompt := range adapter.prompts {
			if !strings.Contains(prompt, index) || !strings.Contains(prompt, cache) {
				t.Fatalf("prompt omitted semantic context: %s", prompt)
			}
		}
	}
}

func TestInjectedPlanFailedDraftPreventsSynthesis(t *testing.T) {
	repo, output := t.TempDir(), filepath.Join(t.TempDir(), "output")
	fs := []*fakeAdapter{{name: "codex", failDraft: true}, {name: "claude"}, {name: "gemini"}}
	lanes := []lane{{name: "codex", model: "m", adapter: fs[0]}, {name: "claude", model: "m", adapter: fs[1]}, {name: "gemini", model: "m", adapter: fs[2]}}
	var out strings.Builder
	if err := runInjected(t, Input{Repo: repo, Goal: "goal", OutputDir: output}, lanes, nil, &out); err == nil {
		t.Fatal("failed draft was accepted")
	}
	if _, err := os.Stat(filepath.Join(output, "plan.md")); !os.IsNotExist(err) {
		t.Fatalf("synthesis ran after failed draft: %v", err)
	}
}

func TestPlanDryRunWritesThreeDraftsThreeCritiquesAndFinalPlan(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	path, err := Plan(context.Background(), Input{Repo: dir, Goal: "make a plan", OutputDir: filepath.Join(dir, "plan"), DryRun: true}, nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("path = %q, want absolute", path)
	}
	for _, name := range []string{"draft-codex.md", "draft-claude.md", "draft-gemini.md", "critique-codex.md", "critique-claude.md", "critique-gemini.md", "plan.md"} {
		if _, err := os.Stat(filepath.Join(dir, "plan", name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if !strings.Contains(out.String(), "no model was called") {
		t.Fatalf("dry run output lacks marker: %s", out.String())
	}
}

func TestPlanRejectsMissingContextAndCancelledRun(t *testing.T) {
	dir := t.TempDir()
	base := Input{Repo: dir, Goal: "goal", OutputDir: filepath.Join(dir, "plan")}
	if _, err := Plan(context.Background(), Input{Repo: dir, Goal: "goal", OutputDir: base.OutputDir, ContextFiles: []string{"missing"}}, bufio.NewReader(strings.NewReader("")), &strings.Builder{}); err == nil {
		t.Fatal("missing context was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Plan(ctx, base, nil, &strings.Builder{}); err != context.Canceled {
		t.Fatalf("cancel error = %v, want context canceled", err)
	}
}

func TestPlanningAnswerChecksCancellationBeforeReadingAndPreservesEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := planningAnswer(ctx, bufio.NewReader(unreadablePlanningReader{})); err != context.Canceled {
		t.Fatalf("pre-cancel error = %v", err)
	}
	got, err := planningAnswer(context.Background(), bufio.NewReader(strings.NewReader("final answer")))
	if got != "final answer" || !errors.Is(err, io.EOF) {
		t.Fatalf("final line = %q, %v", got, err)
	}
}

func TestDiscoverSemanticReadsBacktickedSkillConfiguration(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo, "INDEX.md")
	if err := os.WriteFile(index, []byte("index"), 0644); err != nil {
		t.Fatal(err)
	}
	configuration := "# Semantic Index Configuration\n\n## Token Cache\n**Local path**: `" + repo + "`\n\n## Semantic Index\n**Local path**: `not-the-cache`\n**Entrypoint**: `" + index + "`\n"
	if err := os.WriteFile(filepath.Join(repo, "docs", "SEMANTIC-INDEX.md"), []byte(configuration), 0644); err != nil {
		t.Fatal(err)
	}
	gotIndex, gotCache, err := discoverSemantic(Input{Repo: repo})
	if err != nil || gotIndex != index || gotCache != repo {
		t.Fatalf("discovery = %q, %q, %v", gotIndex, gotCache, err)
	}
}

func TestExplicitSemanticIndexBypassesBrokenProjectPointer(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gimble"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gimble", "semantic-index.json"), []byte("broken json"), 0644); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo, "INDEX.md")
	if err := os.WriteFile(index, []byte("index"), 0644); err != nil {
		t.Fatal(err)
	}
	gotIndex, gotCache, err := discoverSemantic(Input{Repo: repo, SemanticIndex: index, TokenCache: repo})
	if err != nil || gotIndex != index || gotCache != repo {
		t.Fatalf("explicit discovery = %q, %q, %v", gotIndex, gotCache, err)
	}
}

func TestNotYetBuiltSemanticIndexDoesNotBlockPlanning(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	config := "## Token Cache\n**Local path**: `/not-materialized-yet`\n## Semantic Index\n**Status**: Not yet built\n**Entrypoint**: `/not-built-yet/README.md`\n"
	if err := os.WriteFile(filepath.Join(repo, "docs", "SEMANTIC-INDEX.md"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	index, cache, err := discoverSemantic(Input{Repo: repo})
	if err != nil || index != "" || cache != "" {
		t.Fatalf("unavailable index = %q, %q, %v", index, cache, err)
	}
}

func TestInterviewCannotReusePreliminaryDecisions(t *testing.T) {
	repo, output := t.TempDir(), filepath.Join(t.TempDir(), "output")
	fs := []*fakeAdapter{{name: "codex"}, {name: "claude"}, {name: "gemini", question: true, blankFinalDecisions: true}}
	lanes := []lane{{name: "codex", model: "m", adapter: fs[0]}, {name: "claude", model: "m", adapter: fs[1]}, {name: "gemini", model: "m", adapter: fs[2]}}
	var out strings.Builder
	err := runInjected(t, Input{Repo: repo, Goal: "goal", OutputDir: output}, lanes, bufio.NewReader(strings.NewReader("answer\n")), &out)
	if err == nil || !strings.Contains(err.Error(), "omitted its decisions") {
		t.Fatalf("preliminary decisions accepted as final: %v", err)
	}
	for _, name := range []string{"merge-notes.md", "plan.md"} {
		if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
			t.Fatalf("published %s without final decisions: %v", name, err)
		}
	}
}
