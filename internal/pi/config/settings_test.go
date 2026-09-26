package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func writeSettings(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed
}

func newTestManager(t *testing.T, options CreateOptions) (manager *SettingsManager, agentDir, projectDir string) {
	t.Helper()
	base := t.TempDir()
	agentDir = filepath.Join(base, "agent")
	projectDir = filepath.Join(base, "project")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewSettingsManager(projectDir, agentDir, options), agentDir, projectDir
}

func TestSettingsManagerPreservesExternalSettings(t *testing.T) {
	_, agentDir, _ := newTestManager(t, CreateOptions{})
	settingsPath := filepath.Join(agentDir, "settings.json")
	writeSettings(t, settingsPath, map[string]any{"theme": "dark", "defaultModel": "claude-sonnet"})

	manager := NewSettingsManager(filepath.Dir(agentDir), agentDir, CreateOptions{})
	writeSettings(t, settingsPath, map[string]any{
		"theme": "dark", "defaultModel": "claude-sonnet",
		"enabledModels": []string{"claude-opus-4-5", "gpt-5.2-codex"},
	})
	manager.SetDefaultThinkingLevel(model.ThinkingHigh)
	manager.Flush()

	saved := readSettings(t, settingsPath)
	if saved["defaultThinkingLevel"] != "high" || saved["theme"] != "dark" || saved["defaultModel"] != "claude-sonnet" {
		t.Fatalf("saved = %+v", saved)
	}
	enabled, _ := saved["enabledModels"].([]any)
	if len(enabled) != 2 || enabled[0] != "claude-opus-4-5" {
		t.Fatalf("enabledModels not preserved: %+v", saved["enabledModels"])
	}
}

func TestSettingsManagerInMemoryChangeOverridesFileChange(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	settingsPath := filepath.Join(agentDir, "settings.json")
	writeSettings(t, settingsPath, map[string]any{"theme": "dark"})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})

	writeSettings(t, settingsPath, map[string]any{"theme": "dark", "defaultThinkingLevel": "low"})
	manager.SetDefaultThinkingLevel(model.ThinkingHigh)
	manager.Flush()

	saved := readSettings(t, settingsPath)
	if saved["defaultThinkingLevel"] != "high" {
		t.Fatalf("defaultThinkingLevel = %v; want high", saved["defaultThinkingLevel"])
	}
}

func TestSettingsManagerPackages(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	settingsPath := filepath.Join(agentDir, "settings.json")
	writeSettings(t, settingsPath, map[string]any{"extensions": []string{"/local/ext.ts", "./relative/ext.ts"}})

	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if len(manager.GetPackages()) != 0 {
		t.Fatalf("packages = %+v; want empty", manager.GetPackages())
	}
	extensions := manager.GetExtensionPaths()
	if len(extensions) != 2 || extensions[0] != "/local/ext.ts" {
		t.Fatalf("extensions = %+v", extensions)
	}

	writeSettings(t, settingsPath, map[string]any{"packages": []any{
		"npm:simple-pkg",
		map[string]any{"source": "npm:shitty-extensions", "extensions": []string{"extensions/oracle.ts"}, "skills": []string{}},
	}})
	manager = NewSettingsManager(projectDir, agentDir, CreateOptions{})
	packages := manager.GetPackages()
	if len(packages) != 2 {
		t.Fatalf("packages = %+v", packages)
	}
	if packages[0].Source != "npm:simple-pkg" || packages[0].Object {
		t.Fatalf("packages[0] = %+v", packages[0])
	}
	if !packages[1].Object || packages[1].Source != "npm:shitty-extensions" || len(packages[1].Extensions) != 1 || len(packages[1].Skills) != 0 {
		t.Fatalf("packages[1] = %+v", packages[1])
	}
}

func TestSettingsManagerReload(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	settingsPath := filepath.Join(agentDir, "settings.json")
	writeSettings(t, settingsPath, map[string]any{"theme": "dark", "extensions": []string{"/before.ts"}})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})

	writeSettings(t, settingsPath, map[string]any{"theme": "light", "extensions": []string{"/after.ts"}, "defaultModel": "claude-sonnet"})
	manager.Reload()

	if theme, _ := manager.GetTheme(); theme != "light" {
		t.Fatalf("theme = %q", theme)
	}
	if paths := manager.GetExtensionPaths(); len(paths) != 1 || paths[0] != "/after.ts" {
		t.Fatalf("extensions = %+v", paths)
	}
	if modelID, _ := manager.GetDefaultModel(); modelID != "claude-sonnet" {
		t.Fatalf("defaultModel = %q", modelID)
	}
}

