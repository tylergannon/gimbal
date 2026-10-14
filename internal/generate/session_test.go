package generate

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/tylergannon/polytype/goload"
)

func TestGenerationSessionRunsOnlyAfterPersistentMiss(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	dir := cacheFixture(t)
	var session goload.Session
	analyzeWith := func(name string, session *goload.Session) analysisResult {
		t.Helper()
		results, err := analyze(dir, nil, []string{"."}, []analysisRequest{{Dir: dir, Entry: "Flow", Name: name, Command: true, Inspect: true}}, session)
		if err != nil {
			t.Fatal(err)
		}
		return results[0]
	}
	first := analyzeWith("fixture", &session)
	expectCachedSet(t, first, "first")
	initial := session.Stats()
	if initial.PackageChecks != 1 || initial.FileParses != 1 || initial.FileReads != 0 || initial.FallbackLoads != 0 {
		t.Fatalf("cold shared analysis: %+v", initial)
	}
	var untouched goload.Session
	expectCachedSet(t, analyzeWith("fixture", &untouched), "first")
	if stats := untouched.Stats(); stats != (goload.Stats{}) {
		t.Fatalf("persistent hit requested package work: %+v", stats)
	}
	// A distinct generation request misses the persistent result but uses the
	// same source graph. Its literal product must reflect the new request.
	renamed := analyzeWith("renamed", &session)
	expectCachedSet(t, renamed, "first")
	if renamed.Graph.Name != "renamed" {
		t.Fatalf("reused analysis lost requested graph name: %q", renamed.Graph.Name)
	}
	if stats := session.Stats(); stats.PackageChecks != initial.PackageChecks || stats.FileParses != initial.FileParses || stats.PackageHits != initial.PackageHits+1 {
		t.Fatalf("compatible persistent miss repeated analysis: initial=%+v after=%+v", initial, stats)
	}
}

func TestGenerationSessionBindsCapturedSourceOnReuse(t *testing.T) {
	dir := cacheFixture(t)
	input, err := captureAnalysisInputs(dir, nil, []string{"."})
	if err != nil || input == nil {
		t.Fatalf("capture: %v", err)
	}
	path := filepath.Join(dir, "flow.go")
	if err := os.WriteFile(path, []byte("not valid Go source"), 0600); err != nil {
		t.Fatal(err)
	}
	var session goload.Session
	first, err := input.load(&session)
	if err != nil || !input.bound {
		t.Fatalf("captured load: %v bound=%v", err, input.bound)
	}
	pkg, err := first.packageAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	graph, _, err := extractLoaded(pkg, first.imports, "Flow", "fixture", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, analysisResult{Graph: graph}, "first")
	second, err := input.load(&session)
	if err != nil || !input.bound {
		t.Fatalf("captured reuse: %v bound=%v", err, input.bound)
	}
	again, err := second.packageAt(dir)
	if err != nil || again.Types != pkg.Types || again.TypesInfo != pkg.TypesInfo || again.Syntax[0] != pkg.Syntax[0] {
		t.Fatalf("compatible graph not reused: %v", err)
	}
	if stats := session.Stats(); stats.PackageChecks != 1 || stats.FileParses != 1 || stats.PackageHits != 1 || stats.FileReads != 0 || stats.FallbackLoads != 0 {
		t.Fatalf("captured source reread/rechecked: %+v", stats)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(cacheFixtureSource, `"first"`, `"edited"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	edited, err := captureAnalysisInputs(dir, nil, []string{"."})
	if err != nil || edited == nil {
		t.Fatalf("edited capture: %v", err)
	}
	loaded, err := edited.load(&session)
	if err != nil || !edited.bound {
		t.Fatalf("edited load: %v bound=%v", err, edited.bound)
	}
	current, err := loaded.packageAt(dir)
	if err != nil || current.Types == pkg.Types || current.Syntax[0] == pkg.Syntax[0] {
		t.Fatalf("changed source reused old graph: %v", err)
	}
	graph, _, err = extractLoaded(current, loaded.imports, "Flow", "fixture", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectCachedSet(t, analysisResult{Graph: graph}, "edited")
}

func TestGenerationSessionFallbackPreservesDependencyExports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/sharedfallback\n\ngo 1.27\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "value.go")
	if err := os.WriteFile(source, []byte("invalid disk source"), 0600); err != nil {
		t.Fatal(err)
	}
	parsed := 0
	cfg := &packages.Config{Dir: dir, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports, Overlay: map[string][]byte{source: []byte("package value\nimport \"net/http\"\nconst Current=17\nvar Status=http.StatusOK\n")}, ParseFile: func(f *token.FileSet, name string, b []byte) (*ast.File, error) {
		return parser.ParseFile(f, name, b, parser.ParseComments)
	}}

	originalLoader := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = originalLoader })
	loadSourceTypes = func(config *packages.Config, patterns ...string) ([]*packages.Package, error) {
		parse := config.ParseFile
		config.ParseFile = func(f *token.FileSet, name string, b []byte) (*ast.File, error) { parsed++; return parse(f, name, b) }
		return packages.Load(config, patterns...)
	}
	var session goload.Session
	roots, err := loadSessionPackages(&session, cfg, ".")
	if err != nil || len(roots) != 1 {
		t.Fatalf("fallback: %v, %d roots", err, len(roots))
	}
	if len(roots[0].Errors) > 0 {
		t.Fatal(roots[0].Errors)
	}
	value, ok := roots[0].Types.Scope().Lookup("Current").(*types.Const)
	if !ok || constant.Compare(value.Val(), token.NEQ, constant.MakeInt64(17)) {
		t.Fatalf("captured constant = %v", value)
	}
	if parsed != 1 {
		t.Fatalf("fallback parsed %d files; only the one root should need source", parsed)
	}
	if stats := session.Stats(); stats.FallbackLoads != 1 || stats.PackageChecks != 0 {
		t.Fatalf("fallback stats: %+v", stats)
	}
}
