package edittools

import "github.com/tylergannon/gimbal/internal/pi/model"

// preferStrictToolSampling is pi's inlined `{type: "json_schema", strict:
// "prefer"}` on the built-in edit and write definitions. A fresh value is
// returned per tool so no two tools share mutable config.
func preferStrictToolSampling() *model.ConstrainedSamplingConfig {
	return &model.ConstrainedSamplingConfig{
		Type:   model.ConstrainedSamplingJSONSchema,
		Strict: model.ConstrainedSamplingPrefer,
	}
}

func agentToolFromDefinition(d model.ToolDefinition) model.AgentTool {
	return model.AgentTool{
		Name:                d.Name,
		Description:         d.Description,
		Parameters:          d.Parameters,
		Label:               d.Label,
		ExecutionMode:       d.ExecutionMode,
		PrepareArguments:    d.PrepareArguments,
		ConstrainedSampling: d.ConstrainedSampling,
		Execute:             d.Execute,
	}
}
