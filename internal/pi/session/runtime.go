package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/resources"
)

// This file ports sdk.ts, agent-session-services.ts and the runtime assembly
// portion of agent-session-runtime.ts. TUI, HTML export, RPC and extension
// execution are omitted.

// DefaultThinkingLevel is pi's DEFAULT_THINKING_LEVEL.
const DefaultThinkingLevel = model.ThinkingMedium

// Diagnostic is one non-fatal issue collected while creating services or a
// session.
type Diagnostic struct {
	Type    string // "info", "warning" or "error"
	Message string
}

// ServicesOptions configures cwd-bound runtime services.
type ServicesOptions struct {
	Cwd                   string
	AgentDir              string
	Settings              *config.SettingsManager
	ResourceLoader        *resources.DefaultResourceLoader
	ResourceLoaderOptions *resources.DefaultResourceLoaderOptions
}

// Services are the cwd-bound runtime services for one session.
type Services struct {
	Cwd            string
	AgentDir       string
	Settings       *config.SettingsManager
	ResourceLoader *resources.DefaultResourceLoader
	Diagnostics    []Diagnostic
}

// CreateServices creates cwd-bound services and reloads the resource loader.
func CreateServices(ctx context.Context, options ServicesOptions) (*Services, error) {
	cwd := config.ResolvePath(options.Cwd, ".")
	agentDir := options.AgentDir
	if agentDir == "" {
		agentDir = config.GetAgentDir()
	} else {
		agentDir = config.ResolvePath(agentDir, ".")
	}
	settings := options.Settings
	if settings == nil {
		settings = config.NewSettingsManager(cwd, agentDir, config.CreateOptions{})
	}
	loader := options.ResourceLoader
	if loader == nil {
		loaderOptions := resources.DefaultResourceLoaderOptions{Cwd: cwd, AgentDir: agentDir, SettingsManager: settings}
		if options.ResourceLoaderOptions != nil {
			loaderOptions = *options.ResourceLoaderOptions
			if loaderOptions.Cwd == "" {
				loaderOptions.Cwd = cwd
			}
			if loaderOptions.AgentDir == "" {
				loaderOptions.AgentDir = agentDir
			}
			if loaderOptions.SettingsManager == nil {
				loaderOptions.SettingsManager = settings
			}
		}
		loader = resources.NewDefaultResourceLoader(loaderOptions)
	}
	if err := loader.Reload(ctx, nil); err != nil {
		return nil, err
	}
	return &Services{Cwd: cwd, AgentDir: agentDir, Settings: settings, ResourceLoader: loader}, nil
}

// FromServicesOptions configures creating a session from existing services.
type FromServicesOptions struct {
	Services       *Services
	SessionManager *history.SessionManager

	Model             *model.Model
	AvailableModels   []*model.Model
	GetModel          func(provider, modelID string) *model.Model
	HasConfiguredAuth func(provider string) bool

	ThinkingLevel model.ThinkingLevel
	ScopedModels  []ScopedModel

	Tools        []string
	ExcludeTools []string
	NoTools      string
	CustomTools  []model.ToolDefinition
	BaseTools    map[string]model.AgentTool

	StreamFn model.StreamFunction
	APIKey   string
	Headers  model.ProviderHeaders
	Env      model.ProviderEnv
}

// CreateResult is the outcome of creating a session.
type CreateResult struct {
	Session              *Session
	ModelFallbackMessage string
}

// CreateOptions is the full single-call creation surface.
type CreateOptions struct {
	ServicesOptions

	SessionManager    *history.SessionManager
	Model             *model.Model
	AvailableModels   []*model.Model
	GetModel          func(provider, modelID string) *model.Model
	HasConfiguredAuth func(provider string) bool

	ThinkingLevel model.ThinkingLevel
	ScopedModels  []ScopedModel

	Tools        []string
	ExcludeTools []string
	NoTools      string
	CustomTools  []model.ToolDefinition
	BaseTools    map[string]model.AgentTool

	StreamFn model.StreamFunction
	APIKey   string
	Headers  model.ProviderHeaders
	Env      model.ProviderEnv
}

