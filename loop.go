package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tylergannon/polytype"
	"go.yaml.in/yaml/v4"
)

// Task is one assignment selected by a Loop planner.
type Task struct {
	// Name is a short label for recognizing the work.
	Name string `json:"name" yaml:"name"`
	// Description states the desired result and any necessary, non-obvious
	// information. It leaves the approach to the worker.
	Description string `json:"description" yaml:"description"`
	// DefinitionOfDone says how to recognize successful completion of this
	// assignment. It does not declare that the enclosing goal is complete.
	DefinitionOfDone string `json:"definition_of_done" yaml:"definition_of_done"`
	// Validation describes evidence the workflow can gather. Either field may
	// be empty; the workflow still assesses the task against DefinitionOfDone.
	Validation struct {
		// Command is a known executable check.
		Command string `json:"command" yaml:"command,omitempty"`
		// Query is a question for a validator agent.
		Query string `json:"query" yaml:"query,omitempty"`
	} `json:"validation" yaml:"validation"`
}

// plan is the planner's answer for one dispatch. A null Next ends dispatch;
// it does not attest that the enclosing goal has been fulfilled.
type plan struct {
	Next polytype.Nullable[Task] `json:"next"`
}

type loop struct {
	ctx     context.Context
	name    string
	goal    string
	planner *Session
	err     error
}

// Loop opens planner-directed dispatch for goal. The planner is the Session
// chosen and prepared by the workflow. Loop keeps a revisable backlog in its
// run scope and uses values recorded by each task as feedback for the next
// decision. Range over its Tasks method and check Err afterward.
func Loop(ctx context.Context, name, goal string, planner *Session) *loop {
	return &loop{ctx: ctx, name: name, goal: goal, planner: planner}
}

// Tasks yields planner-selected assignments. Each ctx is a child scope that
// contains the structured task and ends when the loop body returns. Values the
// body records in that scope are shown to the planner before its next decision,
// alongside the values currently visible from the loop's parent scopes.
//
// The planner may revise, reorder, and extend the backlog as work reveals what
// matters. It ends dispatch by returning no task. That decision is distinct
// from validation and from fulfillment of the enclosing goal.
func (l *loop) Tasks(yield func(context.Context, Task) bool) {
	parent, err := current(l.ctx)
	if err != nil {
		l.err = err
		return
	}
	l.err = parent.child(l.name).do(l.ctx, func(ctx context.Context) error {
		loopScope, _ := current(ctx)
		dir := filepath.Join(loopScope.run.dir, "scopes", filepath.FromSlash(loopScope.key))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("gimble: %w", err)
		}
		file := filepath.Join(dir, "backlog.md")
		front, err := yaml.Marshal(backlog{Goal: l.goal, Tasks: []Task{}})
		if err != nil {
			return fmt.Errorf("gimble: %w", err)
		}
		if err := os.WriteFile(file, []byte("---\n"+string(front)+"---\n"), 0o644); err != nil {
			return fmt.Errorf("gimble: %w", err)
		}

		var previous string
		for {
			raw, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("gimble: %w", err)
			}
			_, bad := readBacklog(raw, l.goal)
			if bad != nil {
				logf("%s: the backlog does not parse, so the planner is asked to fix it: %v", loopScope.key, bad)
			}
			p, err := l.planner.Generate[plan](ctx, planPrompt(l.name, file, l.planner.workdir, string(raw), bad, ScopeText(ctx), previous))
			if err != nil {
				return err
			}

			revised, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("gimble: %w", err)
			}
			backlog, err := readBacklog(revised, l.goal)
			if err != nil {
				logf("%s: the planner left an invalid backlog, so it will be asked to repair it: %v", loopScope.key, err)
				continue
			}
			if !p.Next.Present {
				loopScope.run.event(loopScope.key, "", "", PlannerDecision{})
				logf("%s: the planner ended dispatch", loopScope.key)
				return nil
			}
			if err := validateTask(p.Next.Value); err != nil {
				return fmt.Errorf("gimble: loop %q: planner returned an invalid task: %w", l.name, err)
			}
			if !slices.Contains(backlog.Tasks, p.Next.Value) {
				return fmt.Errorf("gimble: loop %q: planner returned task %q but did not preserve that assignment in the backlog", l.name, p.Next.Value.Name)
			}

			task := p.Next.Value
			logf("%s: task: %s", loopScope.key, oneLine(task.Name))
			loopScope.run.event(loopScope.key, "", "", PlannerDecision{Task: optionalTask(task)})
			more := true
			taskScope := loopScope.child("task")
			taskCtx := context.WithValue(ctx, taskKey{}, task)
			if err := taskScope.do(taskCtx, func(ctx context.Context) error {
				raw, err := json.Marshal(task)
				if err != nil {
					return fmt.Errorf("gimble: encode task: %w", err)
				}
				if err := store(ctx, "task", raw); err != nil {
					return err
				}
				more = yield(ctx, task)
				previous = taskScope.localText()
				return nil
			}); err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	})
}

