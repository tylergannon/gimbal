package semanticindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
)

type fakeReader struct {
	mu      sync.Mutex
	turns   int
	fail    bool
	invalid bool
	cancel  bool
	mutate  func()
}

func (*fakeReader) CreateSession(context.Context, string, string) (string, error) {
	return "reader", nil
}
func (f *fakeReader) RunTurn(ctx context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	f.mu.Lock()
	f.turns++
	f.mu.Unlock()
	if f.mutate != nil {
		f.mutate()
	}
	if f.cancel {
		<-ctx.Done()
		return gimble.TurnResult{}, ctx.Err()
	}
	if f.fail {
		return gimble.TurnResult{}, errors.New("model failed")
	}
	parts := strings.Split(prompt, "Cite only ")
	if len(parts) < 2 {
		return gimble.TurnResult{}, errors.New("missing assigned source")
	}
	rel, _, _ := strings.Cut(parts[1], " using path")
	line := 1
	if f.invalid {
		line = 9999
	}
	result := readerOutput{Summary: "How to work with " + rel, Themes: []string{"configure " + rel}, Citations: []Citation{{Path: rel, StartLine: line, EndLine: line, Note: "source evidence"}}, Gotchas: []string{}, Recipes: []string{"configure " + rel}}
	raw, _ := json.Marshal(result)
	return gimble.TurnResult{Output: raw}, nil
}
func (*fakeReader) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fakeReader) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*fakeReader) Close(context.Context, string) error                 { return nil }

func runBuild(t *testing.T, ctx context.Context, source, output, mode string, fake *fakeReader) (string, string, error) {
	t.Helper()
	var report strings.Builder
	var entry string
	err := gimble.Run(gimble.Project(ctx, t.TempDir()), "index-test", func(runCtx context.Context) error {
		var err error
		entry, err = build(runCtx, Input{Source: source, Output: output, Mode: mode}, &report, fake)
		return err
	})
	return entry, report.String(), err
}