func TestSettingsManagerKeepsPreviousSettingsOnInvalidReload(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	settingsPath := filepath.Join(agentDir, "settings.json")
	writeSettings(t, settingsPath, map[string]any{"theme": "dark"})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})

	if err := os.WriteFile(settingsPath, []byte("{ invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager.Reload()
	if theme, _ := manager.GetTheme(); theme != "dark" {
		t.Fatalf("theme = %q; want dark", theme)
	}
	errors := manager.DrainErrors()
	if len(errors) != 1 || errors[0].Scope != SettingsScopeGlobal || errors[0].Path != settingsPath {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestSettingsManagerThemeSetting(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"theme": "light/dark"})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})

	if _, ok := manager.GetTheme(); ok {
		t.Fatal("auto theme should not return a fixed theme")
	}
	if setting, _ := manager.GetThemeSetting(); setting != "light/dark" {
		t.Fatalf("theme setting = %q", setting)
	}
	manager.SetTheme("solarized-light/tokyo-night")
	manager.Flush()
	saved := readSettings(t, filepath.Join(agentDir, "settings.json"))
	if saved["theme"] != "solarized-light/tokyo-night" {
		t.Fatalf("theme = %v", saved["theme"])
	}
}

func TestSettingsManagerErrorTracking(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	globalPath := filepath.Join(agentDir, "settings.json")
	projectPath := filepath.Join(projectDir, ConfigDirName, "settings.json")
	if err := os.WriteFile(globalPath, []byte("{ invalid global json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("{ invalid project json"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	errors := manager.DrainErrors()
	if len(errors) != 2 {
		t.Fatalf("errors = %+v", errors)
	}
	if errors[0].Scope != SettingsScopeGlobal || errors[0].Path != globalPath {
		t.Fatalf("errors[0] = %+v", errors[0])
	}
	if errors[1].Scope != SettingsScopeProject || errors[1].Path != projectPath {
		t.Fatalf("errors[1] = %+v", errors[1])
	}
	if len(manager.DrainErrors()) != 0 {
		t.Fatal("second drain should be empty")
	}
}

func TestSettingsManagerProjectTrust(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"theme": "global"})
	writeSettings(t, filepath.Join(projectDir, ConfigDirName, "settings.json"), map[string]any{"theme": "project"})

	untrusted := false
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{ProjectTrusted: &untrusted})
	if manager.IsProjectTrusted() {
		t.Fatal("project should not be trusted")
	}
	if theme, _ := manager.GetTheme(); theme != "global" {
		t.Fatalf("theme = %q; want global", theme)
	}
	if len(manager.GetProjectSettings()) != 0 {
		t.Fatal("project settings should be empty")
	}

	manager.SetProjectTrusted(true)
	if !manager.IsProjectTrusted() {
		t.Fatal("project should be trusted")
	}
	if theme, _ := manager.GetTheme(); theme != "project" {
		t.Fatalf("theme = %q; want project", theme)
	}
}

func TestSettingsManagerRefusesUntrustedProjectWrite(t *testing.T) {
	untrusted := false
	_, agentDir, projectDir := newTestManager(t, CreateOptions{ProjectTrusted: &untrusted})
	projectPath := filepath.Join(projectDir, ConfigDirName, "settings.json")
	writeSettings(t, projectPath, map[string]any{"packages": []string{"npm:existing"}})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{ProjectTrusted: &untrusted})

	if err := manager.SetProjectPackages([]PackageSource{{Source: "npm:new"}}); err == nil || err.Error() != "project is not trusted; refusing to write project settings" {
		t.Fatalf("err = %v", err)
	}
	manager.Flush()
	if len(manager.GetProjectSettings()) != 0 {
		t.Fatal("project settings should stay empty")
	}
	saved := readSettings(t, projectPath)
	packages, _ := saved["packages"].([]any)
	if len(packages) != 1 || packages[0] != "npm:existing" {
		t.Fatalf("packages = %+v", saved["packages"])
	}
}

