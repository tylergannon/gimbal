package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScope(t *testing.T) {
	for _, test := range []struct {
		name  string
		paths []string
		want  selection
	}{
		{"docs", []string{"AGENTS.md", "CHANGELOG.md", "skills/gimbal-release/SKILL.md"}, selection{Kind: "documentation", Packages: []string{}}},
		{"automation", []string{"scripts/pre-commit.mts", ".github/workflows/release.yml", "Justfile"}, selection{Kind: "automation", Release: true, Packages: []string{}}},
		{"runtime prompt", []string{"internal/builtin/prompt.md"}, selection{Kind: "application", Release: true, Packages: []string{}}},
		{"shared Go runtime and web", []string{"engine.go", "internal/envfile/file.go", "web/src/routes/+page.svelte"}, selection{Kind: "application", Release: true, Packages: []string{"./..."}, Web: true}},
		{"route Go", []string{"web/src/routes/projects/[project]/page.server.go", "web/src/routes/projects/[project]/runs/[runID]/page.server.go"}, selection{Kind: "application", Release: true, Packages: []string{"./internal/skgo/...", "./web/..."}, Web: true}},
		{"analyzer fixtures", []string{"internal/gimballint/testdata/src/setchecks/setchecks.go", "internal/gimballint/testdata/src/stub/go.mod"}, selection{Kind: "application", Release: true, Packages: []string{"./internal/gimballint"}}},
		{"dependencies", []string{"go.mod"}, selection{Kind: "application", Release: true, Packages: []string{"./..."}}},
		{"empty", nil, selection{Kind: "documentation", Packages: []string{}}},
		{"nested helper", []string{".agents/skills/df-promise/scripts/promise/main.go", ".agents/skills/df-promise/scripts/promise/go.mod"}, selection{Kind: "automation", Release: true, Packages: []string{}}},
		{"Go automation", []string{"internal/automation/ci-scope/main.go"}, selection{Kind: "automation", Release: true, Packages: []string{"./internal/automation/ci-scope"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := plan(test.paths)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %+v want %+v", got, test.want)
			}
		})
	}
}
func TestInstructionLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills/example/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("[missing](references/missing.md)")
	if checkLinks(root) == nil {
		t.Fatal("missing link accepted")
	}
	write("[external](https://example.com) [self](SKILL.md#section)")
	if err := checkLinks(root); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "out.md"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "out.md"), filepath.Join(filepath.Dir(source), "escape.md")); err != nil {
		t.Fatal(err)
	}
	write("[escape](escape.md)")
	if checkLinks(root) == nil {
		t.Fatal("outside symlink accepted")
	}
}
func TestWebPrerequisitesIncludeTestImports(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module github.com/tylergannon/gimbal\n\ngo 1.27\n")
	write("web/main.go", "package web\nimport \"embed\"\n//go:embed build\nvar Assets embed.FS\n")
	write("consumer/main.go", "package consumer\n")
	write("consumer/main_test.go", "package consumer\nimport _ \"github.com/tylergannon/gimbal/web\"\n")
	write("plain/main.go", "package plain\n")
	for _, test := range []struct {
		packages []string
		want     bool
	}{{[]string{"./consumer"}, true}, {[]string{"./..."}, true}, {[]string{"./plain"}, false}, {nil, false}} {
		got, err := needsWebAssets(root, test.packages)
		if err != nil || got != test.want {
			t.Fatalf("%v got %t error %v", test.packages, got, err)
		}
	}
}

func TestRecursiveRouteScopeReachesExecution(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module github.com/tylergannon/gimbal\n\ngo 1.27\n")
	write("web/main.go", "package web\n")
	write("web/route_test.go", "package web\nimport \"testing\"\nfunc TestRoute(t *testing.T) { t.Fatal(\"route check executed\") }\n")
	write("internal/skgo/main.go", "package skgo\n")
	write("scope.json", `{"go_packages":["./internal/skgo/...","./web/..."]}`)
	t.Chdir(root)
	raw, err := output(root, "go", "list", "./web/...")
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Fatal("fixture has no web package")
	}
	if err := execute([]string{"--go-prerequisites", "--plan", "scope.json"}); err != nil {
		t.Fatal(err)
	}
	if err := execute([]string{"--run-go", "--plan", "scope.json"}); err == nil {
		t.Fatal("recursive scope skipped the failing route test")
	}
	needed, err := needsWebAssets(root, []string{"./internal/skgo/...", "./web/..."})
	if err != nil || !needed {
		t.Fatalf("recursive web prerequisite = %t, %v", needed, err)
	}
}