// Create creates cwd-bound services and then a session from them.
func Create(ctx context.Context, options CreateOptions) (*CreateResult, error) {
	services, err := CreateServices(ctx, options.ServicesOptions)
	if err != nil {
		return nil, err
	}
	return CreateFromServices(ctx, FromServicesOptions{
		Services:          services,
		SessionManager:    options.SessionManager,
		Model:             options.Model,
		AvailableModels:   options.AvailableModels,
		GetModel:          options.GetModel,
		HasConfiguredAuth: options.HasConfiguredAuth,
		ThinkingLevel:     options.ThinkingLevel,
		ScopedModels:      options.ScopedModels,
		Tools:             options.Tools,
		ExcludeTools:      options.ExcludeTools,
		NoTools:           options.NoTools,
		CustomTools:       options.CustomTools,
		BaseTools:         options.BaseTools,
		StreamFn:          options.StreamFn,
		APIKey:            options.APIKey,
		Headers:           options.Headers,
		Env:               options.Env,
	})
}

// CreateFromServices builds a session from already-created services.
func CreateFromServices(ctx context.Context, options FromServicesOptions) (*CreateResult, error) {
	if options.Services == nil {
		return nil, errors.New("session: services are required")
	}
	services := options.Services
	sessionManager := options.SessionManager
	if sessionManager == nil {
		manager, err := history.Create(services.Cwd, history.DefaultSessionDir(services.Cwd, services.AgentDir), nil)
		if err != nil {
			return nil, err
		}
		sessionManager = manager
	}

	existing := sessionManager.BuildSessionContext()
	hasExisting := len(existing.Messages) > 0
	hasThinkingEntry := false
	for _, entry := range sessionManager.GetBranch() {
		if entry.EntryType() == "thinking_level_change" {
			hasThinkingEntry = true
			break
		}
	}

	selectedModel := options.Model
	fallbackMessage := ""
	if selectedModel == nil && hasExisting && existing.Model != nil {
		if options.GetModel != nil {
			restored := options.GetModel(existing.Model.Provider, existing.Model.ModelID)
			if restored != nil && (options.HasConfiguredAuth == nil || options.HasConfiguredAuth(existing.Model.Provider)) {
				selectedModel = restored
			}
		}
		if selectedModel == nil {
			fallbackMessage = fmt.Sprintf("Could not restore model %s/%s", existing.Model.Provider, existing.Model.ModelID)
		}
	}
	if selectedModel == nil {
		defaultProvider, _ := services.Settings.GetDefaultProvider()
		defaultModel, _ := services.Settings.GetDefaultModel()
		defaultThinking, _ := services.Settings.GetDefaultThinkingLevel()
		result := config.FindInitialModel(config.FindInitialModelOptions{
			ScopedModels:         toConfigScoped(options.ScopedModels),
			IsContinuing:         hasExisting,
			DefaultProvider:      defaultProvider,
			DefaultModelID:       defaultModel,
			DefaultThinkingLevel: defaultThinking,
			ModelThinkingLevels:  services.Settings.GetAllModelThinkingLevels(),
			AvailableModels:      options.AvailableModels,
			GetModel:             options.GetModel,
			HasConfiguredAuth:    options.HasConfiguredAuth,
		})
		selectedModel = result.Model
		if result.FallbackMessage != "" {
			fallbackMessage = result.FallbackMessage
		}
		if selectedModel != nil && fallbackMessage != "" {
			fallbackMessage += fmt.Sprintf(". Using %s/%s", selectedModel.Provider, selectedModel.ID)
		}
	}

	thinkingLevel := options.ThinkingLevel
	if thinkingLevel == "" && hasExisting {
		if hasThinkingEntry {
			thinkingLevel = model.ThinkingLevel(existing.ThinkingLevel)
		} else if def, ok := services.Settings.GetDefaultThinkingLevel(); ok {
			thinkingLevel = def
		} else {
			thinkingLevel = DefaultThinkingLevel
		}
	}
	if thinkingLevel == "" && selectedModel != nil {
		if perModel, ok := services.Settings.GetModelThinkingLevel(string(selectedModel.Provider), selectedModel.ID); ok {
			thinkingLevel = perModel
		}
	}
	if thinkingLevel == "" {
		if def, ok := services.Settings.GetDefaultThinkingLevel(); ok {
			thinkingLevel = def
		} else {
			thinkingLevel = DefaultThinkingLevel
		}
	}
	if selectedModel == nil {
		thinkingLevel = model.ThinkingOff
	}

	initial, allowed := resolveToolSelection(services.Settings, options.Tools, options.ExcludeTools, options.NoTools, options.CustomTools)

	initialMessages := make([]model.AgentMessage, 0, len(existing.Messages))
	for _, message := range existing.Messages {
		initialMessages = append(initialMessages, model.CloneMessage(message))
	}

	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{
			Model:         selectedModel,
			ThinkingLevel: thinkingLevel,
			Messages:      initialMessages,
		},
		StreamFn: options.StreamFn,
	})
	builtAgent.SetSessionID(sessionManager.GetSessionID())

	session, err := New(Config{
		Agent:                  builtAgent,
		SessionManager:         sessionManager,
		Settings:               services.Settings,
		Cwd:                    services.Cwd,
		ResourceLoader:         services.ResourceLoader,
		StreamFn:               options.StreamFn,
		APIKey:                 options.APIKey,
		Headers:                options.Headers,
		Env:                    options.Env,
		ScopedModels:           append([]ScopedModel(nil), options.ScopedModels...),
		InitialActiveToolNames: initial,
		AllowedToolNames:       allowed,
		ExcludedToolNames:      options.ExcludeTools,
		BaseTools:              options.BaseTools,
		CustomTools:            options.CustomTools,
	})
	if err != nil {
		return nil, err
	}

	// Save the initial model and thinking level for new sessions so they can be
	// restored on resume.
	if !hasExisting {
		if selectedModel != nil {
			_, _ = sessionManager.AppendModelChange(string(selectedModel.Provider), selectedModel.ID)
		}
		_, _ = sessionManager.AppendThinkingLevelChange(string(thinkingLevel))
	} else if !hasThinkingEntry {
		_, _ = sessionManager.AppendThinkingLevelChange(string(thinkingLevel))
	}

	return &CreateResult{Session: session, ModelFallbackMessage: fallbackMessage}, nil
}

