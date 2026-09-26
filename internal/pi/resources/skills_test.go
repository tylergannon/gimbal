package resources

import (
	"path/filepath"
	"strings"
	"testing"
)

func skillsFixture(name string) string {
	return absPath(filepath.Join("testdata", "skills", name))
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func loadSkillFixture(t *testing.T, name string) LoadSkillsResult {
	t.Helper()
	return LoadSkillsFromDir(LoadSkillsFromDirOptions{Dir: skillsFixture(name), Source: "test"})
}

func hasDiagnosticMessage(result LoadSkillsResult, substring string) bool {
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, substring) {
			return true
		}
	}
	return false
}

func TestLoadSkillsFromDirValidity(t *testing.T) {
	result := loadSkillFixture(t, "valid-skill")
	if len(result.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(result.Skills))
	}
	if result.Skills[0].Name != "valid-skill" {
		t.Errorf("name = %q", result.Skills[0].Name)
	}
	if result.Skills[0].Description != "A valid skill for testing purposes." {
		t.Errorf("description = %q", result.Skills[0].Description)
	}
	if result.Skills[0].SourceInfo.Source != "test" {
		t.Errorf("source = %q", result.Skills[0].SourceInfo.Source)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirNameMismatchAllowed(t *testing.T) {
	result := loadSkillFixture(t, "name-mismatch")
	if len(result.Skills) != 1 || result.Skills[0].Name != "different-name" {
		t.Fatalf("result = %+v", result.Skills)
	}
	if hasDiagnosticMessage(result, "does not match parent directory") {
		t.Errorf("unexpected mismatch diagnostic: %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirWarnings(t *testing.T) {
	cases := []struct {
		fixture string
		message string
	}{
		{"invalid-name-chars", "invalid characters"},
		{"long-name", "exceeds 64 characters"},
		{"consecutive-hyphens", "consecutive hyphens"},
	}
	for _, testCase := range cases {
		result := loadSkillFixture(t, testCase.fixture)
		if len(result.Skills) != 1 {
			t.Errorf("%s: skills = %d, want 1", testCase.fixture, len(result.Skills))
		}
		if !hasDiagnosticMessage(result, testCase.message) {
			t.Errorf("%s: missing diagnostic %q: %+v", testCase.fixture, testCase.message, result.Diagnostics)
		}
	}
}

func TestLoadSkillsFromDirMissingDescription(t *testing.T) {
	result := loadSkillFixture(t, "missing-description")
	if len(result.Skills) != 0 {
		t.Fatalf("skills = %d, want 0", len(result.Skills))
	}
	if !hasDiagnosticMessage(result, "description is required") {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirUnknownField(t *testing.T) {
	result := loadSkillFixture(t, "unknown-field")
	if len(result.Skills) != 1 || len(result.Diagnostics) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestLoadSkillsFromDirNested(t *testing.T) {
	result := loadSkillFixture(t, "nested")
	if len(result.Skills) != 1 || result.Skills[0].Name != "child-skill" {
		t.Fatalf("result = %+v", result.Skills)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirRootPreferred(t *testing.T) {
	result := loadSkillFixture(t, "root-skill-preferred")
	if len(result.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(result.Skills))
	}
	if result.Skills[0].Name != "root-skill-preferred" || result.Skills[0].Description != "Root skill should win." {
		t.Errorf("skill = %+v", result.Skills[0])
	}
}

func TestLoadSkillsFromDirNoFrontmatter(t *testing.T) {
	result := loadSkillFixture(t, "no-frontmatter")
	if len(result.Skills) != 0 {
		t.Fatalf("skills = %d, want 0", len(result.Skills))
	}
	if !hasDiagnosticMessage(result, "description is required") {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirInvalidYAML(t *testing.T) {
	result := loadSkillFixture(t, "invalid-yaml")
	if len(result.Skills) != 0 {
		t.Fatalf("skills = %d, want 0", len(result.Skills))
	}
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected an invalid-YAML diagnostic")
	}
}

func TestLoadSkillsFromDirMultilineDescription(t *testing.T) {
	result := loadSkillFixture(t, "multiline-description")
	if len(result.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(result.Skills))
	}
	if !strings.Contains(result.Skills[0].Description, "\n") {
		t.Errorf("description = %q", result.Skills[0].Description)
	}
	if !strings.Contains(result.Skills[0].Description, "This is a multiline description.") {
		t.Errorf("description = %q", result.Skills[0].Description)
	}
}

func TestLoadSkillsFromDirNonExistent(t *testing.T) {
	result := LoadSkillsFromDir(LoadSkillsFromDirOptions{Dir: "/non/existent/path", Source: "test"})
	if len(result.Skills) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestLoadSkillsFromDirDisableModelInvocation(t *testing.T) {
	result := loadSkillFixture(t, "disable-model-invocation")
	if len(result.Skills) != 1 || !result.Skills[0].DisableModelInvocation {
		t.Fatalf("result = %+v", result.Skills)
	}
	if hasDiagnosticMessage(result, "unknown frontmatter field") {
		t.Errorf("unexpected diagnostic: %+v", result.Diagnostics)
	}
}

func TestLoadSkillsFromDirAllSkills(t *testing.T) {
	result := LoadSkillsFromDir(LoadSkillsFromDirOptions{Dir: skillsFixture("."), Source: "test"})
	if len(result.Skills) < 6 {
		t.Errorf("skills = %d, want >= 6", len(result.Skills))
	}
}

func TestFormatSkillsForPrompt(t *testing.T) {
	if result := FormatSkillsForPrompt(nil); result != "" {
		t.Errorf("empty skills = %q, want empty", result)
	}

	skills := []Skill{
		{Name: "test-skill", Description: "A test skill.", FilePath: "/path/to/skill/SKILL.md", BaseDir: "/path/to/skill"},
	}
	result := FormatSkillsForPrompt(skills)
	for _, want := range []string{
		"<available_skills>", "</available_skills>", "<skill>",
		"<name>test-skill</name>", "<description>A test skill.</description>",
		"<location>/path/to/skill/SKILL.md</location>",
		"The following skills provide specialized instructions",
		"Use the read tool to load a skill's file",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("result missing %q:\n%s", want, result)
		}
	}

	escaped := FormatSkillsForPrompt([]Skill{{
		Name: "test-skill", Description: `A skill with <special> & "characters".`, FilePath: "/x/SKILL.md",
	}})
	for _, want := range []string{"&lt;special&gt;", "&amp;", "&quot;characters&quot;"} {
		if !strings.Contains(escaped, want) {
			t.Errorf("escaped result missing %q", want)
		}
	}

	hidden := []Skill{
		{Name: "visible-skill", Description: "A visible skill.", FilePath: "/visible/SKILL.md"},
		{Name: "hidden-skill", Description: "A hidden skill.", FilePath: "/hidden/SKILL.md", DisableModelInvocation: true},
	}
	visibleResult := FormatSkillsForPrompt(hidden)
	if !strings.Contains(visibleResult, "visible-skill") || strings.Contains(visibleResult, "hidden-skill") {
		t.Errorf("hidden skill leak: %s", visibleResult)
	}
	if strings.Count(visibleResult, "<skill>") != 1 {
		t.Errorf("<skill> count = %d", strings.Count(visibleResult, "<skill>"))
	}

	allHidden := FormatSkillsForPrompt([]Skill{{Name: "hidden", Description: "x", FilePath: "/x", DisableModelInvocation: true}})
	if allHidden != "" {
		t.Errorf("all hidden = %q, want empty", allHidden)
	}
}

func TestLoadSkillsExplicitPaths(t *testing.T) {
	emptyAgent := filepath.Join("testdata", "empty-agent")
	emptyCwd := filepath.Join("testdata", "empty-cwd")

	result := LoadSkills(LoadSkillsOptions{
		AgentDir:        emptyAgent,
		Cwd:             emptyCwd,
		SkillPaths:      []string{skillsFixture("valid-skill")},
		IncludeDefaults: true,
	})
	if len(result.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(result.Skills))
	}
	if result.Skills[0].SourceInfo.Scope != ScopeTemporary {
		t.Errorf("scope = %q, want temporary", result.Skills[0].SourceInfo.Scope)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsMissingPath(t *testing.T) {
	result := LoadSkills(LoadSkillsOptions{
		AgentDir:        filepath.Join("testdata", "empty-agent"),
		Cwd:             filepath.Join("testdata", "empty-cwd"),
		SkillPaths:      []string{"/non/existent/path"},
		IncludeDefaults: true,
	})
	if len(result.Skills) != 0 {
		t.Fatalf("skills = %d, want 0", len(result.Skills))
	}
	if !hasDiagnosticMessage(result, "does not exist") {
		t.Errorf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestLoadSkillsCollisionKeepsFirst(t *testing.T) {
	result := LoadSkills(LoadSkillsOptions{
		AgentDir:        filepath.Join("testdata", "empty-agent"),
		Cwd:             filepath.Join("testdata", "empty-cwd"),
		SkillPaths:      []string{absPath(filepath.Join("testdata", "skills-collision", "first")), absPath(filepath.Join("testdata", "skills-collision", "second"))},
		IncludeDefaults: true,
	})
	if len(result.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(result.Skills))
	}
	if result.Skills[0].SourceInfo.Source != "local" {
		t.Errorf("source = %q", result.Skills[0].SourceInfo.Source)
	}
	foundCollision := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Type == DiagnosticCollision {
			foundCollision = true
		}
	}
	if !foundCollision {
		t.Errorf("expected collision diagnostic: %+v", result.Diagnostics)
	}
}
