package gimbal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/contextdata"
)

// OpenLoop opens a planner scope. Its finish records undelivered steering.
func OpenLoop(ctx context.Context, name string) (context.Context, func(error) error, error) {
	parent, err := current(ctx)
	if err != nil {
		return nil, nil, err
	}
	child := parent.child(name)
	child.loop, child.dispatching = true, true
	if err = prepareLoop(child); err != nil {
		return nil, nil, err
	}
	scoped := child.begin(ctx)
	return scoped, func(err error) error {
		child.finishMu.Lock()
		defer child.finishMu.Unlock()
		if legality := child.canFinish(); legality != nil {
			return legality
		}
		_ = endLoop(scoped)
		return child.finish(err)
	}, nil
}
func prepareLoop(s *scope) error {
	return os.MkdirAll(filepath.Join(s.run.dir, "scopes", filepath.FromSlash(s.key)), 0755)
}

// OpenTask opens a task scope and records its task input in a child snapshot.
// Initialization failure closes the partial scope and returns no live handle.
func OpenTask(ctx context.Context, name string, task Task, base contextdata.Snapshot) (context.Context, contextdata.Snapshot, func(error) error, error) {
	parent, err := current(ctx)
	if err != nil {
		return nil, base, nil, err
	}
	if !parent.loop {
		return nil, base, nil, fmt.Errorf("gimbal: task requires loop scope")
	}
	if err = validateTask(task); err != nil {
		return nil, base, nil, err
	}
	child, taskCtx := prepareTask(parent, ctx, name, task)
	scoped := child.begin(taskCtx)
	entry, err := contextdata.Encode("task", task)
	var ref contextdata.Snapshot
	if err == nil {
		ref, err = writeCompiledContext(scoped, base, entry)
	}
	if err != nil {
		return nil, base, nil, errors.Join(err, child.finish(err))
	}
	return scoped, ref, child.finishCompiled, nil
}
func prepareTask(parent *scope, ctx context.Context, name string, task Task) (*scope, context.Context) {
	return parent.child(name), context.WithValue(ctx, taskKey{}, task)
}
func endLoop(ctx context.Context) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	for _, message := range s.endDispatch() {
		s.run.event(s.key, "", "", Steer{Target: s.key, Source: "person", Message: message})
		logf("%s: a message was dropped, dispatch ended first: %s", s.key, oneLine(message))
	}
	return nil
}

func planNext(ctx context.Context, planner *Session, goal string, tasks []Task, previous string, opts []AgentOption) (Plan, error) {
	s, err := current(ctx)
	if err != nil {
		return Plan{}, err
	}
	if !s.loop {
		return Plan{}, fmt.Errorf("gimbal: planner dispatch requires loop scope")
	}
	if planner == nil {
		return Plan{}, fmt.Errorf("gimbal: planner session is nil")
	}
	if err = planner.reachable(ctx); err != nil {
		return Plan{}, err
	}
	backlog, err := backlogJSON(goal, tasks)
	if err != nil {
		return Plan{}, err
	}
	messages := s.takeMessages()
	for _, message := range messages {
		s.run.event(s.key, "", "", Steer{Target: s.key, Source: "person", Message: message, Landed: true})
		logf("%s: a message reached the planner: %s", s.key, oneLine(message))
	}
	prompt, err := planPrompt(ctx, s.name, planner.workdir, string(backlog), scopeText(ctx), previous, messages)
	if err != nil {
		return Plan{}, err
	}
	result, err := dispatch[answer](ctx, planner, prompt, opts)
	return result.Plan, err
}

// PlanNext performs one planner dispatch using the explicit loop snapshot.
// RecordPlan validates and records its decision separately; the consumer schedules
// repetition and task bodies. Previous is task-local feedback, not inherited input.
func PlanNext(ctx context.Context, input contextdata.Snapshot, planner *Session, goal string, tasks []Task, previous string, opts ...AgentOption) (Plan, error) {
	bound, err := bindCompiledContext(ctx, input)
	if err != nil {
		return Plan{}, err
	}
	return planNext(bound, planner, goal, tasks, previous, opts)
}
func recordPlan(ctx context.Context, goal string, p Plan) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	if !s.loop {
		return fmt.Errorf("gimbal: record plan requires loop scope")
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

// RecordPlan validates a planner decision and records the revised backlog.
func RecordPlan(ctx context.Context, goal string, p Plan) error {
	return recordPlan(ctx, goal, p)
}

// TaskFeedback renders the task's actually recorded local writes before cleanup.
// Its file references belong to this owner's execution environment.
func TaskFeedback(ctx context.Context) (string, error) {
	s, err := current(ctx)
	if err != nil {
		return "", err
	}
	if _, ok := ctx.Value(taskKey{}).(Task); !ok {
		return "", fmt.Errorf("gimbal: task feedback requires task scope")
	}
	// Repair disposable task materializations before the planner receives paths.
	s.mu.Lock()
	values := make([]*scopeValue, 0, len(s.values))
	for _, value := range s.values {
		values = append(values, value)
	}
	s.mu.Unlock()
	for _, value := range values {
		if value.artifact == nil || s.run.contextStore == nil {
			continue
		}
		id, ok := strings.CutPrefix(value.artifact.file, "context/objects/")
		if !ok {
			continue
		}
		data, err := s.run.contextStore.ReadObject(ctx, id)
		if err != nil {
			return "", err
		}
		if _, err = s.run.contextStore.MaterializeText(id, data); err != nil {
			return "", err
		}
	}
	return s.localText(), nil
}
