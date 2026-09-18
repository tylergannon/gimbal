package gimble

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tylergannon/gimble/internal/observation"
)

func TestFileBackedScopeValuesStayContainedAndReplay(t *testing.T) {
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	ctx := observation.WithRegistry(Project(t.Context(), project), registry)
	largeText := strings.Repeat("before ", 6000) + "TEXT-SENTINEL" + strings.Repeat(" after", 6000)
	largeJSON := review{Objections: []string{strings.Repeat("json ", 10_000) + "JSON-SENTINEL"}}
	var dir string
	if err := Run(ctx, "artifacts", nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		return Scope(ctx, "../../child", func(ctx context.Context) error {
			Set(ctx, "../../text", largeText)
			SetJSON(ctx, "json/value", largeJSON)
			data := scopeData(ctx)
			if data.By["../../text"].Value != largeText {
				t.Error("typed scope data did not recover the complete text artifact")
			}
			got, ok := data.By["json/value"].Value.(map[string]any)
			if !ok || !strings.Contains(fmt.Sprint(got["objections"]), "JSON-SENTINEL") {
				t.Errorf("typed scope data did not recover the complete JSON artifact: %#v", data.By["json/value"].Value)
			}
			id := filepath.Base(dir)
			live, err := registry.Snapshot(id)
			if err != nil || len(live.Scopes) < 2 {
				t.Fatalf("live snapshot = %d scopes, %v", len(live.Scopes), err)
			}
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}

	id := filepath.Base(dir)
	rows := readScopeRows(t, dir)
	if len(rows) != 2 {
		t.Fatalf("scopes.json has %d rows, want root and ended child", len(rows))
	}
	child := rows[1]
	if child.Status != observation.StatusEnded {
		t.Fatalf("child scope status = %q, want ended", child.Status)
	}
	textValue := child.Values["../../text"]
	jsonValue := child.Values["json/value"]
	for key, value := range map[string]observation.ScopeValue{"../../text": textValue, "json/value": jsonValue} {
		if value.Artifact == nil || len(value.Value) != 0 {
			t.Fatalf("%s row = %+v, want only an artifact descriptor", key, value)
		}
		name := filepath.Join(dir, filepath.FromSlash(value.Artifact.File))
		rel, err := filepath.Rel(dir, name)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("%s artifact escaped run: %q", key, value.Artifact.File)
		}
	}
	textRaw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(textValue.Artifact.File)))
	if err != nil || string(textRaw) != largeText {
		t.Fatalf("text artifact is not exact: %d bytes, %v", len(textRaw), err)
	}
	jsonRaw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(jsonValue.Artifact.File)))
	var decoded review
	if err != nil || json.Unmarshal(jsonRaw, &decoded) != nil || len(decoded.Objections) != 1 || !strings.Contains(decoded.Objections[0], "JSON-SENTINEL") {
		t.Fatalf("JSON artifact is not complete valid JSON: %d bytes, %v", len(jsonRaw), err)
	}
	if err := filepath.WalkDir(filepath.Join(dir, "artifacts"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".gimble-artifact-") {
			t.Errorf("atomic publication left temporary file %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"observation.json", "observation-deltas.jsonl", "scopes.json"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	rebuilt, err := observation.NewRegistry(project).Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	replayed := rebuilt.Scopes[child.Key].Values["../../text"]
	if replayed.Artifact == nil || *replayed.Artifact != *textValue.Artifact {
		t.Fatalf("log rebuild artifact = %+v, want %+v", replayed.Artifact, textValue.Artifact)
	}

	t.Run("write failure panics", func(t *testing.T) {
		project := t.TempDir()
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = Run(Project(t.Context(), project), "write-failure", nil, func(ctx context.Context) error {
				if err := os.WriteFile(filepath.Join(runDir(ctx), "artifacts"), []byte("not a directory"), 0o644); err != nil {
					return err
				}
				Set(ctx, "large", strings.Repeat("payload ", 10_000))
				return nil
			})
		}()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), `set "large": write artifact`) {
			t.Fatalf("Set recovered %v, want artifact-write panic", recovered)
		}
	})
}

