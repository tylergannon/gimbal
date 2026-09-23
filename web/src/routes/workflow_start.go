package routes

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/internal/builtin"
	"github.com/tylergannon/gimble/internal/host"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/skgo"
)

// StartAccepted identifies the registered run and its owning project.
type StartAccepted struct {
	ProjectID string `json:"project_id"`
	RunID     string `json:"run_id"`
}

// startWorkflow performs shared admission checks; the caller supplies its
// concrete workflow body directly, with no executable selection at runtime.
func startWorkflow(ctx context.Context, projectDir, workDir, conversation, name string, overrides map[gimble.WorkflowRole]polytype.Optional[string], body func(context.Context, string) error) (StartAccepted, error) {
	if strings.TrimSpace(projectDir) == "" {
		return StartAccepted{}, skgo.Invalidf("project_dir", "Choose a project directory.")
	}
	if !filepath.IsAbs(projectDir) {
		return StartAccepted{}, skgo.Invalidf("project_dir", "Use an absolute project directory.")
	}
	if workDir != "" && !filepath.IsAbs(workDir) {
		return StartAccepted{}, skgo.Invalidf("work_dir", "Use an absolute work directory.")
	}
	defaults := builtin.Defaults()
	specs := make(map[gimble.WorkflowRole]string, len(overrides))
	for role, override := range overrides {
		spec := defaults[role]
		if override.Present {
			spec = override.Value
		}
		if spec == "" {
			return StartAccepted{}, skgo.Invalidf("role_"+strings.ReplaceAll(string(role), "-", "_"), "Choose a model for %s.", role)
		}
		specs[role] = spec
	}
	models, err := binding.Roles(specs)
	if err != nil {
		field := ""
		if strings.HasPrefix(err.Error(), "--") {
			role := strings.TrimPrefix(strings.SplitN(err.Error(), ":", 2)[0], "--")
			field = "role_" + strings.ReplaceAll(role, "-", "_")
		}
		return StartAccepted{}, skgo.Invalidf(field, "%s", err)
	}
	owner := host.OwnerFrom(ctx)
	if owner == nil {
		return StartAccepted{}, skgo.Errorf(http.StatusInternalServerError, "This server has no workflow owner.")
	}
	project, err := owner.AdmitProject(projectDir)
	if err != nil {
		return StartAccepted{}, skgo.Invalidf("project_dir", "%s", err)
	}
	if workDir == "" {
		workDir = project.Path()
	}
	id, err := project.Start(name, workDir, conversation, models, func(runCtx context.Context) error { return body(runCtx, workDir) })
	if err != nil {
		field := ""
		switch {
		case strings.Contains(err.Error(), "work_dir"):
			field = "work_dir"
		case strings.Contains(err.Error(), "conversation"):
			field = "conversation"
		}
		return StartAccepted{}, skgo.Invalidf(field, "%s", err)
	}
	return StartAccepted{ProjectID: project.ID(), RunID: id}, nil
}
