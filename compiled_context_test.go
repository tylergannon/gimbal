package gimbal

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func TestCompiledSnapshotRenderingIndependentOfLiveAncestors(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	store := compiledscope.Store{Root: t.TempDir()}
	entry := func(k, v string) compiledscope.Entry {
		b, _ := json.Marshal(v)
		return compiledscope.Entry{Key: k, Value: b}
	}
	err := Run(Project(t.Context(), t.TempDir()), "snapshots", nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		root, err := compiledscope.WriteContext(ctx, "", entry("layer", "parent"), entry("empty", ""))
		if err != nil {
			return err
		}
		var childRef compiledscope.Snapshot
		err = Scope(ctx, "child", func(child context.Context) error {
			writes := []compiledscope.Entry{entry("layer", "child"), entry("large", strings.Repeat("large value ", 150000))}
			for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
				writes = append(writes, entry(k, strings.Repeat("support "+k+" ", 3000)))
			}
			childRef, err = compiledscope.WriteContext(child, root, writes...)
			if err != nil {
				return err
			}
			bound, err := compiledscope.BindContext(child, childRef)
			if err != nil {
				return err
			}
			text, entries := scopeTextAndContext(bound)
			if tokenCount(text) > contextTokenLimit {
				t.Fatal("budget exceeded")
			}
			if strings.Contains(text, "parent") {
				t.Fatal("parent shadow was rendered")
			}
			incomplete := 0
			for _, e := range entries {
				if !e.Complete {
					incomplete++
				}
				if e.Key == "layer" && e.Scope != "child.1" {
					t.Fatal("observation provenance lost")
				}
			}
			if incomplete < 6 {
				t.Fatalf("aggregate wasn't exercised: %d", incomplete)
			}
			for _, v := range visibleValues(bound) {
				if v.value.artifact != nil {
					path := artifactAbsolute(v.owner.run, *v.value.artifact)
					if _, err := os.ReadFile(path); err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(text, path) {
						t.Fatalf("missing complete-value path for %s", v.key)
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		// Erase observation metadata, then bind the retained child after cleanup.
		s, _ := current(ctx)
		s.mu.Lock()
		s.values = nil
		s.keys = nil
		s.mu.Unlock()
		bound, err := compiledscope.BindContext(ctx, childRef)
		if err != nil {
			return err
		}
		if got := scopeData(bound).By["layer"].Text; got != "child" {
			t.Fatalf("child lost after cleanup: %s", got)
		}
		bound, err = compiledscope.BindContext(ctx, root)
		if err != nil {
			return err
		}
		data := scopeData(bound)
		if len(data.Values) != 2 || data.By["layer"].Text != "parent" {
			t.Fatal("parent reference changed")
		}
		_, entries := scopeTextAndContext(bound)
		for _, e := range entries {
			if !e.Complete {
				t.Fatal("small complete value not recorded complete")
			}
		}
		if entries := templateContextEntries(bound, "parent"); entries[1].Complete {
			t.Fatal("omitted empty template value marked complete")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompiledTemplateRetainsOmittedInputsAndJSONCompleteness(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	store := compiledscope.Store{Root: t.TempDir()}
	raw, _ := json.Marshal(map[string]string{"text": strings.Repeat("a", 5000)})
	ref, err := store.Extend(t.Context(), "", compiledscope.Entry{Key: "json", Value: raw}, compiledscope.Entry{Key: "empty", Value: json.RawMessage(`""`)})
	if err != nil {
		t.Fatal(err)
	}
	err = Run(Project(t.Context(), t.TempDir()), "template-snapshot", nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		ctx, err = compiledscope.BindContext(ctx, ref)
		if err != nil {
			return err
		}
		_, entries := scopeTextAndContext(ctx)
		if !entries[0].Complete {
			t.Fatal("full JSON was classified as an excerpt")
		}
		for _, template := range []string{`selected`, `{{.By.empty.Text}}`, `{{.By.json.Text}}`} {
			prompt, entries, err := scopedPrompt(ctx, "inspect", options{scopeTemplate: template})
			if err != nil {
				return err
			}
			if tokenCount(prompt) > contextTokenLimit+tokenCount("inspect") {
				t.Fatal("template exceeded budget")
			}
			label := "Complete scoped context index: "
			at := strings.LastIndex(prompt, label)
			if at < 0 {
				t.Fatal("template omitted snapshot retrieval index")
			}
			indexPath := strings.TrimSpace(prompt[at+len(label):])
			index, err := os.ReadFile(indexPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(index), "## json") || !strings.Contains(string(index), "## empty") {
				t.Fatal("incomplete template index")
			}
			if entries[1].Complete {
				t.Fatal("empty template value incorrectly marked complete")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