// Err returns the error that ended dispatch, if any: persistence, malformed
// planner data, cancellation, or the planner's harness. A failed task
// validation recorded by the workflow is feedback, not a Loop error.
func (l *loop) Err() error {
	return l.err
}

type backlog struct {
	Goal  string `json:"goal" yaml:"goal"`
	Tasks []Task `json:"tasks" yaml:"tasks"`
}

func readBacklog(raw []byte, goal string) (backlog, error) {
	rest, ok := strings.CutPrefix(string(raw), "---\n")
	front, _, closed := strings.Cut(rest, "\n---")
	if !ok || !closed {
		return backlog{}, errors.New("the file does not start with YAML frontmatter between --- lines")
	}
	var b backlog
	if err := yaml.Unmarshal([]byte(front), &b); err != nil {
		return backlog{}, err
	}
	if b.Goal != goal {
		return backlog{}, errors.New("the goal was changed")
	}
	for i, task := range b.Tasks {
		if err := validateTask(task); err != nil {
			return backlog{}, fmt.Errorf("task %d: %w", i+1, err)
		}
	}
	return b, nil
}

func validateTask(task Task) error {
	switch {
	case strings.TrimSpace(task.Name) == "":
		return errors.New("name is blank")
	case strings.TrimSpace(task.Description) == "":
		return errors.New("description is blank")
	case strings.TrimSpace(task.DefinitionOfDone) == "":
		return errors.New("definition of done is blank")
	default:
		return nil
	}
}

func planPrompt(name, file, workdir, backlogText string, bad error, scoped, previous string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You plan the loop %q in %s. Its revisable backlog is %s.\n\n", name, workdir, file)
	b.WriteString("Choose the next assignment that offers the greatest concrete gain toward the goal, based on current evidence, priorities, and real dependencies. Size it for one worker to understand, complete, and demonstrate in one working session. A later task may offer more gain than repairing a nonblocking earlier defect; keep deferred defects visible.\n\n")
	b.WriteString("Treat recorded deterministic results as authoritative: a prose claim or agent judgment cannot override a nonzero command exit.\n\n")
	b.WriteString("Inspect the workspace only to plan. Change only the backlog; do not perform or validate an assignment yourself.\n\n")
	b.WriteString("Describe the desired result and necessary non-obvious facts. Trust the worker to choose the approach. Do not supply procedural checklists, obvious advice, speculative code, or a numerical progress score.\n\n")
	b.WriteString("The backlog has immutable `goal` and a `tasks` list using the result schema. Edit it as the work changes. The exact task you return must remain in that list until its result is available on the next call. Return `next: null` to end dispatch; that does not certify that the goal is fulfilled.\n\n")
	if strings.TrimSpace(scoped) != "" {
		b.WriteString("Scoped context:\n\n" + scoped + "\n\n")
	}
	if strings.TrimSpace(previous) != "" {
		b.WriteString("Previous task record:\n\n" + previous + "\n\n")
	}
	b.WriteString("Backlog now:\n\n" + backlogText + "\n\n")
	if bad != nil {
		fmt.Fprintf(&b, "The backlog is invalid, so no task can be dispatched yet: %v. Repair it before choosing work.\n\n", bad)
	}
	b.WriteString("Revise the backlog and return the next assignment exactly as it appears there, or null.")
	return b.String()
}