func TestDefaultScopeContextStaysWithinItsTokenBudget(t *testing.T) {
	tests := []struct {
		name  string
		set   func(context.Context)
		check func(*testing.T, string)
	}{
		{
			name: "large unicode value",
			set: func(ctx context.Context) {
				Set(ctx, "report", strings.Repeat("évidence ", 7000)+"MIDDLE-ONLY-SENTINEL"+strings.Repeat(" fin", 7000))
			},
			check: func(t *testing.T, text string) {
				if strings.Contains(text, "MIDDLE-ONLY-SENTINEL") || !strings.Contains(text, "bytes omitted") || !utf8.ValidString(text) {
					t.Fatalf("large Unicode value was not a valid bounded reference")
				}
				path := referencePath(t, text)
				raw, err := os.ReadFile(path)
				if err != nil || !strings.Contains(string(raw), "MIDDLE-ONLY-SENTINEL") {
					t.Fatalf("referenced artifact lacks sentinel: %v", err)
				}
			},
		},
		{
			name: "aggregate overflow",
			set: func(ctx context.Context) {
				for i := range 12 {
					key := fmt.Sprintf("entry-%02d", i)
					store(ctx, key, encode(key, strings.Repeat(fmt.Sprintf("value-%02d ", i), 1800)))
				}
			},
			check: func(t *testing.T, text string) {
				for i := range 12 {
					if !strings.Contains(text, fmt.Sprintf("## entry-%02d", i)) {
						t.Fatalf("aggregate render lost entry %d", i)
					}
				}
			},
		},
		{
			name: "reference index fallback",
			set: func(ctx context.Context) {
				for i := range 70 {
					var key strings.Builder
					fmt.Fprintf(&key, "key-%03d-", i)
					for j := range 120 {
						fmt.Fprintf(&key, "%04x", i*131+j*977)
					}
					store(ctx, key.String(), encode(key.String(), "value"))
				}
			},
			check: func(t *testing.T, text string) {
				for _, path := range referencePaths(text) {
					raw, err := os.ReadFile(path)
					if err == nil && strings.Contains(string(raw), "key-000-") && strings.Contains(string(raw), "key-069-") {
						return
					}
				}
				t.Fatal("no referenced context index contains every key")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var prompt string
			adapter := &fake{answer: func(_ context.Context, _, got string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
				prompt = got
				return "done", nil
			}}
			if err := runTest(t, bind(adapter, "m", "worker"), func(ctx context.Context) error {
				test.set(ctx)
				_, err := NewSession(ctx, "worker", t.TempDir()).Generate[Text](ctx, "inspect")
				return err
			}); err != nil {
				t.Fatal(err)
			}
			contextText := strings.TrimPrefix(prompt, "inspect\n\n")
			if got := tokenCount(contextText); got > contextTokenLimit {
				t.Fatalf("scope context uses %d tokens, limit %d", got, contextTokenLimit)
			}
			test.check(t, contextText)
		})
	}
}

func TestTemplatesAndPlannerFeedbackCannotBypassContextBudget(t *testing.T) {
	t.Run("typed template", func(t *testing.T) {
		var prompt string
		adapter := &fake{answer: func(_ context.Context, _, got string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
			prompt = got
			return "done", nil
		}}
		if err := runTest(t, bind(adapter, "m", "worker"), func(ctx context.Context) error {
			SetJSON(ctx, "review", review{Objections: []string{strings.Repeat("typed ", 10_000) + "TEMPLATE-SENTINEL" + strings.Repeat(" tail", 10_000)}})
			_, err := NewSession(ctx, "worker", t.TempDir()).Generate[Text](ctx, "inspect", WithScopeTemplate(`{{index (index .By "review").Value.objections 0}}`))
			return err
		}); err != nil {
			t.Fatal(err)
		}
		contextText := strings.TrimPrefix(prompt, "inspect\n\n")
		if tokenCount(contextText) > contextTokenLimit || strings.Contains(contextText, "TEMPLATE-SENTINEL") {
			t.Fatalf("template output bypassed budget")
		}
		raw, err := os.ReadFile(referencePath(t, contextText))
		if err != nil || !strings.Contains(string(raw), "TEMPLATE-SENTINEL") {
			t.Fatalf("complete template artifact lacks sentinel: %v", err)
		}
	})

	t.Run("planner previous task", func(t *testing.T) {
		var prompts []string
		calls := 0
		adapter := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
			prompts = append(prompts, prompt)
			calls++
			if calls == 1 {
				return `{"tasks":[{"name":"one","description":"do it","definition_of_done":"done","validation":{"command":"","query":""}}],"next":0}`, nil
			}
			return `{"tasks":[],"next":null}`, nil
		}}
		if err := runTest(t, bind(adapter, "m", "planner"), func(ctx context.Context) error {
			planner := NewSession(ctx, "planner", t.TempDir())
			loop := PromiseLoop(ctx, "work", "finish", planner)
			loop.Tasks(func(taskCtx context.Context, _ Task) bool {
				Set(taskCtx, "result", strings.Repeat("result ", 10_000)+"PREVIOUS-SENTINEL"+strings.Repeat(" tail", 10_000))
				return true
			})
			return loop.Err()
		}); err != nil {
			t.Fatal(err)
		}
		if len(prompts) != 2 || strings.Contains(prompts[1], "PREVIOUS-SENTINEL") || !strings.Contains(prompts[1], "Complete value: ") {
			t.Fatalf("planner previous-task record bypassed budget: %d prompts", len(prompts))
		}
		raw, err := os.ReadFile(referencePath(t, prompts[1]))
		if err != nil || !strings.Contains(string(raw), "PREVIOUS-SENTINEL") {
			t.Fatalf("planner context artifact lacks previous-task sentinel: %v", err)
		}
	})
}

func readScopeRows(t *testing.T, dir string) []observation.ScopeRow {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "scopes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []observation.ScopeRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func referencePath(t *testing.T, text string) string {
	t.Helper()
	paths := referencePaths(text)
	if len(paths) > 0 {
		return paths[0]
	}
	t.Fatalf("text has no complete-value reference: %q", text[max(0, len(text)-300):])
	return ""
}

func referencePaths(text string) []string {
	var paths []string
	for _, label := range []string{"Complete value: ", "Complete output: "} {
		remaining := text
		for {
			at := strings.Index(remaining, label)
			if at < 0 {
				break
			}
			path := remaining[at+len(label):]
			if end := strings.IndexByte(path, '\n'); end >= 0 {
				path = path[:end]
			}
			paths = append(paths, strings.TrimSpace(path))
			remaining = remaining[at+len(label):]
		}
	}
	return paths
}
