package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/workflow"
	"golang.org/x/tools/go/packages"
)

const cacheFixtureSource = `package fixture
import("context";"github.com/tylergannon/gimbal")
// Flow documents the original workflow.
func Flow(ctx context.Context, env gimbal.Env) error { gimbal.Set(ctx,"first","literal"); return nil }
`

func cacheFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	mod = []byte(strings.Replace(string(mod), "module github.com/tylergannon/gimbal", "module example.com/cachefixture", 1) + fmt.Sprintf("\nrequire github.com/tylergannon/gimbal v0.0.0\nreplace github.com/tylergannon/gimbal => %s\n", root))
	for name, data := range map[string][]byte{"go.mod": mod, "go.sum": sums, "flow.go": []byte(cacheFixtureSource)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func expectCachedSet(t *testing.T, result analysisResult, name string) {
	t.Helper()
	if len(result.Graph.Body) != 1 {
		t.Fatalf("body: %+v", result.Graph.Body)
	}
	set, ok := result.Graph.Body[0].(workflow.Set)
	if !ok || set.Key != name {
		t.Fatalf("set: %#v; want %s", result.Graph.Body[0], name)
	}
}

func TestGenerationCacheAcrossProcessesAndMissingOutput(t *testing.T) {
	dir := cacheFixture(t)
	cache := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(mode string) {
		t.Helper()
		cmd := exec.Command(executable, "-test.run=^TestGenerationCacheProcess$")
		cmd.Env = append(os.Environ(), "GIMBAL_GENERATE_CACHE_DIR="+cache, "GIMBAL_CACHE_TEST_DIR="+dir, "GIMBAL_CACHE_TEST_MODE="+mode)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s child: %v\n%s", mode, err, output)
		}
		t.Logf("%s: %s", mode, output)
	}
	run("cold")
	output := filepath.Join(dir, "workflow_gen.go")
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), `Key: "first"`) {
		t.Fatalf("generated literal Set missing:\n%s", before)
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	run("warm")
	after, err := os.ReadFile(output)
	if err != nil || string(after) != string(before) {
		t.Fatalf("restored output differs: %v", err)
	}
}

