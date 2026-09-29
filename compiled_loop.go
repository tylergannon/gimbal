package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func (compiledRuntime) OpenLoop(ctx context.Context, name string) (context.Context, func(error) error, error) {
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
func (compiledRuntime) OpenTask(ctx context.Context, name string, raw []byte, base compiledscope.Snapshot) (context.Context, compiledscope.Snapshot, func(error) error, error) {
	parent, err := current(ctx)
	if err != nil {
		return nil, base, nil, err
	}
	if !parent.loop {
		return nil, base, nil, fmt.Errorf("gimbal: task requires loop scope")
	}
	var task Task
	if err = json.Unmarshal(raw, &task); err != nil {
		return nil, base, nil, err
	}
	if err = validateTask(task); err != nil {
		return nil, base, nil, err
	}
	child, taskCtx := prepareTask(parent, ctx, name, task)
	scoped := child.begin(taskCtx)
	entry, err := compiledscope.Encode("task", task)
	var ref compiledscope.Snapshot
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
func (compiledRuntime) EndLoop(ctx context.Context) error { return endLoop(ctx) }

func planNext(ctx context.Context, planner *Session, goal string, tasks []Task, previous string, opts []AgentOption) (plan, error) {
	s, err := current(ctx)
	if err != nil {
		return plan{}, err
	}
	if !s.loop {
		return plan{}, fmt.Errorf("gimbal: planner dispatch requires loop scope")
	}
	if planner == nil {
		return plan{}, fmt.Errorf("gimbal: planner session is nil")
	}
	if err = planner.reachable(ctx); err != nil {
		return plan{}, err
	}
	backlog, err := backlogJSON(goal, tasks)
	if err != nil {
		return plan{}, err
	}
	messages := s.takeMessages()
	for _, message := range messages {
		s.run.event(s.key, "", "", Steer{Target: s.key, Source: "person", Message: message, Landed: true})
		logf("%s: a message reached the planner: %s", s.key, oneLine(message))
	}
	prompt, err := planPrompt(ctx, s.name, planner.workdir, string(backlog), scopeText(ctx), previous, messages)
	if err != nil {
		return plan{}, err
	}
	result, err := dispatch[answer](ctx, planner, prompt, opts)
	return result.plan, err
}
func (compiledRuntime) Plan(ctx context.Context, resource any, input compiledscope.Snapshot, goal string, tasksJSON []byte, previous string) ([]byte, error) {
	planner, ok := resource.(*Session)
	if !ok {
		return nil, fmt.Errorf("invalid planner session")
	}
	var tasks []Task
	if len(tasksJSON) > 0 {
		if err := json.Unmarshal(tasksJSON, &tasks); err != nil {
			return nil, err
		}
	}
	bound, err := bindCompiledContext(ctx, input)
	if err != nil {
		return nil, err
	}
	p, err := planNext(bound, planner, goal, tasks, previous, nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}
func recordPlan(ctx context.Context, goal string, p plan) error {
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
func (compiledRuntime) RecordPlan(ctx context.Context, goal string, raw []byte) error {
	var p plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	return recordPlan(ctx, goal, p)
}
func (compiledRuntime) TaskFeedback(ctx context.Context) (string, error) {
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
