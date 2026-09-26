package session

import (
	"github.com/tylergannon/gimbal/internal/pi/edittools"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/readtools"
	"github.com/tylergannon/gimbal/internal/pi/shell"
)

// This file ports core/tools/index.ts and core/tools/tool-definition-wrapper.ts
// at upstream pin d6af72e1. PowerShell and extension-provided tools are out of
// scope for the headless port.

// ToolName is a built-in tool name.
type ToolName string

// The built-in tool names.
const (
	ToolRead  ToolName = "read"
	ToolBash  ToolName = "bash"
	ToolEdit  ToolName = "edit"
	ToolWrite ToolName = "write"
	ToolGrep  ToolName = "grep"
	ToolFind  ToolName = "find"
	ToolLs    ToolName = "ls"
)

// AllToolNames is the set of built-in tools this port can construct.
var AllToolNames = map[ToolName]bool{
	ToolRead: true, ToolBash: true, ToolEdit: true, ToolWrite: true,
	ToolGrep: true, ToolFind: true, ToolLs: true,
}

// ToolsOptions carries per-tool construction options.
type ToolsOptions struct {
	Read *readtools.ReadToolOptions
	Bash *shell.BashToolOptions
	Edit *edittools.EditOptions
}

// ToolDefinitionFromAgentTool synthesizes a minimal ToolDefinition from an
// AgentTool, mirroring createToolDefinitionFromAgentTool.
func ToolDefinitionFromAgentTool(tool model.AgentTool) model.ToolDefinition {
	return model.ToolDefinition{
		Name:                tool.Name,
		Label:               tool.Label,
		Description:         tool.Description,
		Parameters:          tool.Parameters,
		ConstrainedSampling: tool.ConstrainedSampling,
		PrepareArguments:    tool.PrepareArguments,
		ExecutionMode:       tool.ExecutionMode,
		Execute:             tool.Execute,
	}
}

// WrapToolDefinition wraps a ToolDefinition into an AgentTool for the core
// runtime, mirroring wrapToolDefinition.
func WrapToolDefinition(definition model.ToolDefinition) model.AgentTool {
	return model.AgentTool{
		Name:                definition.Name,
		Label:               definition.Label,
		Description:         definition.Description,
		Parameters:          definition.Parameters,
		ConstrainedSampling: definition.ConstrainedSampling,
		PrepareArguments:    definition.PrepareArguments,
		ExecutionMode:       definition.ExecutionMode,
		Execute:             definition.Execute,
	}
}

// CreateToolDefinition constructs one built-in tool definition by name.
func CreateToolDefinition(name ToolName, cwd string, options *ToolsOptions) (model.ToolDefinition, bool) {
	if options == nil {
		options = &ToolsOptions{}
	}
	switch name {
	case ToolRead:
		return readtools.ReadTool(cwd, options.Read), true
	case ToolBash:
		return bashDefinition(cwd, options.Bash), true
	case ToolEdit:
		return edittools.EditDefinition(cwd, options.Edit), true
	case ToolWrite:
		return edittools.WriteDefinition(cwd, nil), true
	case ToolGrep:
		return readtools.GrepTool(cwd, nil), true
	case ToolFind:
		return readtools.FindTool(cwd, nil), true
	case ToolLs:
		return readtools.LsTool(cwd, nil), true
	default:
		return model.ToolDefinition{}, false
	}
}

// bashDefinition builds the bash tool definition, restoring the prompt
// snippet and guidelines that CreateBashTool cannot carry on AgentTool.
func bashDefinition(cwd string, options *shell.BashToolOptions) model.ToolDefinition {
	tool := shell.CreateBashTool(cwd, options)
	definition := ToolDefinitionFromAgentTool(tool)
	definition.PromptSnippet = shell.BashToolSnippet
	definition.PromptGuidelines = append([]string(nil), shell.BashToolGuidelines...)
	return definition
}

// CreateAllToolDefinitions constructs every built-in tool definition.
func CreateAllToolDefinitions(cwd string, options *ToolsOptions) map[string]model.ToolDefinition {
	definitions := make(map[string]model.ToolDefinition, len(AllToolNames))
	for name := range AllToolNames {
		definition, _ := CreateToolDefinition(name, cwd, options)
		definitions[string(name)] = definition
	}
	return definitions
}

// CreateCodingToolDefinitions is the read/bash/edit/write subset.
func CreateCodingToolDefinitions(cwd string, options *ToolsOptions) []model.ToolDefinition {
	out := make([]model.ToolDefinition, 0, 4)
	for _, name := range []ToolName{ToolRead, ToolBash, ToolEdit, ToolWrite} {
		definition, _ := CreateToolDefinition(name, cwd, options)
		out = append(out, definition)
	}
	return out
}

// CreateReadOnlyToolDefinitions is the read/grep/find/ls subset.
func CreateReadOnlyToolDefinitions(cwd string, options *ToolsOptions) []model.ToolDefinition {
	out := make([]model.ToolDefinition, 0, 4)
	for _, name := range []ToolName{ToolRead, ToolGrep, ToolFind, ToolLs} {
		definition, _ := CreateToolDefinition(name, cwd, options)
		out = append(out, definition)
	}
	return out
}

// DefaultActiveToolNames is the built-in active loadout pi starts with.
var DefaultActiveToolNames = []string{"read", "bash", "edit", "write"}
