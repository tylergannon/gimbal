package workflowpage

import (
	"context"
	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/skgo"
	"net/http"
)

type WorkflowData struct {
	SourceJSON string `json:"source_json"`
}

func load(ctx context.Context) (WorkflowData, error) {
	name := skgo.EventFrom(ctx).Param("workflow")
	data, err := builtin.WorkflowPageJSON(name)
	if err != nil {
		return WorkflowData{}, skgo.Errorf(http.StatusNotFound, "Unknown workflow")
	}
	return WorkflowData{SourceJSON: data}, nil
}

var _ = skgo.Load(load)
