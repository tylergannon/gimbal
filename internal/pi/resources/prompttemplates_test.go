package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubstituteArgs(t *testing.T) {
	cases := []struct {
		name    string
		content string
		args    []string
		want    string
	}{
		{"arguments", "Test: $ARGUMENTS", []string{"a", "b", "c"}, "Test: a b c"},
		{"at", "Test: $@", []string{"a", "b", "c"}, "Test: a b c"},
		{"no recursive", "$ARGUMENTS", []string{"$1", "$ARGUMENTS"}, "$1 $ARGUMENTS"},
		{"no recursive 2", "$@", []string{"$100", "$1"}, "$100 $1"},
		{"mixed", "$1: $ARGUMENTS", []string{"prefix", "a", "b"}, "prefix: prefix a b"},
		{"mixed at", "$1: $@", []string{"prefix", "a", "b"}, "prefix: prefix a b"},
		{"empty arguments", "Test: $ARGUMENTS", nil, "Test: "},
		{"empty at", "Test: $@", nil, "Test: "},
		{"empty numbered", "Test: $1", nil, "Test: "},
		{"multiple arguments", "$ARGUMENTS and $ARGUMENTS", []string{"a", "b"}, "a b and a b"},
		{"special chars", "$1 $2: $ARGUMENTS", []string{"arg100", "@user"}, "arg100 @user: arg100 @user"},
		{"out of range", "$1 $2 $3 $4 $5", []string{"a", "b"}, "a b   "},
		{"unicode", "$ARGUMENTS", []string{"日本語", "🎉", "café"}, "日本語 🎉 café"},
		{"newlines tabs", "$1 $2", []string{"line1\nline2", "tab\tthere"}, "line1\nline2 tab\tthere"},
		{"consecutive", "$1$2", []string{"a", "b"}, "ab"},
		{"quoted args", "$ARGUMENTS", []string{"first arg", "second arg"}, "first arg second arg"},
		{"single", "Test: $ARGUMENTS", []string{"only"}, "Test: only"},
		{"zero index", "$0", []string{"a", "b"}, ""},
		{"decimal", "$1.5", []string{"a"}, "a.5"},
		{"arguments word", "pre$ARGUMENTS", []string{"a", "b"}, "prea b"},
		{"at word", "pre$@", []string{"a", "b"}, "prea b"},
		{"empty middle", "$ARGUMENTS", []string{"a", "", "c"}, "a  c"},
		{"trailing spaces", "$ARGUMENTS", []string{"  leading  ", "trailing  "}, "  leading   trailing  "},
		{"non-matching", "$A $$ $ $ARGS", []string{"a"}, "$A $$ $ $ARGS"},
		{"case sensitive", "$arguments $Arguments $ARGUMENTS", []string{"a", "b"}, "$arguments $Arguments a b"},
		{"long list", "$ARGUMENTS", longArgs(100), joinArgs(longArgs(100))},
		{"multi digit", "$10 $12 $15", numberedArgs(15), "val9 val11 val14"},
		{"escaped dollar", `Price: \$100`, nil, `Price: \`},
		{"mixed wildcard", "$1: $@ ($ARGUMENTS)", []string{"first", "second", "third"}, "first: first second third (first second third)"},
		{"no placeholders", "Just plain text", []string{"a", "b"}, "Just plain text"},
		{"only placeholders", "$1 $2 $@", []string{"a", "b", "c"}, "a b a b c"},
		{"default missing", "List exactly ${1:-7} next steps", nil, "List exactly 7 next steps"},
		{"default all", "${@:-default}\n${ARGUMENTS:-default}", nil, "default\ndefault"},
		{"default all present", "${@:-default}\n${ARGUMENTS:-default}", []string{"This", "would", "be", "the", "arguments"}, "This would be the arguments\nThis would be the arguments"},
		{"default present", "List exactly ${1:-7} next steps", []string{"3"}, "List exactly 3 next steps"},
		{"default empty", "Mode: ${1:-brief}", []string{""}, "Mode: brief"},
		{"multiples", "${1:-7} ${2:-brief}", nil, "7 brief"},
		{"multiples one", "${1:-7} ${2:-brief}", []string{"3"}, "3 brief"},
		{"multiples two", "${1:-7} ${2:-brief}", []string{"3", "verbose"}, "3 verbose"},
		{"no recursive arg", "${1:-7}", []string{"$ARGUMENTS"}, "$ARGUMENTS"},
		{"no recursive default", "${1:-$ARGUMENTS}", []string{"a", "b"}, "a"},
		{"default literal", "${3:-$ARGUMENTS}", []string{"a", "b"}, "$ARGUMENTS"},
		{"default spaces", "${1:-seven steps}", nil, "seven steps"},
		{"default out of range", "${3:-fallback}", []string{"a", "b"}, "fallback"},
		{"mix default", "$1 ${2:-x} $ARGUMENTS", []string{"a"}, "a x a"},
		{"slice", "${@:2}", []string{"a", "b", "c", "d"}, "b c d"},
		{"slice 1", "${@:1}", []string{"a", "b", "c"}, "a b c"},
		{"slice length", "${@:2:2}", []string{"a", "b", "c", "d"}, "b c"},
		{"slice length 1", "${@:1:1}", []string{"a", "b", "c"}, "a"},
		{"slice out of range", "${@:99}", []string{"a", "b"}, ""},
		{"slice zero length", "${@:2:0}", []string{"a", "b", "c"}, ""},
		{"slice length over", "${@:2:99}", []string{"a", "b", "c"}, "b c"},
		{"slice then at", "${@:2} vs $@", []string{"a", "b", "c"}, "b c vs a b c"},
		{"slice no recursive", "${@:1}", []string{"${@:2}", "test"}, "${@:2} test"},
		{"slice mixed", "$1: ${@:2}", []string{"cmd", "arg1", "arg2"}, "cmd: arg1 arg2"},
		{"slice zero", "${@:0}", []string{"a", "b", "c"}, "a b c"},
		{"slice empty", "${@:2}", nil, ""},
		{"slice single", "${@:1}", []string{"only"}, "only"},
		{"slice middle", "Process ${@:2} with $1", []string{"tool", "file1", "file2"}, "Process file1 file2 with tool"},
		{"multiple slices", "${@:1:1} and ${@:2}", []string{"a", "b", "c"}, "a and b c"},
		{"quoted slices", "${@:2}", []string{"cmd", "first arg", "second arg"}, "first arg second arg"},
		{"unicode slice", "${@:1}", []string{"日本語", "🎉", "café"}, "日本語 🎉 café"},
		{"combined", "Run $1 on ${@:2:2}, then process $@", []string{"eslint", "file1.ts", "file2.ts", "file3.ts"}, "Run eslint on file1.ts file2.ts, then process eslint file1.ts file2.ts file3.ts"},
		{"no spacing", "prefix${@:2}suffix", []string{"a", "b", "c"}, "prefixb csuffix"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := SubstituteArgs(testCase.content, testCase.args)
			if got != testCase.want {
				t.Errorf("SubstituteArgs(%q, %v) = %q, want %q", testCase.content, testCase.args, got, testCase.want)
			}
		})
	}
}

func longArgs(count int) []string {
	args := make([]string, count)
	for i := range args {
		args[i] = "arg" + itoa(i)
	}
	return args
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

func numberedArgs(count int) []string {
	args := make([]string, count)
	for i := range args {
		args[i] = "val" + itoa(i)
	}
	return args
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestParseCommandArgs(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{`"first arg" second`, []string{"first arg", "second"}},
		{`'first arg' second`, []string{"first arg", "second"}},
		{`"double" 'single' "double again"`, []string{"double", "single", "double again"}},
		{"", nil},
		{"a  b   c", []string{"a", "b", "c"}},
		{"a\tb\tc", []string{"a", "b", "c"}},
		{`"" " "`, []string{" "}},
		{"$100 @user #tag", []string{"$100", "@user", "#tag"}},
		{"日本語 🎉 café", []string{"日本語", "🎉", "café"}},
		{"label-2\n\nHere is some description #2.", []string{"label-2", "Here", "is", "some", "description", "#2."}},
		{"a b c   ", []string{"a", "b", "c"}},
	}
	for _, testCase := range cases {
		got := ParseCommandArgs(testCase.input)
		if len(got) != len(testCase.want) {
			t.Errorf("ParseCommandArgs(%q) = %v, want %v", testCase.input, got, testCase.want)
			continue
		}
		for i := range got {
			if got[i] != testCase.want[i] {
				t.Errorf("ParseCommandArgs(%q)[%d] = %q, want %q", testCase.input, i, got[i], testCase.want[i])
			}
		}
	}
}

func TestParseAndSubstituteIntegration(t *testing.T) {
	args := ParseCommandArgs(`Button "onClick handler" "disabled support"`)
	got := SubstituteArgs("Create a React component named $1 with features: $ARGUMENTS", args)
	want := "Create a React component named Button with features: Button onClick handler disabled support"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandPromptTemplate(t *testing.T) {
	templates := []PromptTemplate{
		{
			Name:    "arg-test",
			Content: "- arg1: $1\n- rest: ${@:2}",
		},
	}
	got := ExpandPromptTemplate("/arg-test label-2\n\nHere is some description #2.", templates)
	want := "- arg1: label-2\n- rest: Here is some description #2."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got = ExpandPromptTemplate("/arg-test\nlabel-2", []PromptTemplate{{Name: "arg-test", Content: "arg1: $1"}})
	if got != "arg1: label-2" {
		t.Errorf("got %q", got)
	}

	if got := ExpandPromptTemplate("no slash", templates); got != "no slash" {
		t.Errorf("got %q", got)
	}
	if got := ExpandPromptTemplate("/missing", templates); got != "/missing" {
		t.Errorf("got %q", got)
	}
}

func TestLoadPromptTemplates(t *testing.T) {
	testDir := t.TempDir()
	writeFile(t, filepath.Join(testDir, "pr.md"), `---
description: Review PRs from URLs
argument-hint: "<PR-URL>"
---
You are given one or more GitHub PR URLs: $@`)
	writeFile(t, filepath.Join(testDir, "cl.md"), `---
description: Audit changelog entries before release
---
Audit changelog entries.`)
	writeFile(t, filepath.Join(testDir, "empty-hint.md"), `---
description: A command with empty hint
argument-hint: ""
---
Do something`)
	writeFile(t, filepath.Join(testDir, "plain.md"), "Valid prompt content.")

	result := LoadPromptTemplates(LoadPromptTemplatesOptions{
		Cwd:         testDir,
		AgentDir:    testDir,
		PromptPaths: []string{testDir},
	})
	byName := map[string]PromptTemplate{}
	for _, template := range result.Templates {
		byName[template.Name] = template
	}
	if pr := byName["pr"]; pr.ArgumentHint != "<PR-URL>" || pr.Description != "Review PRs from URLs" {
		t.Errorf("pr = %+v", pr)
	}
	if cl := byName["cl"]; cl.ArgumentHint != "" || cl.Description != "Audit changelog entries before release" {
		t.Errorf("cl = %+v", cl)
	}
	if empty := byName["empty-hint"]; empty.ArgumentHint != "" {
		t.Errorf("empty-hint = %+v", empty)
	}
	if plain := byName["plain"]; plain.Description != "Valid prompt content." {
		t.Errorf("plain = %+v", plain)
	}
}

func TestLoadPromptTemplatesInvalidYAML(t *testing.T) {
	testDir := t.TempDir()
	invalidPath := filepath.Join(testDir, "invalid.md")
	writeFile(t, invalidPath, "---\ndescription: Broken: unquoted colon\n---\nDo something.\n")
	writeFile(t, filepath.Join(testDir, "valid.md"), "Valid prompt content.")

	result := LoadPromptTemplates(LoadPromptTemplatesOptions{
		Cwd:         testDir,
		AgentDir:    testDir,
		PromptPaths: []string{testDir},
	})
	if len(result.Templates) != 1 || result.Templates[0].Name != "valid" {
		t.Fatalf("templates = %+v", result.Templates)
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Type == DiagnosticWarning && diagnostic.Path == invalidPath && diagnostic.Message != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
