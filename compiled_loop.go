package gimbal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func init() {
	compiledscope.OpenLoop = func(ctx context.Context, name string) (context.Context, func(error) error, error) {
		parent, err := current(ctx)
		if err != nil {
			return nil, nil, err
		}
		child := parent.child(name)
		child.loop = true
		child.dispatching = true
		dir := filepath.Join(child.run.dir, "scopes", filepath.FromSlash(child.key))
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, err
		}
		scoped := child.begin(ctx)
		return scoped, func(err error) error { _ = compiledscope.EndLoop(scoped); return child.finish(err) }, nil
	}
	compiledscope.OpenTask = func(ctx context.Context, name string, raw []byte) (context.Context, func(error) error, error) {
		parent, err := current(ctx)
		if err != nil {
			return nil, nil, err
		}
		var task Task
		if err = json.Unmarshal(raw, &task); err != nil {
			return nil, nil, err
		}
		child := parent.child(name)
		return child.begin(context.WithValue(ctx, taskKey{}, task)), child.finish, nil
	}
	compiledscope.EndLoop = func(ctx context.Context) error {
		s, err := current(ctx)
		if err != nil {
			return err
		}
		for _, message := range s.endDispatch() {
			s.run.event(s.key, "", "", Steer{Target: s.key, Source: "person", Message: message})
		}
		return nil
	}
	compiledscope.Plan = func(ctx context.Context, resource any, goal, workdir string, tasksJSON []byte, previous string) ([]byte, error) {
		s, err := current(ctx)
		if err != nil {
			return nil, err
		}
		planner, ok := resource.(*Session)
		if !ok {
			return nil, fmt.Errorf("invalid planner session")
		}
		var tasks []Task
		if len(tasksJSON) > 0 {
			if err = json.Unmarshal(tasksJSON, &tasks); err != nil {
				return nil, err
			}
		}
		backlog, err := backlogJSON(goal, tasks)
		if err != nil {
			return nil, err
		}
		messages := s.takeMessages()
		for _, message := range messages {
			s.run.event(s.key, "", "", Steer{Target: s.key, Source: "person", Message: message, Landed: true})
		}
		// Reuse the runtime's prompt construction, budgets and validated plan type.
		prompt, err := planPrompt(ctx, "plan", workdir, string(backlog), scopeText(ctx), previous, messages)
		if err != nil {
			return nil, err
		}
		result, err := dispatch[answer](ctx, planner, prompt, nil)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result.plan)
	}
	compiledscope.RecordPlan = func(ctx context.Context, goal string, raw []byte) error {
		s, err := current(ctx)
		if err != nil {
			return err
		}
		var p plan
		if err = json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if err = validatePlan(p); err != nil {
			return err
		}
		if p.Tasks == nil {
			p.Tasks = []Task{}
		}
		backlog, err := backlogJSON(goal, p.Tasks)
		if err != nil {
			return err
		}
		file := filepath.Join(s.run.dir, "scopes", filepath.FromSlash(s.key), "backlog.md")
		if err = os.WriteFile(file, []byte("---\n"+string(backlog)+"\n---\n"), 0644); err != nil {
			return err
		}
		decision := PlannerDecision{}
		if p.Next.Present {
			decision.Task = optionalTask(p.Tasks[p.Next.Value])
		}
		s.run.event(s.key, "", "", decision)
		return nil
	}
	compiledscope.TaskFeedback = func(ctx context.Context) (string, error) {
		if _, err := current(ctx); err != nil {
			return "", err
		}
		return strings.Join(renderVisible(visibleValues(ctx), -1), "\n\n"), nil
	}
}