func toConfigScoped(scoped []ScopedModel) []config.ScopedModel {
	if len(scoped) == 0 {
		return nil
	}
	out := make([]config.ScopedModel, 0, len(scoped))
	for _, item := range scoped {
		out = append(out, config.ScopedModel{Model: item.Model, ThinkingLevel: item.ThinkingLevel})
	}
	return out
}

// resolveToolSelection mirrors sdk.ts's initial tool selection. It returns the
// initial active names and the allowlist (nil when unrestricted).
func resolveToolSelection(
	settings *config.SettingsManager,
	tools []string,
	excludeTools []string,
	noTools string,
	customTools []model.ToolDefinition,
) (initial []string, allowed []string) {
	configured, hasConfigured := settings.GetDefaultTools()
	allowed = tools
	if tools == nil && noTools == "all" {
		allowed = []string{}
	}
	switch {
	case tools != nil:
		initial = append([]string(nil), tools...)
	case noTools != "":
		initial = []string{}
	case hasConfigured:
		initial = append([]string(nil), configured...)
	default:
		initial = append([]string(nil), DefaultActiveToolNames...)
	}
	if allowed == nil {
		for _, definition := range customTools {
			initial = append(initial, definition.Name)
		}
	}
	excluded := toNameSet(excludeTools)
	if excluded != nil {
		filtered := initial[:0]
		for _, name := range initial {
			if !excluded[name] {
				filtered = append(filtered, name)
			}
		}
		initial = filtered
	}
	return initial, allowed
}