func TestSettingsManagerDefaultProjectTrust(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"defaultProjectTrust": "always"})
	writeSettings(t, filepath.Join(projectDir, ConfigDirName, "settings.json"), map[string]any{"defaultProjectTrust": "never"})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got := manager.GetDefaultProjectTrust(); got != ProjectTrustAlways {
		t.Fatalf("default trust = %q; want always", got)
	}

	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"defaultProjectTrust": "sometimes"})
	manager = NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got := manager.GetDefaultProjectTrust(); got != ProjectTrustAsk {
		t.Fatalf("default trust = %q; want ask", got)
	}
}

func TestSettingsManagerProjectDirCreation(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	if err := os.RemoveAll(filepath.Join(projectDir, ConfigDirName)); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"theme": "dark"})

	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if _, err := os.Stat(filepath.Join(projectDir, ConfigDirName)); !os.IsNotExist(err) {
		t.Fatal(".pi should not be created by reading")
	}
	if theme, _ := manager.GetTheme(); theme != "dark" {
		t.Fatalf("theme = %q", theme)
	}

	if err := manager.SetProjectPackages([]PackageSource{{Source: "npm:test-pkg"}}); err != nil {
		t.Fatal(err)
	}
	manager.Flush()
	if _, err := os.Stat(filepath.Join(projectDir, ConfigDirName, "settings.json")); err != nil {
		t.Fatalf("project settings not written: %v", err)
	}
}

func TestSettingsManagerTerminalCapabilityOverrides(t *testing.T) {
	get := func(terminal map[string]any) map[string]any {
		manager := NewInMemorySettingsManager(Settings{"terminal": terminal}, CreateOptions{})
		return manager.GetTerminalCapabilityOverrides()
	}
	got := get(map[string]any{"images": false, "trueColor": false, "hyperlinks": false})
	if _, present := got["images"]; !present || got["images"] != nil || got["trueColor"] != false || got["hyperlinks"] != false {
		t.Fatalf("overrides = %+v", got)
	}
	got = get(map[string]any{"images": "kitty", "trueColor": true, "hyperlinks": true})
	if got["images"] != "kitty" || got["trueColor"] != true || got["hyperlinks"] != true {
		t.Fatalf("overrides = %+v", got)
	}
	got = get(map[string]any{"images": "auto", "trueColor": "auto", "hyperlinks": "auto"})
	if len(got) != 0 {
		t.Fatalf("overrides = %+v; want empty", got)
	}
}

func TestSettingsManagerRetrySettings(t *testing.T) {
	got := NewInMemorySettingsManager(nil, CreateOptions{}).GetRetrySettings()
	want := RetrySettingsValue{Enabled: true, MaxRetries: 3, BaseDelayMs: 2000, MaxAgentDelayMs: 60000}
	if got != want {
		t.Fatalf("retry = %+v; want %+v", got, want)
	}
	manager := NewInMemorySettingsManager(Settings{"retry": map[string]any{
		"enabled": true, "maxRetries": 10, "baseDelayMs": 500, "maxAgentDelayMs": 5000,
	}}, CreateOptions{})
	want = RetrySettingsValue{Enabled: true, MaxRetries: 10, BaseDelayMs: 500, MaxAgentDelayMs: 5000}
	if got = manager.GetRetrySettings(); got != want {
		t.Fatalf("retry = %+v; want %+v", got, want)
	}
}

func TestSettingsManagerHTTPIdleTimeout(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got, err := manager.GetHTTPIdleTimeoutMs(); err != nil || got != DefaultHTTPIdleTimeoutMs {
		t.Fatalf("timeout = %d, %v; want default", got, err)
	}

	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"httpIdleTimeoutMs": 300000})
	writeSettings(t, filepath.Join(projectDir, ConfigDirName, "settings.json"), map[string]any{"httpIdleTimeoutMs": 0})
	manager = NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got, err := manager.GetHTTPIdleTimeoutMs(); err != nil || got != 0 {
		t.Fatalf("timeout = %d, %v; want 0", got, err)
	}

	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"httpIdleTimeoutMs": -1})
	writeSettings(t, filepath.Join(projectDir, ConfigDirName, "settings.json"), map[string]any{})
	manager = NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if _, err := manager.GetHTTPIdleTimeoutMs(); err == nil || !strings.Contains(err.Error(), "invalid httpIdleTimeoutMs setting") {
		t.Fatalf("err = %v", err)
	}
}

