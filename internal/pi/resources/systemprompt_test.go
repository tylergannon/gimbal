package resources

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func testSkill() Skill {
	return Skill{
		Name:        "test-skill",
		Description: "A test skill.",
		FilePath:    "/skills/test-skill/SKILL.md",
		BaseDir:     "/skills/test-skill",
		SourceInfo:  CreateSyntheticSourceInfo("/skills/test-skill/SKILL.md", SyntheticSourceInfoOptions{Source: "test"}),
	}
}

func buildPrompt(t *testing.T, options BuildSystemPromptOptions) string {
	t.Helper()
	prompt, err := BuildSystemPrompt(options)
	if err != nil {
		t.Fatalf("BuildSystemPrompt: %v", err)
	}
	return prompt
}

func TestBuildSystemPromptEmptyTools(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{SelectedTools: []string{}, Cwd: "/tmp"})
	if !strings.Contains(prompt, "<tools>\n(none)\n") {
		t.Errorf("prompt missing (none):\n%s", prompt)
	}
	if !strings.Contains(prompt, "Show file paths clearly") {
		t.Errorf("prompt missing file paths guideline:\n%s", prompt)
	}
}

func TestBuildSystemPromptPrefixes(t *testing.T) {
	defaultPrompt := buildPrompt(t, BuildSystemPromptOptions{Cwd: "/tmp", SelectedTools: []string{}})
	if !strings.HasPrefix(defaultPrompt, "You are an expert coding assistant operating inside pi") {
		t.Errorf("default prompt prefix = %q", defaultPrompt)
	}

	customPrompt := buildPrompt(t, BuildSystemPromptOptions{CustomPrompt: "You are Exact.", Cwd: "/tmp", SelectedTools: []string{}})
	if !strings.HasPrefix(customPrompt, "You are Exact.\n\n<cwd>") {
		t.Errorf("custom prompt prefix = %q", customPrompt)
	}
}

func TestBuildSystemPromptForced(t *testing.T) {
	forced := "exact"
	prompt := buildPrompt(t, BuildSystemPromptOptions{ForceSystemPrompt: &forced, Cwd: "/tmp"})
	if prompt != "exact" {
		t.Errorf("prompt = %q, want exact", prompt)
	}
}

func TestBuildSystemPromptSections(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{
		CustomPrompt:       "You are Exact.",
		AppendSystemPrompt: "Additional instructions.",
		ContextFiles:       []ContextFile{{Path: "/tmp/AGENTS.md", Content: "Project instructions."}},
		SelectedTools:      []string{},
		Cwd:                "/tmp",
	})
	for _, want := range []string{
		"<addendum>\nAdditional instructions.\n</addendum>",
		"<project_context>\nProject-specific instructions and guidelines:\n\n<project_instructions path=\"/tmp/AGENTS.md\">",
		"<cwd>\n/tmp\n</cwd>",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildSystemPromptDefaultTools(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{
		ToolSnippets: map[string]string{
			"read":  "Read file contents",
			"bash":  "Execute bash commands",
			"edit":  "Make surgical edits",
			"write": "Create or overwrite files",
		},
		Cwd: "/tmp",
	})
	for _, want := range []string{"- read:", "- bash:", "- edit:", "- write:"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildSystemPromptShellGuidance(t *testing.T) {
	cases := []struct {
		tools []string
		want  string
	}{
		{[]string{"powershell"}, "Use PowerShell for file operations"},
		{[]string{"bash", "powershell"}, "Use bash or PowerShell for file operations"},
	}
	for _, testCase := range cases {
		prompt := buildPrompt(t, BuildSystemPromptOptions{SelectedTools: testCase.tools, Cwd: "/tmp"})
		if !strings.Contains(prompt, testCase.want) {
			t.Errorf("tools %v: prompt missing %q:\n%s", testCase.tools, testCase.want, prompt)
		}
	}
}

func TestBuildSystemPromptDocsSection(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{Cwd: "/tmp"})
	if !strings.Contains(prompt, "- When reading pi docs or examples, resolve docs/... under Additional docs and examples/... under Examples, not the current working directory") {
		t.Errorf("prompt missing docs resolution line:\n%s", prompt)
	}
	if !strings.Contains(prompt, "environment variables (docs/environment-variables.md)") {
		t.Errorf("prompt missing environment variables line:\n%s", prompt)
	}
}

func TestBuildSystemPromptCustomToolSnippets(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{
		SelectedTools: []string{"read", "dynamic_tool"},
		ToolSnippets:  map[string]string{"dynamic_tool": "Run dynamic test behavior"},
		Cwd:           "/tmp",
	})
	if !strings.Contains(prompt, "- dynamic_tool: Run dynamic test behavior") {
		t.Errorf("prompt missing dynamic tool:\n%s", prompt)
	}

	withoutSnippet := buildPrompt(t, BuildSystemPromptOptions{
		SelectedTools: []string{"read", "dynamic_tool"},
		Cwd:           "/tmp",
	})
	if strings.Contains(withoutSnippet, "dynamic_tool") {
		t.Errorf("prompt unexpectedly names dynamic_tool:\n%s", withoutSnippet)
	}
}