func TestGenerationCacheProcess(t *testing.T) {
	dir := os.Getenv("GIMBAL_CACHE_TEST_DIR")
	if dir == "" {
		return
	}
	if os.Getenv("GIMBAL_CACHE_TEST_MODE") == "warm" {
		loadSourceTypes = func(*packages.Config, ...string) ([]*packages.Package, error) {
			return nil, fmt.Errorf("warm process repeated root typecheck")
		}
	}
	start := time.Now()
	var err error
	if os.Getenv("GIMBAL_CACHE_TEST_STOCK") == "1" {
		err = Stock(dir)
	} else {
		err = Source(dir, "Flow", "fixture", "workflow_gen.go", "")
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("generation=%s\n", time.Since(start))
}

func TestGenerationCacheEditsOptionsAndCorruption(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	dir := cacheFixture(t)
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	count := 0
	loadSourceTypes = func(c *packages.Config, p ...string) ([]*packages.Package, error) { count++; return load(c, p...) }
	analyzeFlow := func(name string) analysisResult {
		t.Helper()
		r, e := analyzeOne(dir, "Flow", name, nil, true, true)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	first := analyzeFlow("fixture")
	expectCachedSet(t, first, "first")
	if first.Entry.Summary != "Flow documents the original workflow." {
		t.Fatal(first.Entry.Summary)
	}
	_ = analyzeFlow("fixture")
	if count != 1 {
		t.Fatalf("warm typechecks=%d; want1", count)
	}
	changed := strings.ReplaceAll(cacheFixtureSource, "first", "second")
	changed = strings.ReplaceAll(changed, "original workflow", "edited workflow")
	if err := os.WriteFile(filepath.Join(dir, "flow.go"), []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	r := analyzeFlow("fixture")
	expectCachedSet(t, r, "second")
	if r.Entry.Summary != "Flow documents the edited workflow." || count != 2 {
		t.Fatalf("comment/source edit not reflected: %+v loads=%d", r.Entry, count)
	}
	r = analyzeFlow("renamed")
	if r.Graph.Name != "renamed" || count != 3 {
		t.Fatalf("options did not invalidate: %s loads%d", r.Graph.Name, count)
	}
	files, err := filepath.Glob(filepath.Join(os.Getenv("GIMBAL_GENERATE_CACHE_DIR"), "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("missing cache entries: %v", err)
	}
	for _, path := range files {
		if err := os.WriteFile(path, []byte("broken cache"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expectCachedSet(t, analyzeFlow("renamed"), "second")
	if count != 4 {
		t.Fatalf("corruption typechecks=%d", count)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package fixture\nvar invalid = nonexistent\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeOne(dir, "Flow", "renamed", nil, true, true); err == nil || !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("new invalid source hidden by cache: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "broken.go")); err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, analyzeFlow("renamed"), "second")
}

func TestGenerationCacheBindsRootBytesAcrossABA(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	dir := cacheFixture(t)
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	loadSourceTypes = func(c *packages.Config, p ...string) ([]*packages.Package, error) {
		path := filepath.Join(dir, "flow.go")
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(cacheFixtureSource, "first", "wrong")), 0600); err != nil {
			return nil, err
		}
		result, err := load(c, p...)
		if e := os.WriteFile(path, []byte(cacheFixtureSource), 0600); e != nil {
			return nil, e
		}
		return result, err
	}
	r, err := analyzeOne(dir, "Flow", "fixture", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, r, "first")
	loadSourceTypes = func(*packages.Config, ...string) ([]*packages.Package, error) {
		return nil, fmt.Errorf("captured root result was not cached")
	}
	r, err = analyzeOne(dir, "Flow", "fixture", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, r, "first")
}

func TestGenerationCacheRejectsChangedCompilerExportsAcrossABA(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", cache)
	dir := cacheFixture(t)
	if err := os.Mkdir(filepath.Join(dir, "helper"), 0700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "helper", "key.go")
	initial := []byte("package helper\nconst Key = \"first\"\n")
	if err := os.WriteFile(helper, initial, 0600); err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(cacheFixtureSource, `"context";`, `"context";"example.com/cachefixture/helper";`, 1)
	source = strings.Replace(source, `"first"`, "helper.Key", 1)
	if err := os.WriteFile(filepath.Join(dir, "flow.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	loadSourceTypes = func(c *packages.Config, p ...string) ([]*packages.Package, error) {
		if err := os.WriteFile(helper, []byte("package helper\nconst Key = \"changed\"\n"), 0600); err != nil {
			return nil, err
		}
		result, err := load(c, p...)
		if e := os.WriteFile(helper, initial, 0600); e != nil {
			return nil, e
		}
		return result, err
	}
	r, err := analyzeOne(dir, "Flow", "fixture", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, r, "changed")
	entries, _ := filepath.Glob(filepath.Join(cache, "*.json"))
	if len(entries) != 0 {
		t.Fatalf("published result with different actual compiler exports: %v", entries)
	}
	loadSourceTypes = load
	r, err = analyzeOne(dir, "Flow", "fixture", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, r, "first")
}

func TestGenerationCacheRepairsDisposableOutput(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	dir := cacheFixture(t)
	path := filepath.Join(dir, "workflow_gen.go")
	if err := GenerateGraph(dir, "Flow", "fixture", "workflow_gen.go"); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(expected), `Key: "first"`) {
		t.Fatalf("literal workflow missing: %s", expected)
	}
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	for _, cold := range []bool{false, true} {
		if cold {
			t.Setenv("GIMBAL_GENERATE_CACHE", "off")
			loadSourceTypes = load
		} else {
			loadSourceTypes = func(*packages.Config, ...string) ([]*packages.Package, error) {
				return nil, fmt.Errorf("repair missed warm analysis cache")
			}
		}
		for _, damage := range []string{"missing", "stale", "broken"} {
			t.Run(fmt.Sprintf("cold=%t/%s", cold, damage), func(t *testing.T) {
				if damage == "missing" {
					err = os.Remove(path)
				} else if damage == "stale" {
					err = os.WriteFile(path, append(append([]byte(nil), expected...), []byte("\nvar stale = noLongerExists\n")...), 0600)
				} else {
					err = os.WriteFile(path, []byte("// Code generated by internal/generate. DO NOT EDIT.\n\nnot valid Go syntax\n"), 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := GenerateGraph(dir, "Flow", "fixture", "workflow_gen.go"); err != nil {
					t.Fatal(err)
				}
				actual, err := os.ReadFile(path)
				if err != nil || string(actual) != string(expected) {
					t.Fatalf("repair differs: %v", err)
				}
			})
		}
	}
}

func TestGenerationCacheConcurrentPublication(t *testing.T) {
	cache := &analysisCache{dir: t.TempDir(), producer: "fixture-producer"}
	input := &analysisInputs{manifest: inputManifest{Dir: "/fixture"}}
	requests := []analysisRequest{{Name: "fixture"}}
	results := []analysisResult{{Graph: workflow.Graph{Name: "fixture", Body: []workflow.Operation{workflow.Set{Key: "literal"}}}}}
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			for range 10 {
				cache.write(input, requests, results)
			}
		})
	}
	for range 100 {
		if result, ok := cache.read(input, requests); ok {
			expectCachedSet(t, result[0], "literal")
		}
	}
	workers.Wait()
	result, ok := cache.read(input, requests)
	if !ok {
		t.Fatal("concurrent writers left no usable entry")
	}
	expectCachedSet(t, result[0], "literal")
	files, err := os.ReadDir(cache.dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("publication leftovers: %v %v", files, err)
	}
}

func TestCompilerExportIdentityUsesOnlyGoContentAddressedStore(t *testing.T) {
	cache := t.TempDir()
	id := strings.Repeat("abcde012", 8)
	path := filepath.Join(cache, id[:2], id+"-d")
	if got, err := compilerExportID(cache, path); err != nil || got != id {
		t.Fatalf("identity=%s: %v", got, err)
	}
	for _, path := range []string{filepath.Join(cache, "exports.a"), filepath.Join(cache, "other", id+"-d"), filepath.Join(t.TempDir(), id[:2], id+"-d")} {
		if _, err := compilerExportID(cache, path); err == nil {
			t.Fatalf("accepted unsupported store %s", path)
		}
	}
}

func TestGenerationCacheReplacesSourceAndProducerVersions(t *testing.T) {
	cache := &analysisCache{dir: t.TempDir(), producer: "first-producer"}
	input := &analysisInputs{manifest: inputManifest{Dir: "/fixture", Sources: map[string]string{"flow.go": "first-source"}}}
	requests := []analysisRequest{{Name: "fixture"}}
	result := []analysisResult{{Graph: workflow.Graph{Name: "fixture", Body: []workflow.Operation{workflow.Set{Key: "literal"}}}}}
	cache.write(input, requests, result)
	input.manifest.Sources["flow.go"] = "second-source"
	if _, ok := cache.read(input, requests); ok {
		t.Fatal("source change reused old result")
	}
	cache.write(input, requests, result)
	cache.producer = "second-producer"
	if _, ok := cache.read(input, requests); ok {
		t.Fatal("producer change reused old result")
	}
	cache.write(input, requests, result)
	files, err := os.ReadDir(cache.dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("old versions accumulated: %v %v", files, err)
	}
	actual, ok := cache.read(input, requests)
	if !ok {
		t.Fatal("latest version missing")
	}
	expectCachedSet(t, actual[0], "literal")
	// A valid JSON edit can decode successfully while corrupting semantic values.
	path := cache.path(input, requests)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), "literal", "corrupt", 1)
	if changed == string(raw) || !json.Valid([]byte(changed)) {
		t.Fatal("fixture did not alter a valid JSON literal")
	}
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.read(input, requests); ok {
		t.Fatal("accepted valid JSON with corrupted result")
	}
}

func TestGenerationCachePackageErrorsKeepOrdinaryDiagnostics(t *testing.T) {
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	dir := cacheFixture(t)
	for _, source := range []string{
		"package fixture\nvar invalid = nonexistent\n",
		"package fixture\nfunc broken( {\n",
		"package fixture\nimport _ \"example.com/nonexistent\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		_, captureErr := captureAnalysisInputs(dir, nil, []string{"."})
		t.Setenv("GIMBAL_GENERATE_CACHE", "off")
		loadSourceTypes = load
		_, ordinaryErr := analyzeOne(dir, "Flow", "fixture", nil, true, false)
		if captureErr == nil || ordinaryErr == nil || captureErr.Error() != ordinaryErr.Error() {
			t.Fatalf("capture=%v\nordinary=%v", captureErr, ordinaryErr)
		}
		t.Setenv("GIMBAL_GENERATE_CACHE", "")
		loadSourceTypes = func(*packages.Config, ...string) ([]*packages.Package, error) {
			return nil, fmt.Errorf("repeated checker after known package error")
		}
		_, cachedErr := analyzeOne(dir, "Flow", "fixture", nil, true, false)
		if cachedErr == nil || cachedErr.Error() != ordinaryErr.Error() {
			t.Fatalf("cached=%v\nordinary=%v", cachedErr, ordinaryErr)
		}
	}
}
