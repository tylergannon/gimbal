package session

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestResolveToolSelection(t *testing.T) {
	settings := config.NewInMemorySettingsManager(config.Settings{"defaultTools": []any{"grep", "find"}}, config.CreateOptions{})

	initial, allowed := resolveToolSelection(settings, nil, nil, "", nil)
	if allowed != nil {
		t.Fatalf("allowed = %v, want nil", allowed)
	}
	if strings.Join(initial, ",") != "grep,find" {
		t.Fatalf("initial = %v, want [grep find]", initial)
	}

	initial, allowed = resolveToolSelection(settings, []string{"read"}, nil, "", nil)
	if strings.Join(initial, ",") != "read" || allowed == nil || strings.Join(allowed, ",") != "read" {
		t.Fatalf("allowlist initial=%v allowed=%v", initial, allowed)
	}

	initial, _ = resolveToolSelection(settings, []string{"read", "grep"}, []string{"read"}, "", nil)
	if strings.Join(initial, ",") != "grep" {
		t.Fatalf("excluded initial = %v, want [grep]", initial)
	}

	initial, allowed = resolveToolSelection(settings, nil, nil, "all", nil)
	if len(initial) != 0 || allowed == nil || len(allowed) != 0 {
		t.Fatalf("noTools all initial=%v allowed=%v", initial, allowed)
	}

	custom := []model.ToolDefinition{{Name: "sdk_tool"}}
	initial, _ = resolveToolSelection(settings, nil, nil, "builtin", custom)
	if strings.Join(initial, ",") != "sdk_tool" {
		t.Fatalf("noTools builtin initial = %v, want [sdk_tool]", initial)
	}
}

func TestDefaultToolsSetting(t *testing.T) {
	settings := config.NewInMemorySettingsManager(config.Settings{"defaultTools": []any{"grep", "find"}}, config.CreateOptions{})
	session := createTestSession(t, settings, FromServicesOptions{Model: testModel()})

	var names []string
	for _, tool := range session.GetAllTools() {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	want := []string{"bash", "edit", "find", "grep", "ls", "read", "write"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("all tools = %v, want %v", names, want)
	}
	if strings.Join(session.GetActiveToolNames(), ",") != "grep,find" {
		t.Fatalf("active = %v, want [grep find]", session.GetActiveToolNames())
	}
	if !strings.Contains(session.SystemPrompt(), "- grep:") {
		t.Fatalf("system prompt missing grep snippet:\n%s", session.SystemPrompt())
	}
	if strings.Contains(session.SystemPrompt(), "- read:") {
		t.Fatalf("system prompt unexpectedly has read snippet")
	}
}

func createTestSession(t *testing.T, settings *config.SettingsManager, options FromServicesOptions) *Session {
	t.Helper()
	cwd := t.TempDir()
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory manager: %v", err)
	}
	services := &Services{Cwd: cwd, Settings: settings}
	options.Services = services
	options.SessionManager = manager
	if options.StreamFn == nil {
		options.StreamFn = doneStream(testAssistant("summary", model.StopStop))
	}
	result, err := CreateFromServices(context.Background(), options)
	if err != nil {
		t.Fatalf("create from services: %v", err)
	}
	t.Cleanup(result.Session.Dispose)
	return result.Session
}

func TestCustomToolsEnabled(t *testing.T) {
	settings := config.NewInMemorySettingsManager(config.Settings{"defaultTools": []any{"grep"}}, config.CreateOptions{})
	custom := []model.ToolDefinition{{
		Name: "sdk_tool", Label: "SDK Tool", Description: "SDK custom tool", Parameters: model.Object(),
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "ok"}}, Details: map[string]any{}}, nil
		},
	}}
	session := createTestSession(t, settings, FromServicesOptions{Model: testModel(), CustomTools: custom})

	active := session.GetActiveToolNames()
	sort.Strings(active)
	if strings.Join(active, ",") != "grep,sdk_tool" {
		t.Fatalf("active = %v, want [grep sdk_tool]", active)
	}
	if _, ok := session.GetToolDefinition("sdk_tool"); !ok {
		t.Fatalf("sdk_tool not registered")
	}
}

func TestNoToolsAllDisablesEverything(t *testing.T) {
	settings := config.NewInMemorySettingsManager(config.Settings{"defaultTools": []any{"read"}}, config.CreateOptions{})
	session := createTestSession(t, settings, FromServicesOptions{Model: testModel(), NoTools: "all"})
	if len(session.GetAllTools()) != 0 {
		t.Fatalf("all tools = %v, want none", session.GetAllTools())
	}
	if len(session.GetActiveToolNames()) != 0 {
		t.Fatalf("active = %v, want none", session.GetActiveToolNames())
	}
}