func TestBuildSystemPromptPromptGuidelines(t *testing.T) {
	prompt := buildPrompt(t, BuildSystemPromptOptions{
		SelectedTools:    []string{"read", "dynamic_tool"},
		PromptGuidelines: []string{"Use dynamic_tool for project summaries."},
		Cwd:              "/tmp",
	})
	if !strings.Contains(prompt, "- Use dynamic_tool for project summaries.") {
		t.Errorf("prompt missing guideline:\n%s", prompt)
	}

	deduped := buildPrompt(t, BuildSystemPromptOptions{
		SelectedTools:    []string{"read"},
		PromptGuidelines: []string{"Use dynamic_tool for summaries.", "  Use dynamic_tool for summaries.  ", "   "},
		Cwd:              "/tmp",
	})
	if strings.Count(deduped, "- Use dynamic_tool for summaries.") != 1 {
		t.Errorf("guideline count = %d", strings.Count(deduped, "- Use dynamic_tool for summaries."))
	}
}

func TestBuildSystemPromptSkills(t *testing.T) {
	cases := []struct {
		name         string
		customPrompt string
	}{
		{"default prompt", ""},
		{"custom prompt", "Custom system prompt"},
	}
	for _, testCase := range cases {
		prompt := buildPrompt(t, BuildSystemPromptOptions{
			CustomPrompt:  testCase.customPrompt,
			SelectedTools: []string{"bash"},
			Skills:        []Skill{testSkill()},
			Cwd:           "/tmp",
		})
		for _, want := range []string{"<skills>", "<available_skills>", "<name>test-skill</name>", "Use bash to load a skill's file"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s: prompt missing %q:\n%s", testCase.name, want, prompt)
			}
		}
	}

	prompt := buildPrompt(t, BuildSystemPromptOptions{
		SelectedTools: []string{"write"},
		Skills:        []Skill{testSkill()},
		Cwd:           "/tmp",
	})
	if strings.Contains(prompt, "<available_skills>") {
		t.Errorf("skills leaked without read/bash:\n%s", prompt)
	}
}

func TestDiffSystemPromptSections(t *testing.T) {
	one, two := "one", "two"
	previous := model.SystemSections{{Name: "a", Value: &one}, {Name: "b", Value: &two}}
	current := model.SystemSections{{Name: "a", Value: &one}, {Name: "c", Value: &two}}
	patch, changed := DiffSystemPromptSections(previous, current)
	if !changed {
		t.Fatal("expected a changed patch")
	}
	if value, ok := patch.Get("c"); !ok || value == nil || *value != "two" {
		t.Errorf("patch c = %v", value)
	}
	if value, ok := patch.Get("b"); !ok || value != nil {
		t.Errorf("patch b should remove: %v", value)
	}
	if _, ok := patch.Get("a"); ok {
		t.Errorf("patch should not include unchanged a")
	}

	if _, changed := DiffSystemPromptSections(current, current); changed {
		t.Errorf("expected no change")
	}
}