func TestSettingsManagerCacheWarming(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetCacheWarmingMode(); got != CacheWarmingStreaming {
		t.Fatalf("mode = %q; want streaming", got)
	}
	writeSettings(t, filepath.Join(projectDir, ConfigDirName, "settings.json"), map[string]any{"cacheWarming": "idle"})
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetCacheWarmingMode(); got != CacheWarmingStreaming {
		t.Fatalf("project mode leaked: %q", got)
	}
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"cacheWarming": "idle"})
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetCacheWarmingMode(); got != CacheWarmingIdle {
		t.Fatalf("mode = %q; want idle", got)
	}

	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	manager.SetCacheWarmingMode(CacheWarmingOff)
	manager.Flush()
	reloaded := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got := reloaded.GetCacheWarmingMode(); got != CacheWarmingOff {
		t.Fatalf("mode = %q; want off", got)
	}
}

func TestSettingsManagerExternalEditor(t *testing.T) {
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")
	manager := NewInMemorySettingsManager(Settings{"externalEditor": "code --wait"}, CreateOptions{})
	if got := manager.GetExternalEditorCommand(); got != "code --wait" {
		t.Fatalf("editor = %q", got)
	}
	if got := NewInMemorySettingsManager(nil, CreateOptions{}).GetExternalEditorCommand(); got != "vim" {
		t.Fatalf("editor = %q; want vim", got)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "emacs")
	if got := NewInMemorySettingsManager(nil, CreateOptions{}).GetExternalEditorCommand(); got != "emacs" {
		t.Fatalf("editor = %q; want emacs", got)
	}
	if got := externalEditorDefault("windows"); got != "notepad" {
		t.Fatalf("windows default = %q", got)
	}
	if got := externalEditorDefault("darwin"); got != "nano" {
		t.Fatalf("darwin default = %q", got)
	}
}

func TestSettingsManagerTuiMode(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if got := manager.GetTuiMode(); got != TuiModeRegular {
		t.Fatalf("mode = %q", got)
	}
	manager.SetTuiMode(TuiModeFullscreen)
	manager.Flush()
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetTuiMode(); got != TuiModeFullscreen {
		t.Fatalf("mode = %q", got)
	}

	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"tuiMode": "other"})
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetTuiMode(); got != TuiModeRegular {
		t.Fatalf("mode = %q", got)
	}
	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"uiMode": "fullscreen"})
	if got := NewSettingsManager(projectDir, agentDir, CreateOptions{}).GetTuiMode(); got != TuiModeRegular {
		t.Fatalf("mode = %q; old uiMode should be ignored", got)
	}
}

func TestSettingsManagerFullscreenSettings(t *testing.T) {
	_, agentDir, projectDir := newTestManager(t, CreateOptions{})
	manager := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if manager.GetFullscreenExitOutput() != FullscreenExitTranscript || manager.GetFullscreenScrollbar() != ScrollbarAuto || !manager.GetFullscreenCopyOnSelect() {
		t.Fatal("unexpected fullscreen defaults")
	}
	manager.SetFullscreenExitOutput(FullscreenExitResumeHint)
	manager.SetFullscreenScrollbar(ScrollbarHidden)
	manager.SetFullscreenCopyOnSelect(false)
	manager.Flush()

	reloaded := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if reloaded.GetFullscreenExitOutput() != FullscreenExitResumeHint || reloaded.GetFullscreenScrollbar() != ScrollbarHidden || reloaded.GetFullscreenCopyOnSelect() {
		t.Fatalf("fullscreen = %q, %q, %v", reloaded.GetFullscreenExitOutput(), reloaded.GetFullscreenScrollbar(), reloaded.GetFullscreenCopyOnSelect())
	}

	writeSettings(t, filepath.Join(agentDir, "settings.json"), map[string]any{"fullscreenExitOutput": "nothing", "fullscreenScrollbar": "sometimes"})
	invalid := NewSettingsManager(projectDir, agentDir, CreateOptions{})
	if invalid.GetFullscreenExitOutput() != FullscreenExitTranscript || invalid.GetFullscreenScrollbar() != ScrollbarAuto {
		t.Fatalf("invalid fullscreen = %q, %q", invalid.GetFullscreenExitOutput(), invalid.GetFullscreenScrollbar())
	}
}

