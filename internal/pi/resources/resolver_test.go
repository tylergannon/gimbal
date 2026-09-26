package resources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

func writeSkill(t *testing.T, dir, name, description string) string {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\nBody\n"
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPackageManagerResolveAutoDiscovery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	agentDir := t.TempDir()
	cwd := t.TempDir()

	userSkill := writeSkill(t, filepath.Join(agentDir, "skills"), "user-skill", "A user skill.")
	projectSkill := writeSkill(t, filepath.Join(cwd, config.ConfigDirName, "skills"), "project-skill", "A project skill.")

	trusted := true
	settings := config.NewInMemorySettingsManager(config.Settings{}, config.CreateOptions{ProjectTrusted: &trusted})
	manager := NewDefaultPackageManager(cwd, agentDir, settings)

	resolved, err := manager.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	paths := map[string]bool{}
	for _, resource := range resolved.Skills {
		paths[canonicalizePath(resource.Path)] = true
	}
	if !paths[canonicalizePath(userSkill)] {
		t.Errorf("user skill not discovered: %v", paths)
	}
	if !paths[canonicalizePath(projectSkill)] {
		t.Errorf("project skill not discovered: %v", paths)
	}
}

func TestPackageManagerResolveSettingsSkillPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	agentDir := t.TempDir()
	cwd := t.TempDir()
	explicitDir := t.TempDir()
	explicitSkill := writeSkill(t, explicitDir, "explicit-skill", "An explicit skill.")

	settings := config.NewInMemorySettingsManager(
		config.Settings{"skills": []any{explicitDir}},
		config.CreateOptions{},
	)
	manager := NewDefaultPackageManager(cwd, agentDir, settings)

	resolved, err := manager.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	found := false
	for _, resource := range resolved.Skills {
		if canonicalizePath(resource.Path) == canonicalizePath(explicitSkill) && resource.Enabled {
			found = true
		}
	}
	if !found {
		t.Errorf("explicit skill not resolved: %+v", resolved.Skills)
	}
}

func TestPackageManagerPrecedenceProjectWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	agentDir := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(agentDir, "skills")
	projectDir := filepath.Join(cwd, config.ConfigDirName, "skills")
	userSkill := writeSkill(t, userDir, "shared", "User.")
	projectSkill := writeSkill(t, projectDir, "shared", "Project.")

	trusted := true
	settings := config.NewInMemorySettingsManager(config.Settings{}, config.CreateOptions{ProjectTrusted: &trusted})
	manager := NewDefaultPackageManager(cwd, agentDir, settings)
	resolved, err := manager.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	first := ""
	for _, resource := range resolved.Skills {
		if resource.Path == userSkill || resource.Path == projectSkill {
			first = resource.Path
			break
		}
	}
	if canonicalizePath(first) != canonicalizePath(projectSkill) {
		t.Errorf("first resolved skill = %q, want project %q", first, projectSkill)
	}
}

func TestResourceLoaderReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	agentDir := t.TempDir()
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(agentDir, "skills"), "loader-skill", "A loader skill.")
	if err := os.MkdirAll(filepath.Join(cwd, config.ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, config.ConfigDirName, "SYSTEM.md"), []byte("System instructions."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "AGENTS.md"), []byte("Project context."), 0o644); err != nil {
		t.Fatal(err)
	}

	trusted := true
	settings := config.NewInMemorySettingsManager(config.Settings{}, config.CreateOptions{ProjectTrusted: &trusted})
	loader := NewDefaultResourceLoader(DefaultResourceLoaderOptions{Cwd: cwd, AgentDir: agentDir, SettingsManager: settings})
	if err := loader.Reload(t.Context(), nil); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	skills := loader.GetSkills()
	if len(skills.Skills) != 1 || skills.Skills[0].Name != "loader-skill" {
		t.Errorf("skills = %+v", skills.Skills)
	}
	if prompt := loader.GetSystemPrompt(); prompt == nil || *prompt != "System instructions." {
		t.Errorf("system prompt = %v", prompt)
	}
	agentsFiles := loader.GetAgentsFiles()
	foundContext := false
	for _, file := range agentsFiles {
		if filepath.Base(file.Path) == "AGENTS.md" {
			foundContext = true
		}
	}
	if !foundContext {
		t.Errorf("agents files = %+v", agentsFiles)
	}
}