func corpus(t *testing.T) (string, string) {
	t.Helper()
	source := t.TempDir()
	output := filepath.Join(t.TempDir(), "index")
	if err := os.WriteFile(filepath.Join(source, "a.md"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return source, output
}

func stateBytes(t *testing.T, output string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(output, stateFile))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBuildUpdateDeleteNoopAndAudit(t *testing.T) {
	source, output := corpus(t)
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	if f.turns != 1 {
		t.Fatalf("build turns: %d", f.turns)
	}
	first := stateBytes(t, output)
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	if f.turns != 1 {
		t.Fatalf("unchanged update reread source: %d", f.turns)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	if f.turns != 2 {
		t.Fatalf("build reused unchanged source: %d", f.turns)
	}
	if err := os.WriteFile(filepath.Join(source, "b.md"), []byte("gamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "audit", f); err == nil {
		t.Fatal("audit missed added source")
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	if f.turns != 3 {
		t.Fatalf("update did not read only addition: %d", f.turns)
	}
	oldLeaf := filepath.Join(output, "sources", leafName("a.md"))
	if err := os.Remove(filepath.Join(source, "a.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldLeaf); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale leaf survived: %v", err)
	}
	if _, report, err := runBuild(t, context.Background(), source, output, "audit", f); err != nil || !strings.Contains(report, "citations=1") || !strings.Contains(report, "stale=0") {
		t.Fatalf("audit: %v %s", err, report)
	}
	if string(first) == string(stateBytes(t, output)) {
		t.Fatal("state never changed")
	}
}

func TestInvalidCitationFailureCancellationAndMidRunEditPreserveIndex(t *testing.T) {
	source, output := corpus(t)
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	before := string(stateBytes(t, output))
	if err := os.WriteFile(filepath.Join(source, "a.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*fakeReader{{fail: true}, {invalid: true}, {mutate: func() { _ = os.WriteFile(filepath.Join(source, "a.md"), []byte("changed again\n"), 0o644) }}} {
		if _, _, err := runBuild(t, context.Background(), source, output, "update", bad); err == nil {
			t.Fatal("invalid run succeeded")
		}
		if got := string(stateBytes(t, output)); got != before {
			t.Fatal("failure replaced valid index")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := runBuild(t, ctx, source, output, "update", &fakeReader{cancel: true}); err == nil {
		t.Fatal("cancelled run succeeded")
	}
	if got := string(stateBytes(t, output)); got != before {
		t.Fatal("cancellation replaced valid index")
	}
}

func TestOverlapMissingInputCorruptStateAndDebt(t *testing.T) {
	source, output := corpus(t)
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), "", output, "build", f); err == nil {
		t.Fatal("empty source accepted")
	}
	if _, _, err := runBuild(t, context.Background(), source, filepath.Join(source, "inside"), "build", f); err == nil {
		t.Fatal("overlap accepted")
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, filepath.Join(link, "missing", "deep"), "build", f); err == nil {
		t.Fatal("symlink prefix overlap accepted")
	}
	if err := os.WriteFile(filepath.Join(source, "binary"), []byte{0, 1}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stateBytes(t, output)), "binary or non-UTF-8") {
		t.Fatal("unsupported source not reported as debt")
	}
	if err := os.WriteFile(filepath.Join(output, stateFile), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err == nil {
		t.Fatal("corrupt state accepted")
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err == nil {
		t.Fatal("build clobbered corrupt existing index")
	}
}

func TestCustomEvalSurvivesUpdate(t *testing.T) {
	source, output := corpus(t)
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, ".semantic-index", "evals.jsonl")
	data, _ := os.ReadFile(path)
	custom := []byte(`{"id":"my-case","query":"Find alpha","expected_routes":["sources/a.md"],"expected_citations":["a.md:1-1"]}` + "\n")
	if err := os.WriteFile(path, append(data, custom...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "my-case") || !strings.Contains(string(data), "starter-001") || !strings.Contains(string(data), "a.md:1-1") {
		t.Fatalf("bad evals: %s", data)
	}
}

func TestMissingLeafIsRegeneratedAndDifferentSourceIsRejected(t *testing.T) {
	source, output := corpus(t)
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(output, "sources", leafName("a.md"))
	if err := os.Remove(leaf); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "audit", f); err == nil {
		t.Fatal("audit accepted missing leaf")
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	if f.turns != 2 {
		t.Fatalf("missing leaf was not reread: %d", f.turns)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "a.md"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := string(stateBytes(t, output))
	if _, _, err := runBuild(t, context.Background(), other, output, "build", f); err == nil {
		t.Fatal("different source replaced index")
	}
	if got := string(stateBytes(t, output)); got != before {
		t.Fatal("source mismatch damaged index")
	}
}

func TestRoutePagesResolveAndCollapseAfterUpdate(t *testing.T) {
	source, output := corpus(t)
	for i := 1; i < 9; i++ {
		name := fmt.Sprintf("task-%02d.md", i)
		if err := os.WriteFile(filepath.Join(source, name), []byte("task\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := &fakeReader{}
	if _, _, err := runBuild(t, context.Background(), source, output, "build", f); err != nil {
		t.Fatal(err)
	}
	realSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	s, err := readState(output, realSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Routes) == 0 {
		t.Fatal("nine distinct themes did not create route pages")
	}
	root, err := os.ReadFile(filepath.Join(output, "routes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(markdownTargets(string(root))); n > 8 {
		t.Fatalf("unbounded root fanout: %d", n)
	}
	if err := validateIndexedFiles(output, s, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "audit", f); err != nil {
		t.Fatal(err)
	}
	oldRoute := filepath.Join(output, filepath.FromSlash(s.Routes[0]))
	for i := 7; i < 9; i++ {
		if err := os.Remove(filepath.Join(source, fmt.Sprintf("task-%02d.md", i))); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "update", f); err != nil {
		t.Fatal(err)
	}
	s, err = readState(output, realSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Routes) != 0 {
		t.Fatalf("thin index still has route pages: %v", s.Routes)
	}
	if _, err := os.Stat(oldRoute); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale page survived: %v", err)
	}
	if _, _, err := runBuild(t, context.Background(), source, output, "audit", f); err != nil {
		t.Fatal(err)
	}
}

func TestLargeRouteTreeKeepsFanoutBounded(t *testing.T) {
	for _, sharedTheme := range []bool{false, true} {
		stage := t.TempDir()
		s := state{Sources: map[string]sourceState{}}
		var files []sourceFile
		for i := range 80 {
			rel := fmt.Sprintf("task-%03d.md", i)
			files = append(files, sourceFile{Rel: rel})
			theme := fmt.Sprintf("solve task %03d", i)
			if sharedTheme {
				theme = "solve shared task"
			}
			s.Sources[rel] = sourceState{Leaf: "sources/" + leafName(rel), Themes: []string{theme}}
		}
		if err := writeRoutes(stage, files, &s, nil, nil); err != nil {
			t.Fatal(err)
		}
		if len(s.Routes) <= 8 {
			t.Fatalf("80 sources should need multiple route levels: %d, shared=%t", len(s.Routes), sharedTheme)
		}
		for _, path := range append([]string{"routes.md"}, s.Routes...) {
			data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			if n := len(markdownTargets(string(data))); n > 8 {
				t.Fatalf("%s has %d links, shared=%t", path, n, sharedTheme)
			}
		}
		for rel, entry := range s.Sources {
			if len(entry.RouteChain) < 2 {
				t.Fatalf("%s lacks route chain: %v", rel, entry.RouteChain)
			}
		}
	}
}