func TestSettingsManagerOutputPad(t *testing.T) {
	manager := NewInMemorySettingsManager(nil, CreateOptions{})
	if got := manager.GetOutputPad(); got != 1 {
		t.Fatalf("outputPad = %d; want 1", got)
	}
	manager.SetOutputPad(0)
	manager.Flush()
	if got := manager.GetOutputPad(); got != 0 {
		t.Fatalf("outputPad = %d; want 0", got)
	}
	if got := NewInMemorySettingsManager(Settings{"outputPad": 2}, CreateOptions{}).GetOutputPad(); got != 1 {
		t.Fatalf("outputPad = %d; want 1", got)
	}
}

func TestSettingsManagerMermaid(t *testing.T) {
	manager := NewInMemorySettingsManager(nil, CreateOptions{})
	if got := manager.GetMermaidRenderingMode(); got != MermaidStreaming {
		t.Fatalf("mermaid = %q", got)
	}
	manager.SetMermaidRenderingMode(MermaidFinal)
	manager.Flush()
	if got := manager.GetMermaidRenderingMode(); got != MermaidFinal {
		t.Fatalf("mermaid = %q", got)
	}
	if got := NewInMemorySettingsManager(Settings{"markdown": map[string]any{"mermaid": "sometimes"}}, CreateOptions{}).GetMermaidRenderingMode(); got != MermaidStreaming {
		t.Fatalf("mermaid = %q", got)
	}
}

func TestSettingsManagerShellCommandPrefix(t *testing.T) {
	manager := NewInMemorySettingsManager(Settings{"shellCommandPrefix": "shopt -s expand_aliases"}, CreateOptions{})
	if got, _ := manager.GetShellCommandPrefix(); got != "shopt -s expand_aliases" {
		t.Fatalf("prefix = %q", got)
	}
	if _, ok := NewInMemorySettingsManager(nil, CreateOptions{}).GetShellCommandPrefix(); ok {
		t.Fatal("prefix should be absent")
	}
}

func TestSettingsManagerDefaultTools(t *testing.T) {
	manager := NewInMemorySettingsManager(Settings{"defaultTools": []string{}}, CreateOptions{})
	if got, ok := manager.GetDefaultTools(); !ok || got == nil || len(got) != 0 {
		t.Fatalf("defaultTools = %+v, %v; want empty present", got, ok)
	}
	if _, ok := NewInMemorySettingsManager(nil, CreateOptions{}).GetDefaultTools(); ok {
		t.Fatal("defaultTools should be absent")
	}
}

func TestSettingsManagerSessionDir(t *testing.T) {
	home := HomeDir()
	manager := NewInMemorySettingsManager(Settings{"sessionDir": "~/sessions"}, CreateOptions{})
	if got, _ := manager.GetSessionDir(); got != filepath.Join(home, "sessions") {
		t.Fatalf("sessionDir = %q", got)
	}
	if _, ok := NewInMemorySettingsManager(nil, CreateOptions{}).GetSessionDir(); ok {
		t.Fatal("sessionDir should be absent")
	}
}

func TestSettingsManagerShellPath(t *testing.T) {
	home := HomeDir()
	manager := NewInMemorySettingsManager(Settings{"shellPath": "~/.local/bin/agent-shell-sandbox"}, CreateOptions{})
	if got, _ := manager.GetShellPath(); got != filepath.Join(home, ".local/bin/agent-shell-sandbox") {
		t.Fatalf("shellPath = %q", got)
	}
	if got, _ := NewInMemorySettingsManager(Settings{"shellPath": "/bin/zsh"}, CreateOptions{}).GetShellPath(); got != "/bin/zsh" {
		t.Fatalf("shellPath = %q", got)
	}
	if got, _ := NewInMemorySettingsManager(Settings{"shellPath": "~"}, CreateOptions{}).GetShellPath(); got != home {
		t.Fatalf("shellPath = %q; want %q", got, home)
	}
}

func TestSettingsManagerMigratesLegacyFields(t *testing.T) {
	manager := NewInMemorySettingsManager(Settings{
		"queueMode":  "all",
		"websockets": false,
		"retry":      map[string]any{"maxDelayMs": 1234},
	}, CreateOptions{})
	if got := manager.GetSteeringMode(); got != "all" {
		t.Fatalf("steering = %q", got)
	}
	if got := manager.GetTransport(); got != model.TransportSSE {
		t.Fatalf("transport = %q", got)
	}
	if got := manager.GetProviderRetrySettings(); got.MaxRetryDelayMs != 1234 {
		t.Fatalf("maxRetryDelayMs = %d", got.MaxRetryDelayMs)
	}
}
